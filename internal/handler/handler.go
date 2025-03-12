package handler

import (
	"time"

	"github.com/marianozunino/drop/internal/expiration"
)

// Handler handles HTTP requests
type Handler struct {
	expManager *expiration.ExpirationManager
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

// NewHandler creates a new handler
func NewHandler(expManager *expiration.ExpirationManager) *Handler {
	return &Handler{
		expManager: expManager,
	}
}
