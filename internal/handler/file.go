package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/config"
)

// HandleFileAccess serves uploaded files with proper content type detection
func (h *Handler) HandleFileAccess(c echo.Context) error {
	filename := c.Param("filename")

	// Handle custom filename part (e.g., "aaa.jpg/image.jpeg")
	parts := strings.SplitN(filename, "/", 2)
	filename = parts[0]

	// Validate filename to prevent path traversal
	if strings.Contains(filename, "..") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	// Set the complete file path
	filePath := filepath.Join(config.DefaultUploadPath, filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "File not found")
	}

	// Default content type
	contentType := "application/octet-stream"

	// Check if metadata exists to get content type
	metadataPath := filePath + ".meta"
	if _, err := os.Stat(metadataPath); err == nil {
		metadataBytes, err := os.ReadFile(metadataPath)
		if err == nil {
			var metadata FileMetadata
			if err := json.Unmarshal(metadataBytes, &metadata); err == nil && metadata.ContentType != "" {
				// Set content type if available in metadata
				contentType = metadata.ContentType

				// Set expiration header if available
				if !metadata.ExpiresAt.IsZero() {
					expiresMs := metadata.ExpiresAt.UnixNano() / int64(time.Millisecond)
					c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
				}
			}
		}
	}

	// If no specific content type in metadata, detect it
	if contentType == "application/octet-stream" {
		// Method 1: Use mime.TypeByExtension
		ext := filepath.Ext(filename)
		if mimeType := mime.TypeByExtension(ext); mimeType != "" {
			contentType = mimeType
		} else {
			// Method 2: Detect from file content using http.DetectContentType
			file, err := os.Open(filePath)
			if err == nil {
				defer file.Close()

				// Create a buffer to store the header of the file
				buffer := make([]byte, 512)

				// Read the first 512 bytes
				_, err := file.Read(buffer)
				if err == nil {
					// Use the net/http package's DetectContentType function
					contentType = http.DetectContentType(buffer)
				}
			}
		}
	}

	// Set the content type header
	c.Response().Header().Set("Content-Type", contentType)

	// Add Content-Disposition header to encourage inline display
	if strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "audio/") ||
		strings.HasPrefix(contentType, "image/") ||
		contentType == "application/pdf" ||
		strings.HasPrefix(contentType, "text/") {
		c.Response().Header().Set("Content-Disposition", "inline")
	}

	// Serve the file
	return c.File(filePath)
}

