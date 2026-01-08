package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

// Site struct matches Local's sites.json structure
type Site struct {
    ID       string             `json:"id"`
    Name     string             `json:"name"`
    Path     string             `json:"path"`
    Domain   string             `json:"domain"`
    MySQL    MySQLConfig        `json:"mysql,omitempty"`
    Services map[string]Service `json:"services,omitempty"`
}

type MySQLConfig struct {
    Database string `json:"database"`
    User     string `json:"user"`
    Password string `json:"password"`
}

type Service struct {
    Name    string           `json:"name"`
    Version string           `json:"version"`
    Type    string           `json:"type"`
    Ports   map[string][]int `json:"ports,omitempty"`
}

func main() {
    // 1. Parse Arguments
    args := os.Args[1:]

    // Check for help flags
    if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
        printUsage()
        os.Exit(0)
    }

    // Pre-flight check
    if err := checkForBash(); err != nil {
        fmt.Printf("Setup Error: %v\n", err)
        os.Exit(1)
    }

    var searchQuery string
    var action string
    var extraArgs []string

    if len(args) > 0 {
        searchQuery = args[0]
    }
    if len(args) > 1 {
        action = args[1]
    } else {
        action = "shell" // Default action
    }
    if len(args) > 2 {
        extraArgs = args[2:]
    }

    // 2. Load Config
    configDir, err := getLocalConfigDir()
    if err != nil {
        fmt.Printf("Error finding Local config directory: %v\n", err)
        os.Exit(1)
    }

    sites, err := getSites(configDir)
    if err != nil {
        fmt.Printf("Error reading sites.json: %v\n", err)
        os.Exit(1)
    }

    if len(sites) == 0 {
        fmt.Println("No sites found.")
        os.Exit(0)
    }

    // 3. Find Target Site
    var selectedSite *Site

    if searchQuery != "" {
        selectedSite = findSite(sites, searchQuery)
        if selectedSite == nil {
            fmt.Printf("Site '%s' not found. Available sites:\n", searchQuery)
            listSites(sites)
            os.Exit(1)
        }
        fmt.Printf("Found site: %s (Action: %s)\n", selectedSite.Name, action)
    } else {
        selectedSite = selectSiteInteractive(sites)
        if selectedSite == nil {
            return // User exited
        }
    }

    // 4. Find SSH Script
    sshDir := filepath.Join(configDir, "ssh-entry")
    scriptPath, err := findScript(sshDir, *selectedSite)
    if err != nil {
        fmt.Printf("Error finding shell script: %v\n", err)
        os.Exit(1)
    }

    // 5. Execute Command
    runAction(scriptPath, *selectedSite, action, extraArgs)
}

func printUsage() {
    fmt.Printf(`Local CLI - A fast CLI interface for Local by Flywheel/WP

USAGE:
    local-cli [site_name] [action]

ARGUMENTS:
    site_name    Name or ID of the site (fuzzy search supported)
    action       Action to perform (default: shell)

ACTIONS:
    shell        Opens the container shell (zsh/bash)
    db           Opens MySQL console directly
    wp           Opens WP-CLI interactive shell

EXAMPLES:
    # Open interactive list
    local-cli

    # Jump directly to 'sg' site shell
    local-cli sg

    # Open database for 'updraftplus'
    local-cli updraftplus db

    # Open WP-CLI for site ID '5jc4NXQ8I'
    local-cli 5jc4NXQ8I wp
`)
}

// runAction handles logic for executing commands.
//
// We use a "Patch Script" strategy rather than extracting a Docker Container ID
// and running raw `docker exec <id> ...`.
//
// because
// 1. Environment Preservation: Local's shell scripts are Host-Side wrappers
//    that carefully configure custom PHP/MySQL paths and WP-CLI configs.
//    Direct Docker exec would bypass these Local-specific configurations.
// 2. Execution Context: Running commands via the patched script ensures
//    we are executing within Local's intended environment.
func runAction(scriptPath string, site Site, action string, extraArgs []string) {
    var command string

    // 1. Construct the command to run
    switch action {
    case "db":
        if site.MySQL.Database == "" {
            fmt.Println("Error: No MySQL configuration found for this site.")
            os.Exit(1)
        }
        command = fmt.Sprintf("mysql -u%s -p%s %s", site.MySQL.User, site.MySQL.Password, site.MySQL.Database)

    case "wp":
        if len(extraArgs) > 0 {
            command = fmt.Sprintf("wp %s", strings.Join(extraArgs, " "))
        } else {
            command = "" // Empty means interactive mode
        }

    case "shell":
        if len(extraArgs) > 0 {
            command = strings.Join(extraArgs, " ")
        } else {
            command = "" // Interactive mode
        }

    default:
        if len(extraArgs) > 0 {
            command = strings.Join(extraArgs, " ")
        }
    }

    // 2. Read the original Local script
    content, err := os.ReadFile(scriptPath)
    if err != nil {
        fmt.Printf("Error reading script: %v\n", err)
        os.Exit(1)
    }

    // 3. Patch the script (Remove exec $SHELL and add our command)
    finalScriptContent := patchScript(string(content), command)

    // 4. Write to a temp file
    tmpFile, err := os.CreateTemp("", "local-cli-*.sh")
    if err != nil {
        fmt.Printf("Error creating temp file: %v\n", err)
        os.Exit(1)
    }
    defer os.Remove(tmpFile.Name())

    if _, err := tmpFile.WriteString(finalScriptContent); err != nil {
        fmt.Printf("Error writing temp file: %v\n", err)
        os.Exit(1)
    }
    tmpFile.Close()

    // 5. Execute the temp script
    var displayMsg string
    if command != "" {
        displayMsg = fmt.Sprintf("Running '%s' on %s...", command, site.Name)
    } else {
        displayMsg = fmt.Sprintf("Opening shell for %s...", site.Name)
    }
    fmt.Println(displayMsg)

    // We run the temp file directly with bash
    cmd := exec.Command("bash", tmpFile.Name())
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        fmt.Printf("\nProcess finished: %v\n", err)
    }
}

