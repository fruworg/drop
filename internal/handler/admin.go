package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/model"
	"github.com/marianozunino/drop/internal/utils"
	"github.com/marianozunino/drop/templates"
)

// HandleAdminDashboard serves the admin dashboard
func (h *Handler) HandleAdminDashboard(c echo.Context) error {
	if !h.isAdminAuthenticated(c) {
		return c.String(http.StatusUnauthorized, "Unauthorized")
	}

	sortField := c.QueryParam("sort")
	sortDirection := c.QueryParam("dir")
	searchQuery := strings.TrimSpace(c.QueryParam("search"))
	cursor := c.QueryParam("cursor")
	limit := 10

	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 200 {
			limit = parsedLimit
		}
	}

	validSortFields := map[string]bool{
		"filename":     true,
		"originalName": true,
		"size":         true,
		"uploadDate":   true,
		"expires":      true,
	}

	if sortField == "" || !validSortFields[sortField] {
		sortField = "uploadDate"
	}

	if sortDirection != "asc" && sortDirection != "desc" {
		sortDirection = "desc"
	}

	files, nextCursor, err := h.getAllFilesForAdminSortedAndFilteredWithPagination(sortField, sortDirection, searchQuery, limit, cursor)
	if err != nil {
		log.Printf("Error getting files for admin: %v", err)
		return c.String(http.StatusInternalServerError, "Failed to get files")
	}

	totalFiles, err := h.db.CountMetadataFiltered("")
	if err != nil {
		log.Printf("Error getting total file count: %v", err)
		totalFiles = 0
	}

	matchingFiles := len(files)
	if searchQuery != "" {
		matchingFiles, err = h.db.CountMetadataFiltered(searchQuery)
		if err != nil {
			log.Printf("Error getting matching file count: %v", err)
			matchingFiles = len(files)
		}
	}

	totalSize, err := h.db.GetTotalSize()
	if err != nil {
		log.Printf("Error getting total size: %v", err)
		totalSize = 0
	}

	return templates.AdminDashboardPage(files, sortField, sortDirection, searchQuery, cursor, nextCursor, limit, totalFiles, matchingFiles, totalSize).Render(c.Request().Context(), c.Response())
}

// HandleAdminFileView shows detailed view of a single file
func (h *Handler) HandleAdminFileView(c echo.Context) error {
	if !h.isAdminAuthenticated(c) {
		return c.String(http.StatusUnauthorized, "Unauthorized")
	}

	filename := c.Param("filename")
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)
	meta, err := h.db.GetMetadataByID(filePath)
	if err != nil {
		return c.String(http.StatusNotFound, "File not found")
	}

	adminFile := h.enrichFileMetadata(meta)
	return templates.AdminFileView(adminFile).Render(c.Request().Context(), c.Response())
}

// HandleAdminFileDelete deletes a file from admin panel
func (h *Handler) HandleAdminFileDelete(c echo.Context) error {
	if !h.isAdminAuthenticated(c) {
		return c.String(http.StatusUnauthorized, "Unauthorized")
	}

	filename := c.Param("filename")
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)

	meta, err := h.db.GetMetadataByID(filePath)
	if err != nil {
		return c.String(http.StatusNotFound, "File not found")
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Error deleting file %s: %v", filePath, err)
		return c.String(http.StatusInternalServerError, "Failed to delete file")
	}

	if err := h.db.DeleteMetadata(&meta); err != nil {
		log.Printf("Warning: Failed to delete metadata for %s: %v", filePath, err)
	}

	log.Printf("Admin deleted file: %s", filePath)

	redirectURL := "/admin"
	params := []string{}

	if searchQuery := c.QueryParam("search"); searchQuery != "" {
		params = append(params, "search="+searchQuery)
	}
	if sortField := c.QueryParam("sort"); sortField != "" {
		params = append(params, "sort="+sortField)
	}
	if sortDirection := c.QueryParam("dir"); sortDirection != "" {
		params = append(params, "dir="+sortDirection)
	}
	if limit := c.QueryParam("limit"); limit != "" {
		params = append(params, "limit="+limit)
	}

	if len(params) > 0 {
		redirectURL += "?" + strings.Join(params, "&")
	}

	return c.Redirect(http.StatusSeeOther, redirectURL)
}

