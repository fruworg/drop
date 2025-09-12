package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/model"
	"github.com/marianozunino/drop/internal/utils"
)

// HandleFileManagement handles file management operations (delete, update expiration)
func (h *Handler) HandleFileManagement(c echo.Context) error {
	if err := h.parseRequestForm(c); err != nil {
		log.Printf("Info: Non-form request or parsing error: %v", err)
	}

	filename := c.Param("filename")
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	token := c.FormValue("token")
	if token == "" {
		log.Printf("Missing management token for %s by %s", filename, c.RealIP())
		return c.String(http.StatusBadRequest, "Missing management token")
	}

	meta, err := h.db.GetMetadataByToken(token)
	if err != nil {
		log.Printf("Invalid management token for %s by %s: %v", filename, c.RealIP(), err)
		return c.String(http.StatusUnauthorized, "Invalid management token")
	}

	// Verify that the token belongs to the requested resource
	// For URL shorteners, check if the filename matches the ResourcePath
	// For regular files, check if the filename matches the ResourcePath (without extension)
	if meta.IsFile() {
		expectedFilename := filepath.Base(meta.ResourcePath)
		if expectedFilename != filename {
			log.Printf("Token mismatch: token belongs to %s but requested %s", expectedFilename, filename)
			return c.String(http.StatusUnauthorized, "Invalid management token")
		}
	} else {
		if meta.ResourcePath != filename {
			log.Printf("Token mismatch: token belongs to %s but requested %s", meta.ResourcePath, filename)
			return c.String(http.StatusUnauthorized, "Invalid management token")
		}
	}

	if _, deleteRequested := c.Request().Form["delete"]; deleteRequested {
		if meta.IsURLShortener {
			return h.handleURLShortenerDelete(c, filename, meta)
		} else if meta.IsFile() {
			filePath := meta.ResourcePath
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				log.Printf("Physical file %s not found, cleaning up metadata", filePath)
				if err := h.db.DeleteMetadata(&meta); err != nil {
					log.Printf("Warning: Failed to delete orphaned metadata for %s: %v", filename, err)
				}
				return c.String(http.StatusNotFound, "File not found")
			}
			return h.handleFileDelete(c, filePath, meta)
		}
	}

	if expiresStr := c.FormValue("expires"); expiresStr != "" {
		return h.handleExpirationUpdate(c, expiresStr, meta)
	}

	return c.String(http.StatusBadRequest, "No valid operation specified. Use 'delete' or 'expires'.")
}

// parseRequestForm attempts to parse the request form
func (h *Handler) parseRequestForm(c echo.Context) error {
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		return c.Request().ParseForm()
	}
	return nil
}

// handleFileDelete handles the file deletion operation
func (h *Handler) handleFileDelete(c echo.Context, filePath string, meta model.FileMetadata) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Error: Failed to delete file %s for user %s: %v", filePath, c.RealIP(), err)
		return c.String(http.StatusInternalServerError, "Failed to delete file")
	}

	if err := h.db.DeleteMetadata(&meta); err != nil {
		log.Printf("Warning: Failed to delete metadata for %s by user %s: %v", filePath, c.RealIP(), err)
	}

	log.Printf("File deleted: %s by %s", filePath, c.RealIP())
	return c.String(http.StatusOK, "File deleted successfully")
}

// handleExpirationUpdate handles updating the file expiration time
func (h *Handler) handleExpirationUpdate(c echo.Context, expiresStr string, meta model.FileMetadata) error {
	expirationDate, err := utils.ParseExpirationTime(expiresStr)
	if err != nil {
		log.Printf("Invalid expiration format for %s by %s: %v", meta.ResourcePath, c.RealIP(), err)
		return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
	}

	meta.ExpiresAt = &expirationDate

	if err = h.db.StoreMetadata(&meta); err != nil {
		log.Printf("Error: Failed to update expiration for %s by %s: %v", meta.ResourcePath, c.RealIP(), err)
		return c.String(http.StatusInternalServerError, "Failed to update expiration")
	}

	log.Printf("Expiration updated: %s to %v by %s", meta.ResourcePath, expirationDate, c.RealIP())
	return c.String(http.StatusOK, "Expiration updated successfully")
}

// handleURLShortenerDelete handles the deletion of URL shorteners
func (h *Handler) handleURLShortenerDelete(c echo.Context, shortID string, meta model.FileMetadata) error {
	// For URL shorteners, we only need to delete the metadata
	// There's no physical file to remove
	if err := h.db.DeleteMetadata(&meta); err != nil {
		log.Printf("Warning: Failed to delete metadata for URL shortener %s by user %s: %v", shortID, c.RealIP(), err)
		return c.String(http.StatusInternalServerError, "Failed to delete URL shortener")
	}

	log.Printf("URL shortener deleted: %s by %s", shortID, c.RealIP())
	return c.String(http.StatusOK, "URL shortener deleted successfully")
}
