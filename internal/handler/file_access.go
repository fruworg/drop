package handler

import (
	"context"
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
	"github.com/marianozunino/drop/templates"
)

func (h *Handler) HandleFileAccess(c echo.Context) error {
	filename := c.Param("filename")

	meta, err := h.db.GetMetadataByID(filename)
	if err == nil && meta.IsURLShortener {
		return h.HandleURLRedirect(c)
	}
	filePath, err := h.validateAndResolvePath(c)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			log.Printf("Warning: File access error: %v", err)
			return c.String(http.StatusNotFound, "File not found")
		}
		log.Printf("Error: File access error: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	meta, err = h.getFileMetadata(filePath)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to get metadata")
	}

	isPreviewBot := h.isLinkPreviewBot(c.Request())
	if meta.OneTimeView && isPreviewBot {
		return h.servePlaceholderForPreviewBot(c)
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error: Failed to open file for download: %v", err)
		return c.String(http.StatusInternalServerError, "Failed to open file")
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to stat file")
	}

	h.setResponseHeaders(c, meta, fileInfo)

	if h.handleConditionalRequest(c, meta, fileInfo) {
		return nil
	}

	if rangeHeader := c.Request().Header.Get("Range"); rangeHeader != "" {
		return h.handleRangeRequest(c, file, fileInfo, meta)
	}

	contentDisposition := c.Response().Header().Get("Content-Disposition")
	if contentDisposition == "" {
		if shouldDisplayInline(meta.ContentType) {
			c.Response().Header().Set("Content-Disposition", "inline; filename=\""+meta.OriginalName+"\"")
		} else {
			c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+meta.OriginalName+"\"")
		}
	}

	log.Printf("File served: %s (%s) to %s", meta.OriginalName, formatBytes(fileInfo.Size()), c.RealIP())
	c.Response().WriteHeader(http.StatusOK)
	_, err = h.streamFileOptimized(c.Response(), file)

	if err == nil && meta.OneTimeView {
		err = h.deleteOneTimeViewFile(filePath, meta)
	}

	return err
}

// handleRangeRequest handles HTTP Range requests for better streaming
func (h *Handler) handleRangeRequest(c echo.Context, file *os.File, fileInfo os.FileInfo, meta model.FileMetadata) error {
	rangeHeader := c.Request().Header.Get("Range")

	// Parse range header (e.g., "bytes=0-1023")
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return c.String(http.StatusBadRequest, "Invalid range header")
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	ranges := strings.Split(rangeStr, ",")

	// For now, handle single range only
	if len(ranges) > 1 {
		return c.String(http.StatusRequestedRangeNotSatisfiable, "Multiple ranges not supported")
	}

	rangeStr = ranges[0]
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return c.String(http.StatusBadRequest, "Invalid range format")
	}

	startStr := parts[0]
	endStr := parts[1]

	var start, end int64
	var err error

	if startStr == "" {
		// Range like "bytes=-1023" (last 1023 bytes)
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Invalid range end")
		}
		start = fileInfo.Size() - end
		end = fileInfo.Size() - 1
	} else if endStr == "" {
		// Range like "bytes=1024-" (from byte 1024 to end)
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Invalid range start")
		}
		end = fileInfo.Size() - 1
	} else {
		// Range like "bytes=1024-2047"
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Invalid range start")
		}
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Invalid range end")
		}
	}

	// Validate range
	if start < 0 || end >= fileInfo.Size() || start > end {
		return c.String(http.StatusRequestedRangeNotSatisfiable, "Range not satisfiable")
	}

	// Seek to start position
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to seek file")
	}

	// Set response headers for partial content
	contentLength := end - start + 1
	c.Response().Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileInfo.Size()))
	c.Response().Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
	c.Response().Header().Set("Accept-Ranges", "bytes")

	// Set Content-Disposition header
	if shouldDisplayInline(meta.ContentType) {
		c.Response().Header().Set("Content-Disposition", "inline; filename=\""+meta.OriginalName+"\"")
	} else {
		c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+meta.OriginalName+"\"")
	}

	log.Printf("Range request served: %s (%d-%d/%d) to %s", meta.OriginalName, start, end, fileInfo.Size(), c.RealIP())
	c.Response().WriteHeader(http.StatusPartialContent)

	// Copy only the requested range
	_, err = io.CopyN(c.Response(), file, contentLength)
	return err
}

