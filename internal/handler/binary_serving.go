package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/labstack/echo/v4"
)

// BinaryInfo represents information about a downloadable binary
type BinaryInfo struct {
	OS       string
	Arch     string
	Filename string
	URL      string
}

// HandleBinaryDownload serves the client binary for download
func (h *Handler) HandleBinaryDownload(c echo.Context) error {
	platform := c.Param("platform")
	if platform == "" {
		return c.String(http.StatusBadRequest, "Platform parameter is required")
	}

	// Parse platform parameter (e.g., "linux-amd64", "darwin-arm64", "windows-amd64")
	parts := strings.Split(platform, "-")
	if len(parts) != 2 {
		return c.String(http.StatusBadRequest, "Invalid platform format. Use: os-arch (e.g., linux-amd64)")
	}

	osName := parts[0]
	arch := parts[1]

	// Validate OS
	validOS := map[string]bool{
		"linux":   true,
		"darwin":  true,
		"windows": true,
	}
	if !validOS[osName] {
		return c.String(http.StatusBadRequest, "Unsupported OS. Supported: linux, darwin, windows")
	}

	// Validate architecture
	validArch := map[string]bool{
		"amd64": true,
		"arm64": true,
	}
	if !validArch[arch] {
		return c.String(http.StatusBadRequest, "Unsupported architecture. Supported: amd64, arm64")
	}

	// Determine filename
	var filename string
	if osName == "windows" {
		filename = fmt.Sprintf("drop-%s-%s.exe", osName, arch)
	} else {
		filename = fmt.Sprintf("drop-%s-%s", osName, arch)
	}

	// Check if binary exists
	binaryPath := filepath.Join("/app/binaries", filename)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "Binary not found for this platform")
	}

	// Set appropriate headers for download
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Response().Header().Set("Content-Type", "application/octet-stream")

	// Serve the file
	return c.File(binaryPath)
}

// HandleBinaryList returns a list of available binaries
func (h *Handler) HandleBinaryList(c echo.Context) error {
	binariesDir := "/app/binaries"

	// Read the binaries directory
	files, err := os.ReadDir(binariesDir)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to read binaries directory")
	}

	var binaries []BinaryInfo
	baseURL := strings.TrimSuffix(h.cfg.BaseURL, "/")

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()

		// Parse filename to extract OS and architecture
		var os, arch string
		if strings.HasSuffix(filename, ".exe") {
			// Windows binary
			parts := strings.Split(strings.TrimSuffix(filename, ".exe"), "-")
			if len(parts) >= 3 {
				os = parts[1]
				arch = parts[2]
			}
		} else {
			// Unix binary
			parts := strings.Split(filename, "-")
			if len(parts) >= 3 {
				os = parts[1]
				arch = parts[2]
			}
		}

		if os != "" && arch != "" {
			platform := fmt.Sprintf("%s-%s", os, arch)
			binaries = append(binaries, BinaryInfo{
				OS:       os,
				Arch:     arch,
				Filename: filename,
				URL:      fmt.Sprintf("%s/binaries/%s", baseURL, platform),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"binaries":         binaries,
		"current_platform": fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH),
	})
}

// HandleBinaryAutoDetect serves the install script or appropriate binary based on User-Agent
func (h *Handler) HandleBinaryAutoDetect(c echo.Context) error {
	userAgent := c.Request().Header.Get("User-Agent")
	baseURL := strings.TrimSuffix(h.cfg.BaseURL, "/")

	// Check if this is a shell request (pipe to sh)
	acceptHeader := c.Request().Header.Get("Accept")
	if strings.Contains(acceptHeader, "text/plain") || strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "wget") {
		// Serve the install script
		installScript := fmt.Sprintf(`#!/bin/bash
# MZ.DROP Client Installer
# Auto-generated install script

set -e

BASE_URL="%s"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="drop"

# Detect platform
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

# Download and install
platform=$(detect_platform)
echo "Detected platform: $platform"
echo "Downloading MZ.DROP client..."

mkdir -p "$INSTALL_DIR"

if command -v curl >/dev/null 2>&1; then
    curl -L "$BASE_URL/binaries/$platform" -o "$INSTALL_DIR/$BINARY_NAME"
elif command -v wget >/dev/null 2>&1; then
    wget -O "$INSTALL_DIR/$BINARY_NAME" "$BASE_URL/binaries/$platform"
else
    echo "Error: Neither curl nor wget found. Please install one of them."
    exit 1
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

# Add to PATH if not already there
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo "Adding $INSTALL_DIR to PATH..."
    case "$SHELL" in
        */bash) echo 'export PATH="$PATH:'$INSTALL_DIR'"' >> "$HOME/.bashrc" ;;
        */zsh) echo 'export PATH="$PATH:'$INSTALL_DIR'"' >> "$HOME/.zshrc" ;;
        *) echo 'export PATH="$PATH:'$INSTALL_DIR'"' >> "$HOME/.profile" ;;
    esac
    echo "Please run 'source ~/.bashrc' (or ~/.zshrc) or restart your terminal."
fi

echo "Installation completed! You can now use 'drop' command."
echo "Try: drop --help"
`, baseURL)

		c.Response().Header().Set("Content-Type", "text/plain")
		return c.String(http.StatusOK, installScript)
	}

	// Simple detection based on User-Agent for direct binary download
	var platform string
	if strings.Contains(strings.ToLower(userAgent), "windows") {
		platform = "windows-amd64"
	} else if strings.Contains(strings.ToLower(userAgent), "mac") || strings.Contains(strings.ToLower(userAgent), "darwin") {
		// Try to detect ARM vs Intel Mac
		if strings.Contains(strings.ToLower(userAgent), "arm64") || strings.Contains(strings.ToLower(userAgent), "apple") {
			platform = "darwin-arm64"
		} else {
			platform = "darwin-amd64"
		}
	} else {
		// Default to Linux AMD64
		platform = "linux-amd64"
	}

	// Redirect to the appropriate binary
	return c.Redirect(http.StatusFound, fmt.Sprintf("%s/binaries/%s", baseURL, platform))
}
