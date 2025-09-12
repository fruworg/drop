package app

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
	"github.com/marianozunino/drop/internal/handler"
	middie "github.com/marianozunino/drop/internal/middleware"
)

//go:embed favicon.ico
var faviconFS embed.FS

// App represents the application
type App struct {
	server            *echo.Echo
	expirationManager *expiration.ExpirationManager
	config            *config.Config
	db                *db.DB
	actualPort        int
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

func logConfiguration(cfg *config.Config) {
	log.Printf("Drop File Upload Service - Configuration:")
	log.Printf("  Server Port: %d", cfg.Port)
	log.Printf("  Upload Directory: %s", cfg.UploadPath)
	log.Printf("  Database Path: %s", cfg.SQLitePath)
	log.Printf("  Base URL: %s", cfg.BaseURL)
	log.Printf("")
	log.Printf("File Settings:")
	log.Printf("  Max File Size: %s (%.0f MiB)", formatBytes(cfg.MaxSizeToBytes()), cfg.MaxSize)
	log.Printf("  Chunk Size: %s (%.0f MiB)", formatBytes(cfg.ChunkSizeToBytes()), cfg.ChunkSize)
	log.Printf("  ID Length: %d characters", cfg.IdLength)
	log.Printf("")
	log.Printf("Expiration Settings:")
	log.Printf("  Min Retention: %d days", cfg.MinAge)
	log.Printf("  Max Retention: %d days", cfg.MaxAge)
	log.Printf("  Check Interval: %d minutes", cfg.CheckInterval)
	log.Printf("  Expiration Manager: %s", map[bool]string{true: "Enabled", false: "Disabled"}[cfg.ExpirationManagerEnabled])
	log.Printf("")
	log.Printf("Feature Flags:")
	log.Printf("  Admin Panel: %s", map[bool]string{true: "Enabled", false: "Disabled"}[cfg.AdminPanelEnabled])
	log.Printf("  IP Tracking: %s", map[bool]string{true: "Enabled", false: "Disabled"}[cfg.IPTrackingEnabled])
	log.Printf("  URL Shortening: %s", map[bool]string{true: "Enabled", false: "Disabled"}[cfg.URLShorteningEnabled])
	log.Printf("")
	log.Printf("Preview Bots (%d configured):", len(cfg.PreviewBots))
	for i, bot := range cfg.PreviewBots {
		if i < 5 {
			log.Printf("  - %s", bot)
		} else if i == 5 {
			log.Printf("  - ... and %d more", len(cfg.PreviewBots)-5)
			break
		}
	}
}

// New creates a new application instance
func New() (*App, error) {
	configPath := os.Getenv("CONFIG_PATH")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	logConfiguration(cfg)

	err = setup(cfg)
	if err != nil {
		return nil, err
	}

	db, err := db.NewDB(cfg)
	if err != nil {
		log.Printf("Failed to initialize database: %v", err)
		return nil, err
	}

	expirationManager, err := expiration.NewExpirationManager(cfg, db)
	if err != nil {
		log.Printf("Failed to initialize expiration manager: %v", err)
		return nil, err
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Server.ReadTimeout = 10 * time.Minute
	e.Server.WriteTimeout = 10 * time.Minute
	e.Server.IdleTimeout = 15 * time.Minute
	e.Server.ReadHeaderTimeout = 30 * time.Second

	app := &App{
		server:            e,
		expirationManager: expirationManager,
		config:            cfg,
		db:                db,
	}

	e.Use(humanLogger())
	e.Use(middleware.Recover())
	e.Use(middie.SecurityHeaders())

	registerRoutes(e, app)
	return app, nil
}

// NewWithConfig creates a new App instance with a custom configuration
func NewWithConfig(cfg *config.Config) (*App, error) {
	logConfiguration(cfg)

	err := setup(cfg)
	if err != nil {
		return nil, err
	}

	db, err := db.NewDB(cfg)
	if err != nil {
		log.Printf("Failed to initialize database: %v", err)
		return nil, err
	}

	expirationManager, err := expiration.NewExpirationManager(cfg, db)
	if err != nil {
		log.Printf("Failed to initialize expiration manager: %v", err)
		return nil, err
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Server.ReadTimeout = 10 * time.Minute
	e.Server.WriteTimeout = 10 * time.Minute
	e.Server.IdleTimeout = 15 * time.Minute
	e.Server.ReadHeaderTimeout = 30 * time.Second

	app := &App{
		server:            e,
		expirationManager: expirationManager,
		config:            cfg,
		db:                db,
	}

	e.Use(humanLogger())
	e.Use(middleware.Recover())
	e.Use(middie.SecurityHeaders())

	registerRoutes(e, app)
	return app, nil
}

// humanLogger creates a human-friendly logger middleware
func humanLogger() echo.MiddlewareFunc {
	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339} | ${method} ${uri} | ${status} | ${latency_human} | ${bytes_in_human}\n",
		Output: os.Stdout,
	})
}

