package middleware

import (
	"github.com/labstack/echo/v4"
)

// SecurityHeaders adds security-related HTTP headers to responses
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Set security headers
			c.Response().Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			c.Response().Header().Set("X-Frame-Options", "sameorigin")
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Content-Security-Policy", "default-src 'none'; media-src 'self'; style-src 'none' 'unsafe-inline'; img-src 'self'")
			c.Response().Header().Set("Referrer-Policy", "no-referrer, strict-origin-when-cross-origin")
			c.Response().Header().Del("Server") // Remove server header

			return next(c)
		}
	}
}