// HandleFileManagement manages file operations (delete, update expiration)
func (h *Handler) HandleFileManagement(c echo.Context) error {
	// Parse form data for both multipart and standard forms
	if err := c.Request().ParseMultipartForm(32 << 20); err != nil {
		// If multipart parsing fails, try regular form parsing
		if err := c.Request().ParseForm(); err != nil {
			// Log but continue - the request might use a different content type
			log.Printf("Info: Non-form request or parsing error: %v", err)
		}
	}

	// Get and validate the filename
	filename := c.Param("filename")
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	// Set the complete file paths
	filePath := filepath.Join(config.DefaultUploadPath, filename)
	metadataPath := filePath + ".meta"

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "File not found")
	}

	// Get and validate management token
	token := c.FormValue("token")
	if token == "" {
		return c.String(http.StatusBadRequest, "Missing management token")
	}

	// Authenticate the request using the token
	authenticated := false
	var metadata FileMetadata

	// Try to read and validate metadata
	metadataBytes, err := os.ReadFile(metadataPath)
	if err == nil {
		if err := json.Unmarshal(metadataBytes, &metadata); err == nil {
			if metadata.Token == token {
				authenticated = true
			} else {
				return c.String(http.StatusUnauthorized, "Invalid management token")
			}
		} else {
			log.Printf("Warning: Failed to parse metadata for %s: %v", filename, err)
		}
	} else {
		log.Printf("Warning: Metadata file not found for %s: %v", filename, err)
		// For backward compatibility, allow legacy files without metadata
		authenticated = true
	}

	if !authenticated {
		return c.String(http.StatusUnauthorized, "Authentication failed")
	}

	// Handle delete operation
	if _, deleteRequested := c.Request().Form["delete"]; deleteRequested {
		// Delete the file first
		if err := os.Remove(filePath); err != nil {
			log.Printf("Error: Failed to delete file %s: %v", filePath, err)
			return c.String(http.StatusInternalServerError, "Failed to delete file")
		}

		// Then try to delete metadata (ignore errors)
		if err := os.Remove(metadataPath); err != nil {
			log.Printf("Warning: Failed to delete metadata %s: %v", metadataPath, err)
		}

		return c.String(http.StatusOK, "File deleted successfully")
	}

	// Handle expiration update
	if expiresStr := c.FormValue("expires"); expiresStr != "" {
		expirationDate, err := parseExpirationTime(expiresStr)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
		}

		// If we have metadata, update it
		if metadataBytes != nil {
			metadata.ExpiresAt = expirationDate

			// Marshal and save the updated metadata
			updatedMetadata, err := json.Marshal(metadata)
			if err != nil {
				log.Printf("Error: Failed to marshal metadata: %v", err)
				return c.String(http.StatusInternalServerError, "Failed to update expiration")
			}

			if err := os.WriteFile(metadataPath, updatedMetadata, 0o644); err != nil {
				log.Printf("Error: Failed to save metadata: %v", err)
				return c.String(http.StatusInternalServerError, "Failed to save expiration")
			}

			return c.String(http.StatusOK, "Expiration updated successfully")
		}

		// For files without metadata, create new metadata
		newMetadata := FileMetadata{
			Token:      token,
			UploadDate: time.Now(), // Since we don't know the original upload date
			ExpiresAt:  expirationDate,
		}

		// Try to get file information for size
		if fileInfo, err := os.Stat(filePath); err == nil {
			newMetadata.Size = fileInfo.Size()
		}

		// Marshal and save the new metadata
		newMetadataBytes, err := json.Marshal(newMetadata)
		if err != nil {
			log.Printf("Error: Failed to marshal new metadata: %v", err)
			return c.String(http.StatusInternalServerError, "Failed to create expiration metadata")
		}

		if err := os.WriteFile(metadataPath, newMetadataBytes, 0o644); err != nil {
			log.Printf("Error: Failed to save new metadata: %v", err)
			return c.String(http.StatusInternalServerError, "Failed to save expiration metadata")
		}

		return c.String(http.StatusOK, "Expiration created successfully")
	}

	// If we get here, no valid operation was specified
	return c.String(http.StatusBadRequest, "No valid operation specified. Use 'delete' or 'expires'.")
}

