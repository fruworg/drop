#!/bin/bash

# MZ.DROP Client Installer
# This script downloads and installs the appropriate client binary for your platform

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
BASE_URL="http://localhost:8080"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="drop"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to detect platform
detect_platform() {
    local os arch
    
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW32*|MSYS*|MINGW*) os="windows" ;;
        *)          os="unknown" ;;
    esac
    
    case "$(uname -m)" in
        x86_64)     arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)          arch="unknown" ;;
    esac
    
    echo "${os}-${arch}"
}

# Function to download binary
download_binary() {
    local platform="$1"
    local url="$2"
    local output="$3"
    
    print_status "Downloading client for $platform..."
    
    if command -v curl >/dev/null 2>&1; then
        curl -L "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "$output" "$url"
    else
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    if [ $? -eq 0 ]; then
        print_success "Download completed"
    else
        print_error "Download failed"
        exit 1
    fi
}

# Function to make binary executable
make_executable() {
    local binary="$1"
    
    print_status "Making binary executable..."
    chmod +x "$binary"
    
    if [ $? -eq 0 ]; then
        print_success "Binary is now executable"
    else
        print_error "Failed to make binary executable"
        exit 1
    fi
}

# Function to add to PATH
add_to_path() {
    local install_dir="$1"
    local shell_rc=""
    
    # Detect shell and set appropriate rc file
    case "$SHELL" in
        */bash) shell_rc="$HOME/.bashrc" ;;
        */zsh) shell_rc="$HOME/.zshrc" ;;
        */fish) shell_rc="$HOME/.config/fish/config.fish" ;;
        *) shell_rc="$HOME/.profile" ;;
    esac
    
    # Check if install_dir is already in PATH
    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        print_status "Adding $install_dir to PATH in $shell_rc"
        
        if [ -f "$shell_rc" ]; then
            echo "" >> "$shell_rc"
            echo "# MZ.DROP Client" >> "$shell_rc"
            echo "export PATH=\"\$PATH:$install_dir\"" >> "$shell_rc"
            print_success "Added to PATH. Please run 'source $shell_rc' or restart your terminal."
        else
            print_warning "Could not find $shell_rc. Please add $install_dir to your PATH manually."
        fi
    else
        print_success "Install directory is already in PATH"
    fi
}

# Main installation function
install_client() {
    local platform
    local download_url
    local binary_path
    
    # Detect platform
    platform=$(detect_platform)
    print_status "Detected platform: $platform"
    
    # Construct download URL
    download_url="${BASE_URL}/binaries/${platform}"
    
    # Create install directory
    print_status "Creating install directory: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"
    
    # Set binary path
    binary_path="$INSTALL_DIR/$BINARY_NAME"
    
    # Download binary
    download_binary "$platform" "$download_url" "$binary_path"
    
    # Make executable
    make_executable "$binary_path"
    
    # Add to PATH
    add_to_path "$INSTALL_DIR"
    
    # Test installation
    print_status "Testing installation..."
    if "$binary_path" --version >/dev/null 2>&1; then
        print_success "Installation completed successfully!"
        print_status "You can now use 'drop' command from anywhere"
        print_status "Try: drop --help"
    else
        print_warning "Installation completed, but binary test failed"
        print_status "You can try running: $binary_path --help"
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            BASE_URL="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --help|-h)
            echo "MZ.DROP Client Installer"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --url URL          Base URL of the MZ.DROP server (default: http://localhost:8080)"
            echo "  --install-dir DIR   Installation directory (default: \$HOME/.local/bin)"
            echo "  --help, -h         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                                    # Install with defaults"
            echo "  $0 --url https://drop.example.com     # Install from custom server"
            echo "  $0 --install-dir /usr/local/bin      # Install to system directory"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run installation
print_status "Starting MZ.DROP client installation..."
print_status "Server URL: $BASE_URL"
print_status "Install directory: $INSTALL_DIR"

install_client
