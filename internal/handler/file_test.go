package handler_test

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/marianozunino/drop/internal/config"
	"github.com/marianozunino/drop/internal/db"
	"github.com/marianozunino/drop/internal/expiration"
	"github.com/marianozunino/drop/internal/handler"
)

func TestHandler_HandleFileAccess(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		expManager *expiration.ExpirationManager
		cfg        *config.Config
		db         *db.DB
		// Named input parameters for target function.
		c       echo.Context
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler.NewHandler(tt.expManager, tt.cfg, tt.db)
			gotErr := h.HandleFileAccess(tt.c)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("HandleFileAccess() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("HandleFileAccess() succeeded unexpectedly")
			}
		})
	}
}
