package handler_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
	"github.com/marianozunino/drop/internal/handler"
	"github.com/marianozunino/drop/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneTimeView(t *testing.T) {
	// Create a temporary directory for files and DB
	tempDir, err := os.MkdirTemp("", "drop-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Set up test config
	cfg := &config.Config{
		UploadPath:    tempDir,
		MinAge:        1,
		MaxAge:        30,
		MaxSize:       250.0,
		CheckInterval: 60,
		Enabled:       true,
		BaseURL:       "http://localhost:8080/",
		BadgerPath:    filepath.Join(tempDir, "badger"),
		MaxUploadSize: 1024 * 1024, // 1MB
		IdLength:      4,
	}

	// Create a test DB
	testDB, err := db.NewDB(cfg)
	require.NoError(t, err)
	defer testDB.Close()

	// Create expiration manager
	expManager, err := expiration.NewExpirationManager(cfg, testDB)
	require.NoError(t, err)

	// Create handler
	h := handler.NewHandler(expManager, cfg, testDB)

	// Create a test file with a filename that's exactly as the handler would expect
	testFilename := "testfile.txt"
	testFilePath := filepath.Join(tempDir, testFilename)
	testFileContent := "This is a test file for one-time view"
	err = os.WriteFile(testFilePath, []byte(testFileContent), 0o644)
	require.NoError(t, err)

	// Create metadata for the test file with OneTimeView=true
	meta := model.FileMetadata{
		FilePath:     testFilePath,
		Token:        "test-token",
		OriginalName: "original-test.txt",
		Size:         int64(len(testFileContent)),
		ContentType:  "text/plain",
		OneTimeView:  true, // One-time view flag
	}

	// Store metadata
	err = testDB.StoreMetadata(&meta)
	require.NoError(t, err)

	// Set up Echo and request/response
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/"+testFilename, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("filename")
	c.SetParamValues(testFilename)

	// Execute the file access handler
	err = h.HandleFileAccess(c)
	assert.NoError(t, err)

	// Check that the file was served properly
	assert.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	assert.NoError(t, err)
	assert.Equal(t, testFileContent, string(body))
	assert.Equal(t, "true", rec.Header().Get("X-One-Time-View"))

	// Verify the file has been deleted (should now happen synchronously)
	_, err = os.Stat(testFilePath)
	assert.True(t, os.IsNotExist(err), "The file should have been deleted")

	// // Verify the metadata has been deleted
	// _, err = testDB.GetMetadataByID(testFilePath)
	// assert.Error(t, err, "The metadata should have been deleted")
}