// Start starts the application
func (a *App) Start() {
	log.Printf("")
	log.Printf("Starting services...")

	if a.expirationManager != nil {
		a.expirationManager.Start()
	}

	if a.config.Port == 0 {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			log.Printf("Failed to find available port: %v", err)
			return
		}
		a.actualPort = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
	} else {
		a.actualPort = a.config.Port
	}

	serverAddr := fmt.Sprintf(":%d", a.actualPort)
	fullURL := fmt.Sprintf("http://localhost%s", serverAddr)

	log.Printf("Starting HTTP server on %s", serverAddr)
	log.Printf("")

	go func() {
		if err := a.server.Start(serverAddr); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	log.Printf("Drop File Upload Service is now running!")
	log.Printf("  Ready to accept uploads at: %s", fullURL)
	log.Printf("")
}

// GetPort returns the actual port the server is running on
func (a *App) GetPort() int {
	if a.actualPort != 0 {
		return a.actualPort
	}
	return a.config.Port
}

// Stop stops the application
func (a *App) Stop() {
	log.Printf("Stopping services...")

	if a.expirationManager != nil {
		a.expirationManager.Stop()
		log.Printf("Expiration manager stopped")
	}

	log.Printf("All services stopped")
}

// Shutdown gracefully shuts down the server
func (a *App) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

// setup ensures all necessary directories and files exist
func setup(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.UploadPath, 0o755); err != nil {
		return err
	}

	return nil
}

// registerRoutes registers all HTTP routes
func registerRoutes(e *echo.Echo, app *App) {
	var favicon []byte

	e.Use(middleware.BodyLimit(
		fmt.Sprintf("%dM", int(app.config.MaxSize)),
	))
	h := handler.NewHandler(app.expirationManager, app.config, app.db)

	e.GET("/", h.HandleHome)
	e.GET("/chunked", h.HandleChunkedUpload)
	e.POST("/", h.HandleUpload)

	e.POST("/upload/init", h.InitiateChunkedUpload)
	e.POST("/upload/chunk/:upload_id/:chunk", h.UploadChunk)
	e.GET("/upload/status/:upload_id", h.GetUploadStatus)

	e.GET("/stats", h.HandleUploadStats)

	if app.config.AdminPanelEnabled {
		e.GET("/admin/login", h.HandleAdminLogin)
		e.POST("/admin/login", h.HandleAdminLogin)
		e.GET("/admin/logout", h.HandleAdminLogout)
		e.GET("/admin", h.HandleAdminDashboard)
		e.GET("/admin/file/:filename", h.HandleAdminFileView)
		e.POST("/admin/file/:filename", h.HandleAdminFileUpdate)
		e.GET("/admin/file/:filename/delete", h.HandleAdminFileDelete)
	}

	e.GET("/binaries/:platform", h.HandleBinaryDownload)
	e.GET("/binaries", h.HandleBinaryList)
	e.GET("/download", h.HandleBinaryAutoDetect)

	e.GET("/favicon.ico", func(c echo.Context) error {
		if favicon == nil {
			data, err := faviconFS.ReadFile("favicon.ico")
			if err != nil {
				return c.String(http.StatusInternalServerError, "Could not read favicon")
			}
			favicon = data
		}
		return c.Blob(http.StatusOK, "image/x-icon", favicon)
	})

	e.GET("/:filename", h.HandleFileAccess)
	e.POST("/:filename", h.HandleFileManagement)
}
