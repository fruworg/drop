package middleware

import (
	"github.com/labstack/echo/v4"
)

// SecurityHeaders adds security-related HTTP headers to responses
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Response().Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			c.Response().Header().Set("X-Frame-Options", "sameorigin")
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'")
			c.Response().Header().Set("Referrer-Policy", "no-referrer, strict-origin-when-cross-origin")
			c.Response().Header().Del("Server")

			return next(c)
		}
	}
}
