#!/bin/bash

set -e # Exit on error

# Configuration
REPO="ibrahimhajjaj/local-cli"
BINARY_NAME="local-cli"
INSTALL_DIR="/usr/local/bin"
GO_VERSION="1.22.7"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Local CLI Installer${NC}"

# --- 1. Check and Install Go ---
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Go is not installed. Installing Go ${GO_VERSION}...${NC}"
    
    # Detect OS and Arch
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$ARCH" == "arm64" ]] || [[ "$ARCH" == "aarch64" ]]; then
        ARCH="arm64"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi

    GO_PACKAGE="go${GO_VERSION}.${OS}-${ARCH}.tar.gz"
    DOWNLOAD_URL="https://go.dev/dl/${GO_PACKAGE}"

    echo "Downloading from ${DOWNLOAD_URL}..."
    
    # Create temp dir
    TMP_DIR=$(mktemp -d)
    cd $TMP_DIR
    
    # Download Go
    curl -LO "$DOWNLOAD_URL"
    
    # Remove old installation if exists
    sudo rm -rf /usr/local/go
    
    # Extract new Go
    sudo tar -C /usr/local -xzf "$GO_PACKAGE"
    
    # Clean up
    rm "$GO_PACKAGE"
    
    # Add Go to PATH for this script session
    export PATH=$PATH:/usr/local/go/bin
    
    echo -e "${GREEN}Go installed successfully.${NC}"
else
    echo -e "${GREEN}Go is already installed.$(go version)${NC}"
fi

# --- 2. Build and Install Local CLI ---
echo "Cloning $REPO..."
BUILD_DIR=$(mktemp -d)
git clone https://github.com/$REPO.git $BUILD_DIR
cd $BUILD_DIR

echo "Building $BINARY_NAME..."
GOOS=$(uname -s | tr '[:upper:]' '[:lower:]')
GOARCH=$(uname -m)
if [[ "$GOARCH" == "x86_64" ]]; then GOARCH="amd64"; fi
if [[ "$GOARCH" == "arm64" ]] || [[ "$GOARCH" == "aarch64" ]]; then GOARCH="arm64"; fi

# Build
GOOS=$GOOS GOARCH=$GOARCH go build -o $BINARY_NAME

echo "Installing $BINARY_NAME to $INSTALL_DIR (requires sudo)..."
sudo mv $BINARY_NAME $INSTALL_DIR/

# Cleanup
cd /
rm -rf $BUILD_DIR

echo -e "${GREEN}--------------------------------------------------${NC}"
echo -e "${GREEN}Installation Complete!${NC}"
echo -e "Run '${BINARY_NAME} --help' to get started."
echo -e "${GREEN}--------------------------------------------------${NC}"