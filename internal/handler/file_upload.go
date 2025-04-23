package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
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
)

// HandleUpload processes file uploads, either from a form or URL
func (h *Handler) HandleUpload(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, h.cfg.MaxSizeToBytes())

	if err := h.parseRequestForm(c); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	fileInfo, err := h.extractFileContent(c)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	if fileInfo.Size == 0 {
		return c.String(http.StatusBadRequest, "Empty file")
	}

	if fileInfo.Size > h.cfg.MaxSizeToBytes() {
		return c.String(http.StatusBadRequest,
			fmt.Sprintf("File too large (max %d bytes)", h.cfg.MaxSizeToBytes()))
	}

	// Generate unique ID for the file
	useSecretId := c.FormValue("secret") != ""
	id, err := h.generateFileID(useSecretId)
	if err != nil {
		log.Printf("Error: Failed to generate ID: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Create filename and save file
	filename, filePath, err := h.saveFile(id, fileInfo)
	if err != nil {
		log.Printf("Error: Failed to save file: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Handle expiration settings
	expirationDate, err := h.determineExpiration(c, fileInfo.Size)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
	}

	// Handle delete operation
	_, oneTimeView := c.Request().Form["one_time"]

	// Store metadata
	managementToken, err := h.storeFileMetadata(filePath, filename, fileInfo, expirationDate, oneTimeView)
	if err != nil {
		log.Printf("Error: Failed to store metadata: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Return response
	return h.sendUploadResponse(c, filename, fileInfo.Size, managementToken, expirationDate)
}

// FileInfo holds information about the uploaded file
type FileInfo struct {
	Content     []byte
	Name        string
	Size        int64
	ContentType string
}

// extractFileContent obtains file content either from form upload or URL
func (h *Handler) extractFileContent(c echo.Context) (FileInfo, error) {
	var fileInfo FileInfo

	file, header, err := c.Request().FormFile("file")
	if err == nil {
		defer file.Close()

		content, err := io.ReadAll(file)
		if err != nil {
			log.Printf("Error: Failed to read file: %v", err)
			return fileInfo, fmt.Errorf("Failed to read file")
		}

		fileInfo = FileInfo{
			Content:     content,
			Name:        header.Filename,
			Size:        header.Size,
			ContentType: http.DetectContentType(content),
		}
		return fileInfo, nil
	}

	// Try URL-based upload if form upload failed
	return h.downloadFromURL(c)
}

// downloadFromURL downloads a file from the URL provided in the form
func (h *Handler) downloadFromURL(c echo.Context) (FileInfo, error) {
	var fileInfo FileInfo

	url := c.FormValue("url")
	if url == "" {
		return fileInfo, fmt.Errorf("No file or URL provided")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Error: Failed to download from URL: %v", err)
		return fileInfo, fmt.Errorf("Failed to download from URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: URL returned non-200 status: %d", resp.StatusCode)
		return fileInfo, fmt.Errorf("URL returned status %d", resp.StatusCode)
	}

	// Check content length
	if err := h.checkContentLength(resp, h.cfg.MaxSizeToBytes()); err != nil {
		return fileInfo, err
	}

	// Read content
	content, err := io.ReadAll(io.LimitReader(resp.Body, h.cfg.MaxSizeToBytes()))
	if err != nil {
		log.Printf("Error: Failed to read from URL: %v", err)
		return fileInfo, fmt.Errorf("Failed to read from URL")
	}

	// Extract filename from URL path
	fileName := h.extractFilenameFromURL(url)

	fileInfo = FileInfo{
		Content:     content,
		Name:        fileName,
		Size:        int64(len(content)),
		ContentType: resp.Header.Get("Content-Type"),
	}

	return fileInfo, nil
}

// checkContentLength validates the Content-Length header
func (h *Handler) checkContentLength(resp *http.Response, maxSize int64) error {
	contentLength := resp.Header.Get("Content-Length")
	if contentLength != "" {
		length, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			log.Printf("Warning: Invalid Content-Length: %v", err)
		} else if length > maxSize {
			return fmt.Errorf("File too large (max %d bytes)", maxSize)
		}
	}
	return nil
}

// extractFilenameFromURL extracts a filename from a URL
func (h *Handler) extractFilenameFromURL(url string) string {
	fileName := "download"
	urlPath := strings.Split(url, "/")

	if len(urlPath) > 0 && urlPath[len(urlPath)-1] != "" {
		fileName = urlPath[len(urlPath)-1]
		// Remove query parameters from filename
		if idx := strings.IndexByte(fileName, '?'); idx > 0 {
			fileName = fileName[:idx]
		}
	}

	return fileName
}

// generateFileID creates a unique ID for the file
func (h *Handler) generateFileID(useSecretId bool) (string, error) {
	if useSecretId {
		return generateID(8)
	}
	return generateID(h.cfg.IdLength)
}

// saveFile saves the file content to disk
func (h *Handler) saveFile(id string, fileInfo FileInfo) (string, string, error) {
	fileExt := filepath.Ext(fileInfo.Name)
	filename := id
	if fileExt != "" {
		filename += fileExt
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)
	if err := os.WriteFile(filePath, fileInfo.Content, 0o644); err != nil {
		return "", "", err
	}

	log.Printf("Saved file: %s (%d bytes) with ID: %s", fileInfo.Name, fileInfo.Size, id)
	return filename, filePath, nil
}

// determineExpiration determines when the file should expire
func (h *Handler) determineExpiration(c echo.Context, fileSize int64) (time.Time, error) {
	expiresStr := c.FormValue("expires")
	if expiresStr != "" {
		expirationDate, err := utils.ParseExpirationTime(expiresStr)
		if err != nil {
			return expirationDate, err
		}

		maxExpiration := h.expManager.GetExpirationDate(fileSize)
		log.Printf("Requested expiration date: %v", expirationDate)

		if expirationDate.After(maxExpiration) {
			// Do not allow expiration dates that break the retention policy
			log.Printf("Warning: Expiration date is too far in the future, using max expiration set by retention policy")
			return maxExpiration, nil
		} else if expirationDate.Before(time.Now()) {
			// Do not allow expiration dates that are in the past
			log.Printf("Warning: Expiration date is in the past, using max expiration set by retention policy")
			return maxExpiration, nil
		} else {
			// Expiration date is valid
			log.Printf("Expiration date: %v", expirationDate)
			return expirationDate, nil
		}

	}

	expirationDate := h.expManager.GetExpirationDate(fileSize)
	return expirationDate, nil
}

// storeFileMetadata creates and stores metadata for the file
func (h *Handler) storeFileMetadata(filePath, fileName string, fileInfo FileInfo, expirationDate time.Time, oneTimeView bool) (string, error) {
	managementToken, err := generateID(16)
	if err != nil {
		log.Printf("Warning: Failed to generate management token: %v", err)
		managementToken = filepath.Base(filePath)
	}

	contentType := fileInfo.ContentType
	if contentType == "" {
		contentType = http.DetectContentType(fileInfo.Content)
	}

	metadata := model.FileMetadata{
		FilePath:     filePath,
		Token:        managementToken,
		OriginalName: fileInfo.Name,
		UploadDate:   time.Now(),
		Size:         fileInfo.Size,
		ContentType:  contentType,
		OneTimeView:  oneTimeView,
	}

	if !expirationDate.IsZero() {
		metadata.ExpiresAt = expirationDate
	}

	if err := h.db.StoreMetadata(&metadata); err != nil {
		return "", err
	}

	return managementToken, nil
}

// sendUploadResponse sends the appropriate response to the client
func (h *Handler) sendUploadResponse(c echo.Context, filename string, fileSize int64, token string, expirationDate time.Time) error {
	c.Response().Header().Set("X-Token", token)
	fileURL := h.expManager.Config.BaseURL + filename

	if !expirationDate.IsZero() {
		expiresMs := expirationDate.UnixNano() / int64(time.Millisecond)
		c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
	}

	if strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		response := map[string]interface{}{
			"url":   fileURL,
			"size":  fileSize,
			"token": token,
		}

		if !expirationDate.IsZero() {
			response["expires_at"] = expirationDate
			days := int(expirationDate.Sub(time.Now()).Hours() / 24)
			response["expires_in_days"] = days
		}

		return c.JSON(http.StatusOK, response)
	}

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	return c.String(http.StatusOK, fileURL+"\n")
}

func generateID(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes)[:length], nil
}
