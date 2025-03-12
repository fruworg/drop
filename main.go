package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/marianozunino/drop/config"
	"github.com/marianozunino/drop/expiration"
	"github.com/marianozunino/drop/templates"
)

const (
	// Configuration
	uploadPath    = "./uploads"
	maxUploadSize = 100 * 1024 * 1024      // 100MB
	idLength      = 4                      // Adjust for longer/shorter URLs
	configPath    = "./config/config.json" // Path to expiration config file
)

var expirationManager *expiration.ExpirationManager

// TokenResponse is used to return a management token
type TokenResponse struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

// FileMetadata stores information about uploaded files
type FileMetadata struct {
	Token        string    `json:"token"`
	OriginalName string    `json:"original_name,omitempty"`
	UploadDate   time.Time `json:"upload_date"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type,omitempty"`
}

// SetupDefaultConfig creates a default configuration file if none exists
func SetupDefaultConfig(configPath string) error {
	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // File exists, no need to create
	}

	// Create default config
	data, err := json.MarshalIndent(config.DefaultConfig, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	// Write config file
	return os.WriteFile(configPath, data, 0o644)
}

func main() {
	// Create necessary directories
	if err := os.MkdirAll(uploadPath, 0o755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Initialize expiration manager
	if err := SetupDefaultConfig(configPath); err != nil {
		log.Printf("Warning: Failed to create default config file: %v", err)
	}

	var err error
	expirationManager, err = expiration.NewExpirationManager(configPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize expiration manager: %v", err)
	} else {
		expirationManager.Start()
		defer expirationManager.Stop()
	}

	// Create a new Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", handleHome)
	e.POST("/", handleUpload)

	e.GET("/form", handleHome) // Redirect for legacy compat
	e.POST("/config", handleConfig)

	// File routes
	e.GET("/:filename", handleFileAccess)
	e.POST("/:filename", handleFileManagement)

	// Handle graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			e.Logger.Fatal(err)
		}

		if expirationManager != nil {
			expirationManager.Stop()
		}
	}()

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}

// handleHome serves the homepage
func handleHome(c echo.Context) error {
	// Render the homepage template
	if expirationManager == nil {
		return c.String(http.StatusInternalServerError, "Server configuration not available")
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	err := templates.HomePage(expirationManager.Config).Render(context.Background(), c.Response())
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error rendering template: %v", err))
	}

	return nil
}

// handleFileAccess serves uploaded files with proper content type detection
func handleFileAccess(c echo.Context) error {
	filename := c.Param("filename")

	// Handle custom filename part (e.g., "aaa.jpg/image.jpeg")
	parts := strings.SplitN(filename, "/", 2)
	filename = parts[0]

	// Validate filename to prevent path traversal
	if strings.Contains(filename, "..") {
		return c.String(http.StatusBadRequest, "Invalid file path")
	}

	// Set the complete file path
	filePath := filepath.Join(uploadPath, filename)

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
			// This reads the first 512 bytes to determine content type
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

func handleFileManagement(c echo.Context) error {
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
	filePath := filepath.Join(uploadPath, filename)
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
		// TODO: Consider making this more secure in production
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

// handleConfig handles viewing and updating the expiration configuration
func handleConfig(c echo.Context) error {
	if expirationManager == nil {
		return c.String(http.StatusInternalServerError, "Expiration manager not available")
	}

	// Only accepting POST for config updates
	if c.Request().Header.Get("Content-Type") != "application/json" {
		return c.String(http.StatusBadRequest, "Content-Type must be application/json")
	}

	// Read body
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.String(http.StatusBadRequest, "Failed to read request body")
	}

	// Write to config file
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to write config file")
	}

	// Reload config
	if err := expirationManager.LoadConfig(); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Invalid configuration: %v", err))
	}

	return c.String(http.StatusOK, "Configuration updated successfully")
}

// handleUpload processes file uploads
func handleUpload(c echo.Context) error {
	// Limit request body size
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxUploadSize)

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

		if length > maxUploadSize {
			return c.String(http.StatusBadRequest, fmt.Sprintf("File too large (max %d bytes)", maxUploadSize))
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
		id, err = generateID(idLength)
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
	filePath := filepath.Join(uploadPath, filename)

	// Save file
	if err := os.WriteFile(filePath, fileContent, 0o644); err != nil {
		return c.String(http.StatusInternalServerError, "Server error")
	}

	// Calculate expiration date
	var expirationDate time.Time

	// Check if expiration was specified
	expiresStr := c.FormValue("expires")
	if expiresStr != "" {
		// Try to parse as hours first
		if hours, err := strconv.Atoi(expiresStr); err == nil {
			expirationDate = time.Now().Add(time.Duration(hours) * time.Hour)
		} else {
			// Try to parse as milliseconds since epoch
			if ms, err := strconv.ParseInt(expiresStr, 10, 64); err == nil {
				expirationDate = time.Unix(0, ms*int64(time.Millisecond))
			}
		}
	} else if expirationManager != nil {
		// Use the default retention policy
		expirationDate = expirationManager.GetExpirationDate(fileSize)
		spew.Dump(expirationDate)
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
	fileURL := expirationManager.Config.BaseURL + filename

	// Check content type to determine response format
	requestContentType := c.Request().Header.Get("Content-Type")
	if strings.Contains(requestContentType, "application/json") {
		response := struct {
			URL           string     `json:"url"`
			Size          int64      `json:"size"`
			Token         string     `json:"token,omitempty"`
			ExpiresAt     *time.Time `json:"expires_at,omitempty"`
			ExpiresInDays *int       `json:"expires_in_days,omitempty"`
		}{
			URL:   fileURL,
			Size:  fileSize,
			Token: managementToken,
		}

		if !expirationDate.IsZero() {
			response.ExpiresAt = &expirationDate
			days := int(expirationDate.Sub(time.Now()).Hours() / 24)
			response.ExpiresInDays = &days
		}

		return c.JSON(http.StatusOK, response)
	} else if c.Request().Header.Get("User-Agent") != "" && !strings.Contains(c.Request().Header.Get("User-Agent"), "curl") {
		// Browser response
		c.Response().Header().Set("Content-Type", "text/html")

		expiresInfo := ""
		if !expirationDate.IsZero() {
			expiresInfo = fmt.Sprintf("<p>Expires: %s (in %d days)</p>",
				expirationDate.Format("2006-01-02"),
				int(expirationDate.Sub(time.Now()).Hours()/24))
		}

		htmlResponse := fmt.Sprintf(`
			<!DOCTYPE html>
			<html>
			<head>
				<title>Upload Successful</title>
			</head>
			<body>
				<h2>Upload Successful</h2>
				<p>Your file is available at: <a href="%s">%s</a></p>
				<p>File size: %s</p>
				%s
				<p>Management token: <tt>%s</tt></p>
				<p>Keep this token to manage your file later!</p>
			</body>
			</html>
		`, fileURL, fileURL, formatFileSize(fileSize), expiresInfo, managementToken)

		return c.HTML(http.StatusOK, htmlResponse)
	} else {
		// CLI/curl response - just return the URL
		plainResponse := fmt.Sprintf("%s\n", fileURL)

		if !expirationDate.IsZero() {
			plainResponse += fmt.Sprintf("Expires: %s (in %d days)\n",
				expirationDate.Format("2006-01-02"),
				int(expirationDate.Sub(time.Now()).Hours()/24))
		}

		return c.String(http.StatusOK, plainResponse)
	}
}

// generateID creates a random hex string of given length
func generateID(length int) (string, error) {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// formatFileSize converts bytes to human-readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
