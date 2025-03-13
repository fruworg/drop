package app

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/expiration"
	"github.com/marianozunino/drop/internal/handler"
	custommiddleware "github.com/marianozunino/drop/internal/middleware"
)

//go:embed favicon.ico
var faviconFS embed.FS

// App represents the application
type App struct {
	server            *echo.Echo
	expirationManager *expiration.ExpirationManager
	config            *config.Config
}

// New creates a new application instance
func New() (*App, error) {
	// Setup config and directories
	err := setup()
	if err != nil {
		return nil, err
	}

	// Initialize expiration manager
	expirationManager, err := expiration.NewExpirationManager(config.DefaultConfigPath)
	if err != nil {
		log.Printf("Warning: Failed to initialize expiration manager: %v", err)
	}

	// Initialize Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Setup app
	app := &App{
		server:            e,
		expirationManager: expirationManager,
		config:            &expirationManager.Config,
	}

	// Configure middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(custommiddleware.SecurityHeaders())

	// Register routes
	registerRoutes(e, app)

	return app, nil
}

// Start starts the application and all services
func (a *App) Start() {
	// Start expiration manager if available
	if a.expirationManager != nil {
		a.expirationManager.Start()
	}

	// Start the HTTP server in a separate goroutine
	go func() {
		if err := a.server.Start(":8080"); err != nil {
			log.Printf("Server stopped: %v", err)
		}
	}()

	log.Println("Server started on :8080")
}

// Stop stops all application services
func (a *App) Stop() {
	if a.expirationManager != nil {
		a.expirationManager.Stop()
	}
}

// Shutdown gracefully shuts down the server
func (a *App) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

// setup ensures all necessary directories and files exist
func setup() error {
	// Create uploads directory
	if err := os.MkdirAll(config.DefaultUploadPath, 0o755); err != nil {
		return err
	}

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(config.DefaultConfigPath), 0o755); err != nil {
		return err
	}

	// Create default config file if it doesn't exist
	if err := config.SetupDefaultConfig(); err != nil {
		log.Printf("Warning: Failed to create default config file: %v", err)
	}

	return nil
}

// registerRoutes registers all HTTP routes
func registerRoutes(e *echo.Echo, app *App) {
	h := handler.NewHandler(app.expirationManager)
	var favicon []byte = nil

	// Define routes
	e.GET("/", h.HandleHome)
	e.POST("/", h.HandleUpload)
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
