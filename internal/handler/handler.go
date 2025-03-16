package handler

import (
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
)

// Handler handles HTTP requests
type Handler struct {
	expManager *expiration.ExpirationManager
	db         *db.DB
	cfg        *config.Config
}

// NewHandler creates a new handler
func NewHandler(expManager *expiration.ExpirationManager, cfg *config.Config, db *db.DB) *Handler {
	return &Handler{
		expManager: expManager,
		db:         db,
		cfg:        cfg,
	}
}
