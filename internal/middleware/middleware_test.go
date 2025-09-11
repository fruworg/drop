package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders(t *testing.T) {
	e := echo.New()

	testHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "test response")
	}

	e.Use(SecurityHeaders())
	e.GET("/test", testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	headers := rec.Header()

	assert.Equal(t, "*", headers.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", headers.Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", headers.Get("Access-Control-Allow-Headers"))

	assert.Equal(t, "max-age=63072000; includeSubDomains; preload", headers.Get("Strict-Transport-Security"))
	assert.Equal(t, "sameorigin", headers.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
	assert.Equal(t, "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'", headers.Get("Content-Security-Policy"))
	assert.Equal(t, "no-referrer, strict-origin-when-cross-origin", headers.Get("Referrer-Policy"))

	assert.Empty(t, headers.Get("Server"))
}

func TestSecurityHeadersWithDifferentMethods(t *testing.T) {
	e := echo.New()

	testHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "test response")
	}

	e.Use(SecurityHeaders())
	e.POST("/test", testHandler)
	e.PUT("/test", testHandler)
	e.DELETE("/test", testHandler)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			headers := rec.Header()
			assert.Equal(t, "*", headers.Get("Access-Control-Allow-Origin"))
			assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", headers.Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "sameorigin", headers.Get("X-Frame-Options"))
			assert.Empty(t, headers.Get("Server"))
		})
	}
}

func TestSecurityHeadersWithErrorResponse(t *testing.T) {
	e := echo.New()

	errorHandler := func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
	}

	e.Use(SecurityHeaders())
	e.GET("/error", errorHandler)

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	headers := rec.Header()
	assert.Equal(t, "*", headers.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "sameorigin", headers.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Empty(t, headers.Get("Server"))
}

func TestSecurityHeadersMiddlewareChain(t *testing.T) {
	e := echo.New()

	customMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Custom-Header", "custom-value")
			return next(c)
		}
	}

	testHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "chained response")
	}

	e.Use(SecurityHeaders())
	e.Use(customMiddleware)
	e.GET("/chain", testHandler)

	req := httptest.NewRequest(http.MethodGet, "/chain", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	headers := rec.Header()

	assert.Equal(t, "*", headers.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "sameorigin", headers.Get("X-Frame-Options"))
	assert.Empty(t, headers.Get("Server"))

	assert.Equal(t, "custom-value", headers.Get("X-Custom-Header"))
}

func TestSecurityHeadersWithOPTIONSRequest(t *testing.T) {
	e := echo.New()

	testHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "options response")
	}

	e.Use(SecurityHeaders())
	e.OPTIONS("/test", testHandler)

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	headers := rec.Header()

	assert.Equal(t, "*", headers.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", headers.Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", headers.Get("Access-Control-Allow-Headers"))

	assert.Equal(t, "sameorigin", headers.Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
	assert.Empty(t, headers.Get("Server"))
}
