// Package middleware provides HTTP middleware for the TimeSync API.
package middleware

import (
	"net/http"
	"strings"
)

// ─── Security Headers (OWASP Top-10 mitigations) ──────────────────────────

// SecurityHeaders sets a comprehensive set of security-related HTTP response
// headers on every response.
//
// Addresses OWASP A05: Security Misconfiguration.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Prevent MIME-type sniffing (OWASP A03).
		h.Set("X-Content-Type-Options", "nosniff")

		// Prevent the page being embedded in frames (clickjacking).
		h.Set("X-Frame-Options", "DENY")

		// Enable XSS filter in legacy browsers.
		h.Set("X-XSS-Protection", "1; mode=block")

		// Strict-Transport-Security: enforce HTTPS for 1 year, including subdomains.
		// Preload readiness included.
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

		// Content-Security-Policy: only allow our own origin.
		// The frontend SPA origin is allowed in the connect-src.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Referrer policy: don't leak URL info.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy: disable powerful browser features we don't need.
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		// Remove the server identification header.
		h.Del("Server")

		next.ServeHTTP(w, r)
	})
}

// ─── CORS ──────────────────────────────────────────────────────────────────

// CORS returns a middleware that enforces a strict Cross-Origin Resource
// Sharing policy.
//
// Only origins in the allowedOrigins list are permitted.  Wildcard "*" is
// NOT allowed for credentialed requests; each origin must be explicit.
//
// Addresses OWASP A01: Broken Access Control.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	// Build a fast lookup set.
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[strings.ToLower(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && originSet[strings.ToLower(origin)] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "false") // tokens via header, not cookies
				w.Header().Set("Access-Control-Expose-Headers", "X-Session-ID")
			}

			// Handle preflight OPTIONS requests.
			if r.Method == http.MethodOptions {
				if origin != "" && originSet[strings.ToLower(origin)] {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers",
						"Content-Type, X-Session-ID, Accept, Authorization")
					w.Header().Set("Access-Control-Max-Age", "600") // cache preflight for 10 min
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ─── Session validation ────────────────────────────────────────────────────

// RequireSession rejects requests that do not carry a well-formed
// X-Session-ID header.  The header value must be a 36-character UUID.
//
// This doubles as CSRF protection for API endpoints:
// browsers cannot automatically attach custom headers to cross-origin
// requests, so requiring X-Session-ID prevents CSRF attacks without
// needing a separate CSRF token (OWASP A01, A07).
func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("X-Session-ID")
		if !isValidSessionID(sessionID) {
			http.Error(w, `{"error":"missing or invalid X-Session-ID header"}`, http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isValidSessionID performs a lightweight format check on a UUID v4 string.
// We accept any 36-character string in the standard UUID format
// (8-4-4-4-12 hex digits separated by hyphens).
func isValidSessionID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !isHexChar(byte(c)) {
			return false
		}
	}
	return true
}

func isHexChar(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// ─── Content-Type enforcement ──────────────────────────────────────────────

// RequireJSON rejects non-GET requests that do not declare
// Content-Type: application/json.
// Addresses OWASP A03 (injection via unexpected content types).
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodOptions && r.Method != http.MethodDelete {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				http.Error(w, `{"error":"Content-Type must be application/json"}`, http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Chain applies a list of middleware functions right-to-left (outermost first).
// Usage:  Chain(h, mw1, mw2, mw3)  →  mw1(mw2(mw3(h)))
func Chain(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}