// HandleUpload processes file uploads
func (h *Handler) HandleUpload(c echo.Context) error {
	// Limit request body size
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, config.MaxUploadSize)

	var fileContent []byte
	var fileName string
	var fileSize int64
	var contentType string

	// Check if a file was uploaded
	file, header, err := c.Request().FormFile("file")
	if err == nil {
		defer file.Close()

		// Read file content
		content, err := io.ReadAll(file)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to read file")
		}

		fileContent = content
		fileName = header.Filename
		fileSize = header.Size
		contentType = header.Header.Get("Content-Type")
	} else {
		// Check if URL was provided
		url := c.FormValue("url")
		if url == "" {
			return c.String(http.StatusBadRequest, "No file or URL provided")
		}

		// Download from URL
		resp, err := http.Get(url)
		if err != nil {
			return c.String(http.StatusBadRequest, "Failed to download from URL")
		}
		defer resp.Body.Close()

		// Get content length
		contentLength := resp.Header.Get("Content-Length")
		if contentLength == "" {
			return c.String(http.StatusBadRequest, "Remote server did not provide Content-Length")
		}

		length, err := strconv.ParseInt(contentLength, 10, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Invalid Content-Length")
		}

		if length > config.MaxUploadSize {
			return c.String(http.StatusBadRequest, fmt.Sprintf("File too large (max %d bytes)", config.MaxUploadSize))
		}

		// Read content
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to read from URL")
		}

		// Extract filename from URL
		urlPath := strings.Split(url, "/")
		if len(urlPath) > 0 {
			fileName = urlPath[len(urlPath)-1]
		} else {
			fileName = "download"
		}

		fileContent = content
		fileSize = int64(len(content))
		contentType = resp.Header.Get("Content-Type")
	}

	// Check if secret parameter was provided
	useSecretId := c.FormValue("secret") != ""

	// Generate a unique ID for the file
	var id string
	if useSecretId {
		// Use a longer ID for "secret" URLs
		id, err = generateID(8)
	} else {
		id, err = generateID(config.DefaultIDLength)
	}

	if err != nil {
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Determine file extension
	fileExt := filepath.Ext(fileName)

	// Create filename with original extension
	filename := id
	if fileExt != "" {
		filename += fileExt
	}

	// Create file path
	filePath := filepath.Join(config.DefaultUploadPath, filename)

	// Save file
	if err := os.WriteFile(filePath, fileContent, 0o644); err != nil {
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Calculate expiration date
	var expirationDate time.Time

	// Check if expiration was specified
	expiresStr := c.FormValue("expires")
	if expiresStr != "" {
		expirationDate, err = parseExpirationTime(expiresStr)
		if err != nil {
			return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid expiration format: %v", err))
		}
	} else if h.expManager != nil {
		// Use the default retention policy
		expirationDate = h.expManager.GetExpirationDate(fileSize)
	}

	// Generate management token
	managementToken, err := generateID(16)
	if err != nil {
		log.Printf("Warning: Failed to generate management token: %v", err)
	}

	// Store metadata alongside the file
	metadata := FileMetadata{
		Token:        managementToken,
		OriginalName: fileName,
		UploadDate:   time.Now(),
		Size:         fileSize,
		ContentType:  contentType,
	}

	if !expirationDate.IsZero() {
		metadata.ExpiresAt = expirationDate
	}

	// Serialize to JSON
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		log.Printf("Warning: Failed to serialize file metadata: %v", err)
	} else {
		// Create metadata file with the same name but .meta extension
		metadataPath := filePath + ".meta"
		if err := os.WriteFile(metadataPath, metadataBytes, 0o644); err != nil {
			log.Printf("Warning: Failed to write metadata file: %v", err)
		}
	}

	// Set X-Token header for file management
	c.Response().Header().Set("X-Token", managementToken)

	// Return the URL to the uploaded file
	fileURL := h.expManager.Config.BaseURL + filename

	// If expires date exists, set header
	if !expirationDate.IsZero() {
		expiresMs := expirationDate.UnixNano() / int64(time.Millisecond)
		c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
	}

	// Check content type to determine response format
	if strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		// Return JSON response
		response := map[string]interface{}{
			"url":   fileURL,
			"size":  fileSize,
			"token": managementToken,
		}

		if !expirationDate.IsZero() {
			response["expires_at"] = expirationDate
			days := int(expirationDate.Sub(time.Now()).Hours() / 24)
			response["expires_in_days"] = days
		}

		return c.JSON(http.StatusOK, response)
	} else {
		// Return plain text URL (for CLI usage)
		c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
		return c.String(http.StatusOK, fileURL+"\n")
	}
}

// parseExpirationTime parses different expiration time formats
func parseExpirationTime(expiresStr string) (time.Time, error) {
	// Try to parse as hours first
	if hours, err := strconv.Atoi(expiresStr); err == nil {
		return time.Now().Add(time.Duration(hours) * time.Hour), nil
	}

	// Try to parse as milliseconds since epoch
	if ms, err := strconv.ParseInt(expiresStr, 10, 64); err == nil {
		return time.Unix(0, ms*int64(time.Millisecond)), nil
	}

	// Try standard date formats
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, expiresStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date/time format")
}

// generateID creates a random hex string of given length
func generateID(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}
