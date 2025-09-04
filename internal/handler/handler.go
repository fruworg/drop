package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
)

// Handler handles HTTP requests
type Handler struct {
	expManager     *expiration.ExpirationManager
	db             *db.DB
	cfg            *config.Config
	chunkedManager *ChunkedUploadManager
}

// NewHandler creates a new handler
func NewHandler(expManager *expiration.ExpirationManager, cfg *config.Config, db *db.DB) *Handler {
	return &Handler{
		expManager:     expManager,
		db:             db,
		cfg:            cfg,
		chunkedManager: NewChunkedUploadManager(cfg),
	}
}

// HandleUploadStats returns upload statistics
func (h *Handler) HandleUploadStats(c echo.Context) error {
	stats := map[string]interface{}{
		"active_uploads": len(h.chunkedManager.uploads),
	}

	return c.JSON(http.StatusOK, stats)
}
