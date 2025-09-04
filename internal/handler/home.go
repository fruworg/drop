package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/templates"
)

// HandleHome serves the homepage
func (h *Handler) HandleHome(c echo.Context) error {
	if h.expManager == nil {
		return c.String(http.StatusInternalServerError, "Server configuration not available")
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	err := templates.HomePage(*h.cfg).Render(context.Background(), c.Response())
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error rendering template: %v", err))
	}

	return nil
}

// HandleChunkedUpload serves the chunked upload page
func (h *Handler) HandleChunkedUpload(c echo.Context) error {
	if h.expManager == nil {
		return c.String(http.StatusInternalServerError, "Server configuration not available")
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	err := templates.ChunkedUploadPage(*h.cfg).Render(context.Background(), c.Response())
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error rendering template: %v", err))
	}

	return nil
}
