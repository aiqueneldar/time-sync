package handlers

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/aiqueneldar/time-sync/backend/internal/session"
)

// AuthHandler handles authentication operations for a specific system.
//
//	POST   /api/auth/{systemID}   – authenticate (exchange credentials for token)
//	GET    /api/auth/{systemID}   – check current auth status
//	DELETE /api/auth/{systemID}   – logout (remove token from session)
type AuthHandler struct {
	registry *adapters.Registry
	sessions *session.Store
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(registry *adapters.Registry, sessions *session.Store) *AuthHandler {
	return &AuthHandler{registry: registry, sessions: sessions}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract system ID from the path: /api/auth/{systemID}
	systemID := extractPathSegment(r.URL.Path, "/api/auth/")
	if systemID == "" {
		writeError(w, http.StatusBadRequest, "missing systemId in path")
		return
	}

	adapter, ok := h.registry.Get(systemID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown system: "+systemID)
		return
	}

	sess := h.sessions.GetOrCreate(r.Header.Get("X-Session-ID"))

	switch r.Method {
	case http.MethodGet:
		h.handleStatus(w, sess, systemID)
	case http.MethodPost:
		h.handleAuthenticate(w, r, sess, adapter)
	case http.MethodDelete:
		h.handleLogout(w, sess, systemID)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleStatus returns the sanitised auth status for the given system.
// Token values are never included in the response.
func (h *AuthHandler) handleStatus(w http.ResponseWriter, sess *session.Session, systemID string) {
	auth := sess.GetAuth(systemID)
	status := models.AuthStatus{
		SystemID:      systemID,
		Authenticated: auth != nil && !auth.IsExpired(),
	}
	if auth != nil {
		status.ExpiresAt = auth.ExpiresAt
	}
	writeJSON(w, http.StatusOK, status)
}

// handleAuthenticate exchanges credentials for an auth token and stores it
// in the session.  Credentials are accepted from the request body only and
// are never logged or stored beyond the token exchange.
func (h *AuthHandler) handleAuthenticate(w http.ResponseWriter, r *http.Request, sess *session.Session, adapter adapters.Adapter) {
	// Limit request body to 64 KB to prevent DoS (OWASP A04).
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var fields map[string]string
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	// Validate that all required auth fields are present.
	for _, af := range adapter.AuthFields() {
		if af.Required && strings.TrimSpace(fields[af.Key]) == "" {
			writeError(w, http.StatusBadRequest, "missing required field: "+af.Key)
			return
		}
	}

	auth, err := adapter.Authenticate(r.Context(), fields)
	if err != nil {
		// Return 401 for auth failures; 502 for upstream errors.
		status := http.StatusUnauthorized
		if !isAuthError(err) {
			status = http.StatusBadGateway
		}
		writeError(w, status, err.Error())
		return
	}

	// Store in session – NEVER returned to caller.
	sess.SetAuth(adapter.SystemID(), auth)

	// Update selected systems list.
	sess.Mu.Lock()
	if !slices.Contains(sess.SelectedSystems, adapter.SystemID()) {
		sess.SelectedSystems = append(sess.SelectedSystems, adapter.SystemID())
	}
	sess.Mu.Unlock()

	writeJSON(w, http.StatusOK, models.AuthStatus{
		SystemID:      adapter.SystemID(),
		Authenticated: true,
		ExpiresAt:     auth.ExpiresAt,
	})
}

// handleLogout removes the auth token for the given system from the session.
func (h *AuthHandler) handleLogout(w http.ResponseWriter, sess *session.Session, systemID string) {
	sess.Mu.Lock()
	delete(sess.Auth, systemID)
	// Remove from selected systems.
	updated := make([]string, 0, len(sess.SelectedSystems))
	for _, s := range sess.SelectedSystems {
		if s != systemID {
			updated = append(updated, s)
		}
	}
	sess.SelectedSystems = updated
	sess.Mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// extractPathSegment extracts the URL path segment that follows prefix.
func extractPathSegment(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	seg := strings.TrimPrefix(path, prefix)
	// Discard any trailing path components.
	if i := strings.Index(seg, "/"); i >= 0 {
		seg = seg[:i]
	}
	return seg
}

// isAuthError returns true if the error message suggests an authentication
// failure (wrong credentials) rather than a network / server error.
func isAuthError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "invalid")
}
