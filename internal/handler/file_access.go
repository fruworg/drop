// Modified file_access.go with the fix for preview bots
// This version uses a configurable list of preview bots

package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/model"
	"github.com/marianozunino/drop/templates"
)

// HandleFileAccess serves the requested file to the client
func (h *Handler) HandleFileAccess(c echo.Context) error {
	filePath, err := h.validateAndResolvePath(c)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			log.Printf("Warning: File access error: %v", err)
			return c.String(http.StatusNotFound, "File not found")
		}
		log.Printf("Error: File access error: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Get file metadata
	meta, err := h.getFileMetadata(filePath)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to get metadata")
	}

	// Check if this is a preview bot request for a one-time file
	isPreviewBot := h.isLinkPreviewBot(c.Request())
	if meta.OneTimeView && isPreviewBot {
		return h.servePlaceholderForPreviewBot(c)
	}

	// Set response headers
	h.setResponseHeaders(c, meta)

	contentDisposition := c.Response().Header().Get("Content-Disposition")

	if strings.Contains(contentDisposition, "inline") {
		err = c.Inline(filePath, meta.OriginalName)
	} else {
		err = c.Attachment(filePath, meta.OriginalName)
	}

	if err == nil && meta.OneTimeView {
		err = h.deleteOneTimeViewFile(filePath, meta)
	}

	return err
}

// isLinkPreviewBot determines if the request is likely from a preview bot
func (h *Handler) isLinkPreviewBot(req *http.Request) bool {
	userAgent := req.Header.Get("User-Agent")

	// Use configured bot patterns from config if available
	if h.cfg.PreviewBots != nil && len(h.cfg.PreviewBots) > 0 {
		userAgentLower := strings.ToLower(userAgent)
		for _, bot := range h.cfg.PreviewBots {
			if strings.Contains(userAgentLower, strings.ToLower(bot)) {
				log.Printf("Preview bot detected: %s", userAgent)
				return true
			}
		}
	} else {
		// Fallback to default list if no configuration is available
		defaultBots := []string{
			"slack",
			"slackbot",
			"facebookexternalhit",
			"twitterbot",
			"discordbot",
			"whatsapp",
			"googlebot",
			"linkedinbot",
			"telegram",
			"skype",
			"viber",
		}

		userAgentLower := strings.ToLower(userAgent)
		for _, bot := range defaultBots {
			if strings.Contains(userAgentLower, bot) {
				log.Printf("Preview bot detected (using default list): %s", userAgent)
				return true
			}
		}
	}

	// Additional detection for bots that don't identify themselves clearly
	// Check for common preview bot behavior
	if req.Header.Get("X-Purpose") == "preview" ||
		req.Header.Get("X-Facebook") != "" ||
		req.Header.Get("X-Forwarded-For") != "" && strings.Contains(req.Header.Get("Accept"), "image/*") {
		log.Printf("Preview bot behavior detected from headers")
		return true
	}

	return false
}

// servePlaceholderForPreviewBot returns a small placeholder response for preview bots
// to avoid consuming one-time links
func (h *Handler) servePlaceholderForPreviewBot(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	err := templates.Preview().Render(context.Background(), c.Response())
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error rendering template: %v", err))
	}

	return nil
}

func (h *Handler) deleteOneTimeViewFile(path string, meta model.FileMetadata) error {
	time.Sleep(100 * time.Millisecond)

	log.Printf("Deleting one-time view file: %s", path)
	var err error

	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("Error: Failed to delete one-time view file %s: %v", path, err)
	}

	if err = h.db.DeleteMetadata(&meta); err != nil {
		log.Printf("Warning: Failed to delete metadata for one-time view file %s: %v", path, err)
	}

	return err
}

// validateAndResolvePath validates and resolves the file path from the request
func (h *Handler) validateAndResolvePath(c echo.Context) (string, error) {
	filename := c.Param("filename")

	parts := strings.SplitN(filename, "/", 2)
	filename = parts[0]
	filePath := filepath.Join(h.cfg.UploadPath, filename)

	if _, err := os.Stat(filePath); err != nil {
		return "", err
	}

	return filePath, nil
}

// getFileMetadata retrieves metadata for the specified file
func (h *Handler) getFileMetadata(filePath string) (model.FileMetadata, error) {
	meta, err := h.db.GetMetadataByID(filePath)
	if err != nil {
		log.Printf("Error: Failed to get metadata: %v", err)
		return model.FileMetadata{}, err
	}
	return meta, nil
}

// setResponseHeaders sets appropriate response headers based on file metadata
func (h *Handler) setResponseHeaders(c echo.Context, meta model.FileMetadata) {
	contentType := "application/octet-stream"
	if meta.ContentType != "" {
		contentType = meta.ContentType
	}

	// Set expiration header if applicable
	if !meta.ExpiresAt.IsZero() {
		expiresMs := meta.ExpiresAt.UnixNano() / int64(time.Millisecond)
		c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
	}

	c.Response().Header().Set("Content-Type", contentType)

	if meta.OneTimeView {
		c.Response().Header().Set("X-One-Time-View", "true")
	}

	log.Printf("Content-Type: %s", contentType)

	// Set content disposition based on content type
	if shouldDisplayInline(contentType) {
		c.Response().Header().Set("Content-Disposition", "inline")
	}
}

// shouldDisplayInline determines if the content should be displayed inline in the browser
func shouldDisplayInline(contentType string) bool {
	return strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/") ||
		strings.HasPrefix(contentType, "image/") ||
		contentType == "application/pdf" ||
		strings.HasPrefix(contentType, "text/")
}