// handleConditionalRequest handles If-None-Match and If-Modified-Since headers
func (h *Handler) handleConditionalRequest(c echo.Context, meta model.FileMetadata, fileInfo os.FileInfo) bool {
	// Handle If-None-Match (ETag)
	if ifNoneMatch := c.Request().Header.Get("If-None-Match"); ifNoneMatch != "" {
		etag := fmt.Sprintf("\"%d-%d\"", fileInfo.Size(), fileInfo.ModTime().Unix())
		if ifNoneMatch == etag {
			c.Response().WriteHeader(http.StatusNotModified)
			return true
		}
	}

	// Handle If-Modified-Since
	if ifModifiedSince := c.Request().Header.Get("If-Modified-Since"); ifModifiedSince != "" {
		if t, err := time.Parse(http.TimeFormat, ifModifiedSince); err == nil {
			if !fileInfo.ModTime().After(t) {
				c.Response().WriteHeader(http.StatusNotModified)
				return true
			}
		}
	}

	return false
}

// streamFileOptimized streams a file with optimized buffering
func (h *Handler) streamFileOptimized(w http.ResponseWriter, file *os.File) (int64, error) {
	bufferSize := h.cfg.StreamingBufferSizeToBytes()
	if bufferSize <= 0 {
		bufferSize = 64 * 1024 // Default 64KB
	}

	buffer := make([]byte, bufferSize)
	var totalWritten int64

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			written, writeErr := w.Write(buffer[:n])
			totalWritten += int64(written)
			if writeErr != nil {
				return totalWritten, writeErr
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return totalWritten, err
		}
	}

	return totalWritten, nil
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
func (h *Handler) setResponseHeaders(c echo.Context, meta model.FileMetadata, fileInfo os.FileInfo) {
	contentType := "application/octet-stream"
	if meta.ContentType != "" {
		contentType = meta.ContentType
	}

	// Set expiration header if applicable
	if meta.ExpiresAt != nil && !meta.ExpiresAt.IsZero() {
		expiresMs := meta.ExpiresAt.UnixNano() / int64(time.Millisecond)
		c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
	}

	c.Response().Header().Set("Content-Type", contentType)

	// Enable range requests for better streaming
	c.Response().Header().Set("Accept-Ranges", "bytes")

	// Add caching headers for better performance
	// For one-time files, no caching
	if meta.OneTimeView {
		c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Response().Header().Set("Pragma", "no-cache")
		c.Response().Header().Set("Expires", "0")
	} else {
		// For regular files, allow caching but with revalidation
		c.Response().Header().Set("Cache-Control", "public, max-age=3600, must-revalidate")
		c.Response().Header().Set("ETag", fmt.Sprintf("\"%d-%d\"", fileInfo.Size(), fileInfo.ModTime().Unix()))
	}

	if meta.OneTimeView {
		c.Response().Header().Set("X-One-Time-View", "true")
	}

	log.Printf("Content-Type: %s", contentType)

	// Set content disposition based on content type
	if shouldDisplayInline(contentType) {
		c.Response().Header().Set("Content-Disposition", "inline")
	}

	// Add compression for text-based content types
	if shouldCompress(contentType) {
		c.Response().Header().Set("Content-Encoding", "gzip")
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

// shouldCompress determines if the content type should be compressed
func shouldCompress(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/xml" ||
		contentType == "application/javascript" ||
		contentType == "application/css"
}
