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
