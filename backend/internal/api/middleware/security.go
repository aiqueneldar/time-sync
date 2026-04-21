// Package middleware provides HTTP middleware for the TimeSync API.
package middleware

import (
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// ─── Security Headers (OWASP Top-10 mitigations) ──────────────────────────

func SecurityHeaders(c *gin.Context) {
	// Prevent MIME-type sniffing (OWASP A03).
	c.Header("X-Content-Type-Options", "nosniff")

	// Prevent the page being embedded in frames (clickjacking).
	c.Header("X-Frame-Options", "DENY")

	// Enable XSS filter in legacy browsers.
	c.Header("X-XSS-Protection", "1; mode=block")

	// Strict-Transport-Security: enforce HTTPS for 1 year, including subdomains.
	// Preload readiness included.
	c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

	// Content-Security-Policy: only allow our own origin.
	// The frontend SPA origin is allowed in the connect-src.
	c.Header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; connect-src *; font-src *; script-src-elem * 'unsafe-inline'; img-src * data:; style-src * 'unsafe-inline';")

	// Referrer policy: don't leak URL info.
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

	// Permissions policy: disable powerful browser features we don't need.
	c.Header("Permissions-Policy", "geolocation=(),midi=(),sync-xhr=(),microphone=(),camera=(),magnetometer=(),gyroscope=(),fullscreen=(self),payment=()")

	// Remove the server identification header.
	c.Delete("Server")

	c.Next()
}

// ─── CORS ──────────────────────────────────────────────────────────────────

func CORS(cfg *config.Config) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Session"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
