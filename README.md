# Local CLI

A fast, interactive command-line tool for [Local by Flywheel / Local WP](https://localwp.com/). 

Access site shells, databases, and WP-CLI directly from your terminal without using the GUI.

## Features

- **Fuzzy Search:** Jump to sites by name or ID (e.g., `local-cli sg`).
- **One-click Database:** Connect to MySQL instantly (e.g., `local-cli sg db`).
- **WP-CLI Access:** Open WP-CLI shells easily.
- **Direct Commands:** Run any command directly (e.g., `local-cli sg ls -la` or `local-cli sg wp plugin list`).
- **Auto-Installer:** Installs Go and the tool automatically on macOS/Linux.

## Installation

### One-line Install (Recommended)
This script checks for Go, installs it if missing, and then installs Local CLI.

```bash
curl -sSL https://raw.githubusercontent.com/ibrahimhajjaj/local-cli/main/install.sh | bash
```

### Manual Install
1. Ensure [Go](https://go.dev/dl/) is installed.
2. Clone and build:
```bash
git clone https://github.com/ibrahimhajjaj/local-cli.git
cd local-cli
go build -o local-cli
sudo mv local-cli /usr/local/bin/
```

## Usage

```bash
# Interactive menu (lists all sites)
local-cli

# Open shell for specific site (supports fuzzy matching)
local-cli my-site-name

# Open MySQL console for a site
local-cli my-site-name db

# Open WP-CLI interactive shell
local-cli my-site-name wp

# Run commands directly
local-cli my-site-name ls -la
local-cli my-site-name git status
local-cli my-site-name wp plugin list
```