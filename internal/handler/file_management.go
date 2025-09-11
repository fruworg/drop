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

	filePath := filepath.Join(h.cfg.UploadPath, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "File not found")
	}

	// Get and validate management token
	meta, err := h.validateManagementToken(c, filePath)
	if err != nil {
		return err
	}

	// Handle delete operation
	if _, deleteRequested := c.Request().Form["delete"]; deleteRequested {
		return h.handleFileDelete(c, filePath, meta)
	}

	// Handle expiration update
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

// validateManagementToken validates the management token for file operations
func (h *Handler) validateManagementToken(c echo.Context, filePath string) (model.FileMetadata, error) {
	var meta model.FileMetadata

	token := c.FormValue("token")
	if token == "" {
		log.Printf("Missing management token for %s by %s", filePath, c.RealIP())
		return meta, c.String(http.StatusBadRequest, "Missing management token")
	}

	var err error
	meta, err = h.db.GetMetadataByID(filePath)
	if err != nil {
		log.Printf("Warning: Failed to get metadata for %s by %s: %v", filepath.Base(filePath), c.RealIP(), err)
		return meta, c.String(http.StatusInternalServerError, "Failed to get metadata")
	}

	if meta.Token != token {
		log.Printf("Invalid management token for %s by %s", filePath, c.RealIP())
		return meta, c.String(http.StatusUnauthorized, "Invalid management token")
	}

	return meta, nil
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
		log.Printf("Invalid expiration format for %s by %s: %v", meta.FilePath, c.RealIP(), err)
		return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
	}

	meta.ExpiresAt = &expirationDate

	if err = h.db.StoreMetadata(&meta); err != nil {
		log.Printf("Error: Failed to update expiration for %s by %s: %v", meta.FilePath, c.RealIP(), err)
		return c.String(http.StatusInternalServerError, "Failed to update expiration")
	}

	log.Printf("Expiration updated: %s to %v by %s", meta.FilePath, expirationDate, c.RealIP())
	return c.String(http.StatusOK, "Expiration updated successfully")
}