// HandleAdminFileUpdate updates file metadata
func (h *Handler) HandleAdminFileUpdate(c echo.Context) error {
	if !h.isAdminAuthenticated(c) {
		return c.String(http.StatusUnauthorized, "Unauthorized")
	}

	filename := c.Param("filename")
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)
	meta, err := h.db.GetMetadataByID(filePath)
	if err != nil {
		return c.String(http.StatusNotFound, "File not found")
	}

	if expiresStr := c.FormValue("expires"); expiresStr != "" {
		expirationDate, err := utils.ParseExpirationTime(expiresStr)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
		}
		meta.ExpiresAt = &expirationDate
	}

	if c.FormValue("one_time_view") == "on" {
		meta.OneTimeView = true
	} else {
		meta.OneTimeView = false
	}

	if originalName := c.FormValue("original_name"); originalName != "" {
		meta.OriginalName = originalName
	}

	if err := h.db.StoreMetadata(&meta); err != nil {
		log.Printf("Error updating metadata for %s: %v", filePath, err)
		return c.String(http.StatusInternalServerError, "Failed to update file")
	}

	log.Printf("Admin updated file: %s", filePath)
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/file/%s", filename))
}

// HandleAdminLogin handles admin login (simple implementation)
func (h *Handler) HandleAdminLogin(c echo.Context) error {
	if c.Request().Method == "GET" {
		return templates.AdminLogin().Render(c.Request().Context(), c.Response())
	}

	username := c.FormValue("username")
	password := c.FormValue("password")
	if h.cfg.ValidateAdminPassword(username, password) {
		c.SetCookie(&http.Cookie{
			Name:     "admin_auth",
			Value:    "true",
			Path:     "/",
			MaxAge:   3600,
			HttpOnly: true,
		})
		return c.Redirect(http.StatusSeeOther, "/admin")
	}

	return c.String(http.StatusUnauthorized, "Invalid username or password")
}

// HandleAdminLogout handles admin logout
func (h *Handler) HandleAdminLogout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     "admin_auth",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	return c.Redirect(http.StatusSeeOther, "/admin/login")
}

// isAdminAuthenticated checks if the user is authenticated as admin
func (h *Handler) isAdminAuthenticated(c echo.Context) bool {
	cookie, err := c.Cookie("admin_auth")
	if err != nil {
		return false
	}
	return cookie.Value == "true"
}

// getAllFilesForAdminSortedAndFilteredWithPagination retrieves files with pagination
func (h *Handler) getAllFilesForAdminSortedAndFilteredWithPagination(sortField, sortDirection, searchQuery string, limit int, cursor string) ([]model.AdminFileInfo, string, error) {
	metadatas, nextCursor, err := h.db.ListMetadataFilteredAndSortedWithPagination(searchQuery, sortField, sortDirection, limit, cursor)
	if err != nil {
		return nil, "", err
	}

	var adminFiles []model.AdminFileInfo
	for _, meta := range metadatas {
		adminFile := h.enrichFileMetadata(meta)
		adminFiles = append(adminFiles, adminFile)
	}

	return adminFiles, nextCursor, nil
}

// getAllFilesForAdminSortedAndFiltered retrieves all files with admin-specific information, filters them, and sorts them
func (h *Handler) getAllFilesForAdminSortedAndFiltered(sortField, sortDirection, searchQuery string) ([]model.AdminFileInfo, error) {
	metadatas, err := h.db.ListMetadataFilteredAndSorted(searchQuery, sortField, sortDirection)
	if err != nil {
		return nil, err
	}

	var adminFiles []model.AdminFileInfo
	for _, meta := range metadatas {
		adminFile := h.enrichFileMetadata(meta)
		adminFiles = append(adminFiles, adminFile)
	}

	return adminFiles, nil
}

// getAllFilesForAdminSorted retrieves all files with admin-specific information and sorts them
func (h *Handler) getAllFilesForAdminSorted(sortField, sortDirection string) ([]model.AdminFileInfo, error) {
	return h.getAllFilesForAdminSortedAndFiltered(sortField, sortDirection, "")
}

// getAllFilesForAdmin retrieves all files with admin-specific information
func (h *Handler) getAllFilesForAdmin() ([]model.AdminFileInfo, error) {
	return h.getAllFilesForAdminSorted("uploadDate", "desc")
}

// enrichFileMetadata adds admin-specific information to file metadata
func (h *Handler) enrichFileMetadata(meta model.FileMetadata) model.AdminFileInfo {
	adminFile := model.AdminFileInfo{
		FileMetadata: meta,
		IsExpired:    false,
		DaysLeft:     0,
	}

	if meta.ExpiresAt != nil && !meta.ExpiresAt.IsZero() {
		now := time.Now()
		if meta.ExpiresAt.Before(now) {
			adminFile.IsExpired = true
		} else {
			adminFile.DaysLeft = int(meta.ExpiresAt.Sub(now).Hours() / 24)
		}
	}

	return adminFile
}
