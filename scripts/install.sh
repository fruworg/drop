#!/bin/sh
# Install script for Drop CLI
# Usage: curl -fsSL https://raw.githubusercontent.com/marianozunino/drop/main/scripts/install.sh | sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY_NAME="drop"
REPO="marianozunino/drop"

# Detect platform
detect_platform() {
    os=""
    arch=""
    
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

# Get latest release version
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4
    else
        echo "latest"
    fi
}

# Download and install binary
install_binary() {
    platform=$1
    version=$2
    platform_file="${platform//-/_}"
    
    # Determine file extension based on platform
    if [ "$platform" != "${platform#*windows}" ]; then
        file_ext="zip"
        download_url="https://github.com/${REPO}/releases/download/${version}/drop_${platform_file}.${file_ext}"
    else
        file_ext="tar.gz"
        download_url="https://github.com/${REPO}/releases/download/${version}/drop_${platform_file}.${file_ext}"
    fi
    
    echo -e "${BLUE}Downloading Drop CLI ${version} for ${platform}...${NC}"
    
    # Create temp directory
    temp_dir=$(mktemp -d)
    cd "$temp_dir"
    
    # Download and extract
    if command -v curl >/dev/null 2>&1; then
        curl -L "$download_url" -o "drop.${file_ext}"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "drop.${file_ext}" "$download_url"
    else
        echo -e "${RED}Error: Neither curl nor wget found. Please install one of them.${NC}"
        exit 1
    fi
    
    # Extract based on file type
    if [ "$file_ext" = "zip" ]; then
        unzip "drop.${file_ext}"
    else
        tar -xzf "drop.${file_ext}"
    fi
    
    # Install binary
    mkdir -p "$INSTALL_DIR"
    mv "$BINARY_NAME" "$INSTALL_DIR/"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    # Cleanup
    cd /
    rm -rf "$temp_dir"
    
    echo -e "${GREEN}✓ Installed Drop CLI to $INSTALL_DIR/$BINARY_NAME${NC}"
}

# Add to PATH
add_to_path() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *) 
            echo -e "${YELLOW}Adding $INSTALL_DIR to PATH...${NC}"
            
            # Detect shell
            case "$SHELL" in
                */bash)
                    echo 'export PATH="$PATH:'$INSTALL_DIR'"' >> "$HOME/.bashrc"
                    echo -e "${BLUE}Please run: source ~/.bashrc${NC}"
                    ;;
                */zsh)
                    echo 'export PATH="$PATH:'$INSTALL_DIR'"' >> "$HOME/.zshrc"
                    echo -e "${BLUE}Please run: source ~/.zshrc${NC}"
                    ;;
                *)
                    echo 'export PATH="$PATH:'$INSTALL_DIR'"' >> "$HOME/.profile"
                    echo -e "${BLUE}Please run: source ~/.profile${NC}"
                    ;;
            esac
            ;;
    esac
}

# Main installation
main() {
    echo -e "${GREEN}🚀 Installing Drop CLI...${NC}"
    
    # Detect platform
    platform=$(detect_platform)
    if [ "$platform" = "unknown-unknown" ]; then
        echo -e "${RED}Error: Unsupported platform: $platform${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Detected platform: $platform${NC}"
    
    # Get latest version
    version=$(get_latest_version)
    echo -e "${BLUE}Latest version: $version${NC}"
    
    # Install binary
    install_binary "$platform" "$version"
    
    # Add to PATH
    add_to_path
    
    echo ""
    echo -e "${GREEN}🎉 Installation completed!${NC}"
    echo -e "${BLUE}Try: $BINARY_NAME --help${NC}"
    echo -e "${BLUE}Or: $BINARY_NAME --version${NC}"
}

# Run main function
main "$@"
