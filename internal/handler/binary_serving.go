package handler

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"text/template"

	"github.com/labstack/echo/v4"
)

//go:embed install.sh.tmpl
var installScriptTemplate embed.FS

// BinaryInfo represents information about a downloadable binary
type BinaryInfo struct {
	OS       string
	Arch     string
	Filename string
	URL      string
}

// HandleBinaryDownload redirects to GitHub releases for CLI binary downloads
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
		filename = fmt.Sprintf("drop_windows_%s.zip", arch)
	} else {
		filename = fmt.Sprintf("drop_%s_%s.tar.gz", osName, arch)
	}

	// Redirect to GitHub releases
	githubURL := fmt.Sprintf("https://github.com/marianozunino/drop/releases/latest/download/%s", filename)
	return c.Redirect(http.StatusFound, githubURL)
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
		// Serve the install script using embedded template
		tmplContent, err := installScriptTemplate.ReadFile("install.sh.tmpl")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to read install script template")
		}

		tmpl, err := template.New("install").Parse(string(tmplContent))
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to parse install script template")
		}

		var script strings.Builder
		err = tmpl.Execute(&script, map[string]string{
			"BaseURL": baseURL,
		})
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to execute install script template")
		}

		c.Response().Header().Set("Content-Type", "text/plain")
		return c.String(http.StatusOK, script.String())
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

	// Redirect to GitHub releases
	githubURL := fmt.Sprintf("https://github.com/marianozunino/drop/releases/latest/download/drop_%s.tar.gz", platform)
	if strings.Contains(strings.ToLower(userAgent), "windows") {
		githubURL = fmt.Sprintf("https://github.com/marianozunino/drop/releases/latest/download/drop_windows_amd64.zip")
	}
	return c.Redirect(http.StatusFound, githubURL)
}