// patchScript modifies the Local shell script to support one-off commands.
//
// The original script ends with 'exec $SHELL' which creates an interactive session.
// This function strips that line and appends our custom command instead.
// This allows the script to run the command and then exit, enabling
// command chaining (e.g., '&&') and piping (e.g., '|').
func patchScript(content string, command string) string {
    if command == "" {
        return content // No command? Return original script (interactive)
    }

    lines := strings.Split(content, "\n")
    var newLines []string

    // Rebuild script content, skipping lines that launch interactive shell
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        // Skip the final 'exec $SHELL' line
        // Also skip the echo statement "Launching shell" to reduce noise
        if trimmed == "exec $SHELL" || strings.Contains(trimmed, "Launching shell") {
            continue
        }
        newLines = append(newLines, line)
    }

    // Join lines back and add our command with 'exec' so script terminates correctly
    finalContent := strings.Join(newLines, "\n")
    return fmt.Sprintf("%s\nexec %s\n", finalContent, command)
}

// findScript locates the correct shell script for a given site.
// It searches the 'ssh-entry' directory for a .sh file containing the site's ID or path.
func findScript(sshDir string, site Site) (string, error) {
    entries, err := os.ReadDir(sshDir)
    if err != nil {
        return "", err
    }

    for _, entry := range entries {
        if !strings.HasSuffix(entry.Name(), ".sh") {
            continue
        }

        scriptPath := filepath.Join(sshDir, entry.Name())
        content, err := os.ReadFile(scriptPath)
        if err != nil {
            continue
        }
        contentStr := string(content)

        if strings.Contains(contentStr, site.ID) {
            return scriptPath, nil
        }

        pathBase := filepath.Base(site.Path)
        if pathBase != "" && strings.Contains(contentStr, pathBase) {
            return scriptPath, nil
        }
    }

    return "", fmt.Errorf("no matching shell script found for site: %s", site.Name)
}

func checkForBash() error {
    if _, err := exec.LookPath("bash"); err != nil {
        return fmt.Errorf("'bash' command not found. On Windows, install Git Bash or WSL and ensure bash is in your PATH")
    }
    return nil
}

func findSite(sites []Site, query string) *Site {
    lowerQuery := strings.ToLower(query)

    for _, s := range sites {
        // Exact ID match
        if strings.EqualFold(s.ID, query) {
            return &s
        }
        // Exact Name match
        if strings.EqualFold(s.Name, query) {
            return &s
        }
        // Partial Name match (contains)
        if strings.Contains(strings.ToLower(s.Name), lowerQuery) {
            return &s
        }
    }
    return nil
}

func selectSiteInteractive(sites []Site) *Site {
    listSites(sites)
    fmt.Print("\nSelect site number: ")

    reader := bufio.NewReader(os.Stdin)
    input, _ := reader.ReadString('\n')
    input = strings.TrimSpace(input)

    var selection int
    _, err := fmt.Sscanf(input, "%d", &selection)
    if err != nil || selection < 1 || selection > len(sites) {
        fmt.Println("Invalid selection.")
        return nil
    }

    return &sites[selection-1]
}

func listSites(sites []Site) {
    fmt.Println("--- Local Sites ---")
    for i, site := range sites {
        domain := site.Domain
        if domain == "" {
            domain = "no domain"
        }
        fmt.Printf("%d. %-30s %-20s (ID: %s)\n", i+1, site.Name, domain, site.ID)
    }
}

func getLocalConfigDir() (string, error) {
    baseDir, err := os.UserConfigDir()
    if err != nil {
        return "", err
    }
    dir := filepath.Join(baseDir, "Local")
    if _, err := os.Stat(dir); err == nil {
        return dir, nil
    }
    dirLegacy := filepath.Join(baseDir, "Local by Flywheel")
    if _, err := os.Stat(dirLegacy); err == nil {
        return dirLegacy, nil
    }
    return filepath.Join(baseDir, "Local"), nil
}

func getSites(configDir string) ([]Site, error) {
    sitesPath := filepath.Join(configDir, "sites.json")
    data, err := os.ReadFile(sitesPath)
    if err != nil {
        return nil, err
    }

    var siteMap map[string]Site
    if err := json.Unmarshal(data, &siteMap); err != nil {
        return nil, fmt.Errorf("failed to parse sites.json: %w", err)
    }

    var sites []Site
    for _, s := range siteMap {
        if s.ID == "" {
            continue
        }
        sites = append(sites, s)
    }

    return sites, nil
}