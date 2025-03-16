package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/model"
)

// HandleFileAccess serves the requested file to the client
func (h *Handler) HandleFileAccess(c echo.Context) error {
	filePath, err := h.validateAndResolvePath(c)
	if err != nil {
		return err
	}

	// Get file metadata
	meta, err := h.getFileMetadata(filePath)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to get metadata")
	}

	// Set response headers
	h.setResponseHeaders(c, meta)

	contentDisposition := c.Response().Header().Get("Content-Disposition")

	if strings.Contains(contentDisposition, "inline") {
		return c.Inline(filePath, meta.OriginalName)
	}

	// Serve the file
	return c.Attachment(filePath, meta.OriginalName)
}

// validateAndResolvePath validates and resolves the file path from the request
func (h *Handler) validateAndResolvePath(c echo.Context) (string, error) {
	filename := c.Param("filename")

	parts := strings.SplitN(filename, "/", 2)
	filename = parts[0]

	if strings.Contains(filename, "..") {
		return "", c.String(http.StatusBadRequest, "Invalid file path")
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", c.String(http.StatusNotFound, "File not found")
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
