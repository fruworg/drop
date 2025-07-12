package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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

	// Handle expiration settings
	expirationDate, err := h.determineExpiration(c, fileInfo.Size)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
	}

	// Handle delete operation
	_, oneTimeView := c.Request().Form["one_time"]

	// Store metadata
	managementToken, err := h.storeFileMetadata(fileInfo.FilePath, fileInfo.Name, fileInfo, expirationDate, oneTimeView)
	if err != nil {
		log.Printf("Error: Failed to store metadata: %v", err)
		// Clean up the file if metadata storage fails
		if err := os.Remove(fileInfo.FilePath); err != nil {
			log.Printf("Warning: Failed to clean up file after metadata error: %v", err)
		}
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Return response
	return h.sendUploadResponse(c, fileInfo.Filename, fileInfo.Size, managementToken, expirationDate)
}

// FileInfo holds information about the uploaded file
type FileInfo struct {
	FilePath    string // Path where file was saved
	Filename    string // Final filename (with extension)
	Name        string // Original filename
	Size        int64
	ContentType string
}


func (h *Handler) extractFileContent(c echo.Context) (FileInfo, error) {
	file, header, err := c.Request().FormFile("file")
	if err == nil {
		defer file.Close()
		return h.saveFromFormFile(file, header)
	}

	// Try URL-based upload if form upload failed
	return h.downloadFromURL(c)
}


type ProgressReader struct {
	reader    io.Reader
	total     int64
	current   int64
	filename  string
	startTime time.Time
}

func NewProgressReader(reader io.Reader, total int64, filename string) *ProgressReader {
	return &ProgressReader{
		reader:    reader,
		total:     total,
		filename:  filename,
		startTime: time.Now(),
	}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	// Log progress every 10% or every 10MB, whichever comes first
	if pr.total > 0 {
		percentage := float64(pr.current) / float64(pr.total) * 100
		if pr.current%(10*1024*1024) == 0 || // Every 10MB
			int(percentage)%10 == 0 && pr.current > 0 { // Every 10%
			elapsed := time.Since(pr.startTime)
			speed := float64(pr.current) / elapsed.Seconds() / 1024 / 1024 // MB/s
			log.Printf("Upload progress: %s - %.1f%% (%s/%s) - %.2f MB/s",
				pr.filename, percentage, formatBytes(pr.current), formatBytes(pr.total), speed)
		}
	}

	return n, err
}


func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if size >= 10 {
		return fmt.Sprintf("%.1f %s", size, units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}


func (h *Handler) saveFromFormFile(file io.Reader, header *multipart.FileHeader) (FileInfo, error) {
	// Generate unique ID for the file
	useSecretId := false // You might want to pass this as a parameter
	id, err := h.generateFileID(useSecretId)
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to generate ID: %w", err)
	}

	// Create filename and file path
	fileExt := filepath.Ext(header.Filename)
	filename := id
	if fileExt != "" {
		filename += fileExt
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)

	// Create the file
	dst, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return FileInfo{}, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Wrap the reader with progress tracking
	progressReader := NewProgressReader(file, header.Size, header.Filename)
	log.Printf("Starting upload: %s (%s)", header.Filename, formatBytes(header.Size))

	// Copy with size limit and progress tracking
	limitedReader := io.LimitReader(progressReader, h.cfg.MaxSizeToBytes())
	size, err := io.Copy(dst, limitedReader)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return FileInfo{}, fmt.Errorf("failed to save file: %w", err)
	}

	// Detect content type by reading first 512 bytes
	contentType := h.detectContentType(filePath)

	fileInfo := FileInfo{
		FilePath:    filePath,
		Filename:    filename,
		Name:        header.Filename,
		Size:        size,
		ContentType: contentType,
	}

	elapsed := time.Since(progressReader.startTime)
	avgSpeed := float64(size) / elapsed.Seconds() / 1024 / 1024 // MB/s
	log.Printf("✓ Upload completed: %s (%s) with ID: %s - %.2f MB/s",
		header.Filename, formatBytes(size), id, avgSpeed)
	return fileInfo, nil
}


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

	// Generate unique ID for the file
	useSecretId := c.FormValue("secret") != ""
	id, err := h.generateFileID(useSecretId)
	if err != nil {
		return fileInfo, fmt.Errorf("failed to generate ID: %w", err)
	}

	// Extract filename from URL path
	originalName := h.extractFilenameFromURL(url)
	fileExt := filepath.Ext(originalName)
	filename := id
	if fileExt != "" {
		filename += fileExt
	}

	filePath := filepath.Join(h.cfg.UploadPath, filename)

	// Create the file
	dst, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fileInfo, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Get content length for progress tracking
	contentLength := max(resp.ContentLength, 0)

	// Wrap the response body with progress tracking
	progressReader := NewProgressReader(resp.Body, contentLength, originalName)
	log.Printf("Starting download: %s (%s)", originalName, formatBytes(contentLength))

	// Stream download with size limit and progress tracking
	limitedReader := io.LimitReader(progressReader, h.cfg.MaxSizeToBytes())
	size, err := io.Copy(dst, limitedReader)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		log.Printf("Error: Failed to save from URL: %v", err)
		return fileInfo, fmt.Errorf("Failed to save from URL")
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = h.detectContentType(filePath)
	}

	fileInfo = FileInfo{
		FilePath:    filePath,
		Filename:    filename,
		Name:        originalName,
		Size:        size,
		ContentType: contentType,
	}

	log.Printf("✓ Download completed: %s (%d bytes) with ID: %s", originalName, size, id)
	return fileInfo, nil
}


func (h *Handler) detectContentType(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "application/octet-stream"
	}

	return http.DetectContentType(buffer[:n])
}


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


func (h *Handler) generateFileID(useSecretId bool) (string, error) {
	if useSecretId {
		return generateID(8)
	}
	return generateID(h.cfg.IdLength)
}


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


func (h *Handler) storeFileMetadata(filePath, fileName string, fileInfo FileInfo, expirationDate time.Time, oneTimeView bool) (string, error) {
	managementToken, err := generateID(16)
	if err != nil {
		log.Printf("Warning: Failed to generate management token: %v", err)
		managementToken = filepath.Base(filePath)
	}

	metadata := model.FileMetadata{
		FilePath:     filePath,
		Token:        managementToken,
		OriginalName: fileName,
		UploadDate:   time.Now(),
		Size:         fileInfo.Size,
		ContentType:  fileInfo.ContentType,
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


func (h *Handler) sendUploadResponse(c echo.Context, filename string, fileSize int64, token string, expirationDate time.Time) error {
	c.Response().Header().Set("X-Token", token)
	fileURL := h.expManager.Config.BaseURL + filename

	if !expirationDate.IsZero() {
		expiresMs := expirationDate.UnixNano() / int64(time.Millisecond)
		c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
	}

	if strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		response := map[string]any{
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
