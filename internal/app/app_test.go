package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatBytes(t *testing.T) {
	result := formatBytes(512)
	assert.Equal(t, "512 B", result)

	result = formatBytes(1024)
	assert.Equal(t, "1.00 KB", result)

	result = formatBytes(1536)
	assert.Equal(t, "1.50 KB", result)

	result = formatBytes(1024 * 1024)
	assert.Equal(t, "1.00 MB", result)

	result = formatBytes(2.5 * 1024 * 1024)
	assert.Equal(t, "2.50 MB", result)

	result = formatBytes(1024 * 1024 * 1024)
	assert.Equal(t, "1.00 GB", result)

	result = formatBytes(3972840000)
	assert.Equal(t, "3.70 GB", result)

	result = formatBytes(1024 * 1024 * 1024 * 1024)
	assert.Equal(t, "1.00 TB", result)

	result = formatBytes(0)
	assert.Equal(t, "0 B", result)

	result = formatBytes(1024 * 1024 * 1024 * 1024 * 1024)
	assert.Equal(t, "1024.0 TB", result)
}

func TestSetup(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		UploadPath: filepath.Join(tempDir, "uploads"),
	}

	err := setup(cfg)
	assert.NoError(t, err)

	_, err = os.Stat(cfg.UploadPath)
	assert.NoError(t, err)
}

func TestSetupWithExistingDirectory(t *testing.T) {
	tempDir := t.TempDir()
	uploadPath := filepath.Join(tempDir, "existing-uploads")

	err := os.MkdirAll(uploadPath, 0755)
	require.NoError(t, err)

	cfg := &config.Config{
		UploadPath: uploadPath,
	}

	err = setup(cfg)
	assert.NoError(t, err)

	_, err = os.Stat(uploadPath)
	assert.NoError(t, err)
}

func TestSetupWithInvalidPath(t *testing.T) {
	cfg := &config.Config{
		UploadPath: "/invalid/path/that/does/not/exist",
	}

	err := setup(cfg)
	assert.Error(t, err)
}

func TestNewWithValidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	dbPath := filepath.Join(tempDir, "test.db")

	configContent := `port: 8080
min_age_days: 1
max_age_days: 30
max_size_mib: 250.0
upload_path: "` + filepath.Join(tempDir, "uploads") + `"
check_interval_min: 60
expiration_manager_enabled: true
base_url: "http://localhost:8080/"
sqlite_path: "` + dbPath + `"
id_length: 4
chunk_size_mib: 4.0
streaming_buffer_size_kb: 64
preview_bots:
  - "slack"
  - "slackbot"`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	require.NoError(t, err)
	assert.NotNil(t, app)

	assert.NotNil(t, app.server)
	assert.NotNil(t, app.expirationManager)
	assert.NotNil(t, app.config)
	assert.NotNil(t, app.db)

	app.Stop()
	app.db.Close()
}

func TestNewWithInvalidConfigPath(t *testing.T) {
	os.Setenv("CONFIG_PATH", "/non/existent/config.yaml")
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	assert.Error(t, err)
	assert.Nil(t, app)
}

func TestNewWithInvalidConfigContent(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid.yaml")

	invalidContent := `port: 8080
invalid: yaml: content: [`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	assert.Error(t, err)
	assert.Nil(t, app)
}

func TestAppStartAndStop(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	dbPath := filepath.Join(tempDir, "test.db")

	configContent := `port: 0
min_age_days: 1
max_age_days: 30
max_size_mib: 250.0
upload_path: "` + filepath.Join(tempDir, "uploads") + `"
check_interval_min: 60
expiration_manager_enabled: false
base_url: "http://localhost:8080/"
sqlite_path: "` + dbPath + `"
id_length: 4
chunk_size_mib: 4.0
streaming_buffer_size_kb: 64
preview_bots:
  - "slack"`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	require.NoError(t, err)
	defer app.db.Close()

	app.Start()

	time.Sleep(100 * time.Millisecond)

	app.Stop()

	time.Sleep(100 * time.Millisecond)
}

func TestAppShutdown(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	dbPath := filepath.Join(tempDir, "test.db")

	configContent := `port: 0
min_age_days: 1
max_age_days: 30
max_size_mib: 250.0
upload_path: "` + filepath.Join(tempDir, "uploads") + `"
check_interval_min: 60
expiration_manager_enabled: false
base_url: "http://localhost:8080/"
sqlite_path: "` + dbPath + `"
id_length: 4
chunk_size_mib: 4.0
streaming_buffer_size_kb: 64
preview_bots:
  - "slack"`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	require.NoError(t, err)
	defer app.db.Close()

	app.Start()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = app.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestAppShutdownWithTimeout(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	dbPath := filepath.Join(tempDir, "test.db")

	configContent := `port: 0
min_age_days: 1
max_age_days: 30
max_size_mib: 250.0
upload_path: "` + filepath.Join(tempDir, "uploads") + `"
check_interval_min: 60
expiration_manager_enabled: false
base_url: "http://localhost:8080/"
sqlite_path: "` + dbPath + `"
id_length: 4
chunk_size_mib: 4.0
streaming_buffer_size_kb: 64
preview_bots:
  - "slack"`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	require.NoError(t, err)
	defer app.db.Close()

	app.Start()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	err = app.Shutdown(ctx)
	_ = err
}

func TestHumanLogger(t *testing.T) {
	e := echo.New()

	e.Use(humanLogger())

	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "test response")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test response", rec.Body.String())
}

func TestRegisterRoutes(t *testing.T) {
	e := echo.New()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &config.Config{
		UploadPath: filepath.Join(tempDir, "uploads"),
		SQLitePath: dbPath,
		MaxSize:    250.0,
	}

	db, err := db.NewDB(cfg)
	require.NoError(t, err)
	defer db.Close()

	expManager, err := expiration.NewExpirationManager(cfg, db)
	require.NoError(t, err)

	app := &App{
		server:            e,
		expirationManager: expManager,
		config:            cfg,
		db:                db,
	}

	registerRoutes(e, app)

	routes := []string{
		"/",
		"/chunked",
		"/stats",
		"/binaries",
		"/download",
		"/favicon.ico",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		assert.NotEqual(t, http.StatusNotFound, rec.Code, "Route %s should exist", route)
	}
}

func TestAppWithExpirationManagerDisabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	dbPath := filepath.Join(tempDir, "test.db")

	configContent := `port: 0
min_age_days: 1
max_age_days: 30
max_size_mib: 250.0
upload_path: "` + filepath.Join(tempDir, "uploads") + `"
check_interval_min: 60
expiration_manager_enabled: false
base_url: "http://localhost:8080/"
sqlite_path: "` + dbPath + `"
id_length: 4
chunk_size_mib: 4.0
streaming_buffer_size_kb: 64
preview_bots:
  - "slack"`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("CONFIG_PATH", configPath)
	defer os.Unsetenv("CONFIG_PATH")

	app, err := New()
	require.NoError(t, err)
	defer app.db.Close()

	assert.NotNil(t, app.expirationManager)

	app.Start()
	time.Sleep(100 * time.Millisecond)
	app.Stop()
}
