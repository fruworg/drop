package handler

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/model"
)

func (h *Handler) HandleURLShortening(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, h.cfg.MaxSizeToBytes())

	if err := h.parseRequestForm(c); err != nil {
		log.Printf("[HandleURLShortening] Failed to parse form: %v", err)
		return c.String(http.StatusBadRequest, "Invalid request form.")
	}

	originalURL := c.FormValue("url")
	if originalURL == "" {
		return c.String(http.StatusBadRequest, "No URL provided")
	}

	if !h.isValidURL(originalURL) {
		return c.String(http.StatusBadRequest, "Invalid URL format")
	}

	useSecretId := c.FormValue("secret") != ""
	id, err := h.generateUniqueID(useSecretId)
	if err != nil {
		log.Printf("[HandleURLShortening] Failed to generate ID: %v", err)
		return c.String(http.StatusInternalServerError, "Failed to generate short URL")
	}

	expirationDate, err := h.determineExpiration(c, 0)
	if err != nil {
		log.Printf("[HandleURLShortening] Invalid expiration format: %v", err)
		return c.String(http.StatusBadRequest, "Invalid expiration format.")
	}

	_, oneTimeView := c.Request().Form["one_time"]

	managementToken, err := h.storeURLMetadata(id, originalURL, expirationDate, oneTimeView, c)
	if err != nil {
		log.Printf("[HandleURLShortening] Failed to store metadata: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	if err := h.sendURLShorteningResponse(c, id, managementToken, expirationDate); err != nil {
		log.Printf("[HandleURLShortening] Failed to send response: %v", err)
		return c.String(http.StatusInternalServerError, "Server error")
	}

	return nil
}

func (h *Handler) isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

func (h *Handler) storeURLMetadata(shortPath, originalURL string, expirationDate time.Time, oneTimeView bool, c echo.Context) (string, error) {
	managementToken, err := generateID(16)
	if err != nil {
		log.Printf("Warning: Failed to generate management token: %v", err)
		managementToken = shortPath
	}

	var ipAddress string
	if h.cfg.IPTrackingEnabled {
		ipAddress = c.RealIP()
	}

	metadata := model.FileMetadata{
		ResourcePath:   shortPath,
		Token:          managementToken,
		OriginalName:   "URL Shortener",
		UploadDate:     time.Now(),
		Size:           0,
		ContentType:    "text/html",
		OneTimeView:    oneTimeView,
		OriginalURL:    originalURL,
		IsURLShortener: true,
		AccessCount:    0,
		IPAddress:      ipAddress,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if !expirationDate.IsZero() {
		metadata.ExpiresAt = &expirationDate
	}

	if err := h.db.StoreMetadata(&metadata); err != nil {
		return "", err
	}

	return managementToken, nil
}

func (h *Handler) sendURLShorteningResponse(c echo.Context, shortID, token string, expirationDate time.Time) error {
	c.Response().Header().Set("X-Token", token)
	shortURL := h.expManager.Config.BaseURL + shortID

	if !expirationDate.IsZero() {
		expiresMs := expirationDate.UnixNano() / int64(time.Millisecond)
		c.Response().Header().Set("X-Expires", fmt.Sprintf("%d", expiresMs))
	}

	if strings.Contains(c.Request().Header.Get("Accept"), "application/json") {
		response := map[string]any{
			"url":   shortURL,
			"size":  0,
			"token": token,
			"md5":   "",
		}

		if !expirationDate.IsZero() {
			response["expires_at"] = expirationDate.Format(time.RFC3339)
			days := int(time.Until(expirationDate).Hours() / 24)
			response["expires_in_days"] = days
		}

		return c.JSON(http.StatusOK, response)
	}

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	return c.String(http.StatusOK, shortURL+"\n")
}

func (h *Handler) HandleURLRedirect(c echo.Context) error {
	filename := c.Param("filename")

	metadata, err := h.db.GetMetadataByID(filename)
	if err != nil {
		log.Printf("[HandleURLRedirect] Failed to get metadata for %s: %v", filename, err)
		return c.String(http.StatusNotFound, "Short URL not found")
	}

	if !metadata.IsURLShortener {
		return h.HandleFileAccess(c)
	}

	if metadata.ExpiresAt != nil && metadata.ExpiresAt.Before(time.Now()) {
		return c.String(http.StatusGone, "Short URL has expired")
	}

	if metadata.OneTimeView {
		go func() {
			if err := h.db.DeleteMetadata(&metadata); err != nil {
				log.Printf("[HandleURLRedirect] Failed to delete one-time URL %s: %v", filename, err)
			}
		}()
	}

	return c.Redirect(http.StatusFound, metadata.OriginalURL)
}

func (h *Handler) generateUniqueID(useSecretId bool) (string, error) {
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		var length int
		if useSecretId {
			length = 8
		} else {
			length = h.cfg.IdLength
		}

		id, err := generateID(length)
		if err != nil {
			return "", err
		}

		// Check if ID already exists
		_, err = h.db.GetMetadataByID(id)
		if err != nil {
			// ID doesn't exist, we can use it
			return id, nil
		}

		// ID exists, try again
		log.Printf("[generateUniqueID] Collision detected for ID %s, retrying...", id)
	}

	return "", fmt.Errorf("failed to generate unique ID after %d retries", maxRetries)
}
