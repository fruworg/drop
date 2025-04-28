package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
	"github.com/marianozunino/drop/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (string, *Handler, *db.DB, func()) {
	tempDir, err := os.MkdirTemp("", "drop-test")
	require.NoError(t, err)

	testDBPath := filepath.Join(tempDir, "test.db")

	cfg := &config.Config{
		UploadPath:               tempDir,
		MinAge:                   1,
		MaxAge:                   30,
		MaxSize:                  250.0,
		CheckInterval:            60,
		ExpirationManagerEnabled: true,
		BaseURL:                  "http://localhost:8080/",
		SQLitePath:               testDBPath,
		IdLength:                 4,
		PreviewBots: []string{
			"slack",
			"slackbot",
			"facebookexternalhit",
			"twitterbot",
		},
	}

	testDB, err := db.NewDB(cfg)
	require.NoError(t, err)

	expManager, err := expiration.NewExpirationManager(cfg, testDB)
	require.NoError(t, err)

	h := NewHandler(expManager, cfg, testDB)

	cleanup := func() {
		testDB.Close()
		os.RemoveAll(tempDir)
	}

	return tempDir, h, testDB, cleanup
}

func createTestFile(t *testing.T, tempDir string, db *db.DB, filename, content string, oneTimeView bool) string {
	filePath := filepath.Join(tempDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0o644)
	require.NoError(t, err)

	meta := model.FileMetadata{
		FilePath:     filePath,
		Token:        "test-token",
		OriginalName: "original-" + filename,
		Size:         int64(len(content)),
		ContentType:  "text/plain",
		OneTimeView:  oneTimeView,
	}

	err = db.StoreMetadata(&meta)
	require.NoError(t, err)

	return filePath
}

func TestStandardFileAccess(t *testing.T) {
	tempDir, h, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testFilename := "standard.txt"
	testContent := "This is a standard file"
	filePath := createTestFile(t, tempDir, db, testFilename, testContent, false)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues(testFilename)

	err := h.HandleFileAccess(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(body))

	_, err = os.Stat(filePath)
	assert.NoError(t, err, "The file should still exist")
}

func TestOneTimeFileAccess(t *testing.T) {
	tempDir, h, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testFilename := "onetime.txt"
	testContent := "This is a one-time file"
	filePath := createTestFile(t, tempDir, db, testFilename, testContent, true)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues(testFilename)

	err := h.HandleFileAccess(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(body))
	assert.Equal(t, "true", rec.Header().Get("X-One-Time-View"))

	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "The file should have been deleted")

	req = httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues(testFilename)

	err = h.HandleFileAccess(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPreviewBotAccess(t *testing.T) {
	tempDir, h, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testFilename := "onetime-bot.txt"
	testContent := "This is a one-time file accessed by a bot"
	filePath := createTestFile(t, tempDir, db, testFilename, testContent, true)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
	req.Header.Set("User-Agent", "Slackbot-LinkExpanding 1.0")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues(testFilename)

	err := h.HandleFileAccess(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(string(body), "One-Time Download Link"), "Response should contain placeholder text")
	assert.True(t, strings.Contains(rec.Header().Get("Content-Type"), "text/html"), "Content type should be HTML for the placeholder")

	_, err = os.Stat(filePath)
	assert.NoError(t, err, "The file should still exist after bot access")

	req = httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues(testFilename)

	err = h.HandleFileAccess(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	body, err = io.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.Equal(t, testContent, string(body))

	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "The file should have been deleted after real user access")
}

func TestNonExistentFile(t *testing.T) {
	_, h, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent.txt", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues("nonexistent.txt")

	err := h.HandleFileAccess(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDifferentContentTypes(t *testing.T) {
	tempDir, h, db, cleanup := setupTestEnvironment(t)
	defer cleanup()

	testCases := []struct {
		filename       string
		content        string
		contentType    string
		shouldBeInline bool
	}{
		{
			filename:       "document.txt",
			content:        "This is a text document",
			contentType:    "text/plain",
			shouldBeInline: true,
		},
		{
			filename:       "image.jpg",
			content:        "fake image content",
			contentType:    "image/jpeg",
			shouldBeInline: true,
		},
		{
			filename:       "archive.zip",
			content:        "fake zip content",
			contentType:    "application/zip",
			shouldBeInline: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tc.filename)
			err := os.WriteFile(filePath, []byte(tc.content), 0o644)
			require.NoError(t, err)

			meta := model.FileMetadata{
				FilePath:     filePath,
				Token:        "test-token",
				OriginalName: "original-" + tc.filename,
				Size:         int64(len(tc.content)),
				ContentType:  tc.contentType,
				OneTimeView:  false,
			}

			err = db.StoreMetadata(&meta)
			require.NoError(t, err)

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/"+tc.filename, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("filename")
			c.SetParamValues(tc.filename)

			err = h.HandleFileAccess(c)
			assert.NoError(t, err)

			assert.Equal(t, tc.contentType, rec.Header().Get("Content-Type"))

			contentDisposition := rec.Header().Get("Content-Disposition")
			if tc.shouldBeInline {
				assert.Contains(t, contentDisposition, "inline")
			} else {
				assert.NotContains(t, contentDisposition, "inline")
			}
		})
	}
}

func TestCustomUserAgentDetection(t *testing.T) {
	testCases := []struct {
		name      string
		userAgent string
		isBot     bool
	}{
		{
			name:      "Slack Bot",
			userAgent: "Slackbot-LinkExpanding 1.0",
			isBot:     true,
		},
		{
			name:      "Twitter Bot",
			userAgent: "Twitterbot/1.0",
			isBot:     true,
		},
		{
			name:      "Chrome Browser",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			isBot:     false,
		},
		{
			name:      "Firefox Browser",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:89.0) Gecko/20100101 Firefox/89.0",
			isBot:     false,
		},
		{
			name:      "Bot not in config",
			userAgent: "LinkedInBot/1.0",
			isBot:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir, h, db, cleanup := setupTestEnvironment(t)
			defer cleanup()

			testFilename := "ua-test.txt"
			testContent := "This is a test file for user agent detection"
			createTestFile(t, tempDir, db, testFilename, testContent, true)

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
			req.Header.Set("User-Agent", tc.userAgent)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("filename")
			c.SetParamValues(testFilename)

			err := h.HandleFileAccess(c)
			assert.NoError(t, err)

			if tc.isBot {
				assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
				body, err := io.ReadAll(rec.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(body), "One-Time Download Link")
			} else {
				assert.Equal(t, "text/plain", rec.Header().Get("Content-Type"))
				body, err := io.ReadAll(rec.Body)
				assert.NoError(t, err)
				assert.Equal(t, testContent, string(body))
			}
		})
	}
}
