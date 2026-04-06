package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/aiqueneldar/time-sync/backend/internal/session"
)

// AuthHandler handles authentication operations for a specific system.
//
//	POST   /api/auth/{systemID}   – authenticate (or initiate OIDC flow)
//	GET    /api/auth/{systemID}   – check current auth status
//	DELETE /api/auth/{systemID}   – logout (remove token from session)
//
// OIDC two-step flow
// ──────────────────
// Step 1 – initial POST with {baseUrl, company}:
//
//	The adapter calls GET <authRoot> on Maconomy, discovers the OIDC provider,
//	and returns an *OIDCRequiredError.  The handler stores the pending state in
//	the session and replies HTTP 202:
//	  { "status": "oidc_required", "authUrl": "...", "redirectUri": "..." }
//
// Step 2 – code-exchange POST with {_oidcCode, _oidcState, baseUrl, company}:
//
//	The handler verifies the state nonce (CSRF guard), injects _oidcRedirectUri
//	from the session, and calls Authenticate again.  On success the reconnect
//	token is stored and HTTP 200 is returned.
type AuthHandler struct {
	registry *adapters.Registry
	sessions *session.Store
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(registry *adapters.Registry, sessions *session.Store) *AuthHandler {
	return &AuthHandler{registry: registry, sessions: sessions}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

// handleStatus returns the sanitised auth status. Token values are never exposed.
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

// handleAuthenticate covers both the initial credential POST and the OIDC
// code-exchange POST.  The distinction is made by the presence of _oidcCode
// in the request body.
func (h *AuthHandler) handleAuthenticate(w http.ResponseWriter, r *http.Request, sess *session.Session, adapter adapters.Adapter) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	body, err := io.ReadAll(r.Body)
	var fields map[string]string
	if err := json.Unmarshal(body, &fields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	// ── OIDC code exchange (Step 2) ────────────────────────────────────────
	// The frontend sends _oidcCode and _oidcState after the popup completes.
	if oidcCode := fields["_oidcCode"]; oidcCode != "" {
		h.handleOIDCCodeExchange(w, r, sess, adapter, fields, oidcCode)
		return
	}

	// ── Normal / OIDC discovery (Step 1) ──────────────────────────────────
	// Validate required fields (skipping internal _oidc* keys).
	for _, af := range adapter.AuthFields() {
		if af.Required && strings.TrimSpace(fields[af.Key]) == "" {
			writeError(w, http.StatusBadRequest, "missing required field: "+af.Key)
			return
		}
	}

	auth, err := adapter.Authenticate(r.Context(), fields)
	if err != nil {
		// Check whether the adapter is signalling that OIDC is required.
		var oidcErr *models.OIDCRequiredError
		if errors.As(err, &oidcErr) {
			h.handleOIDCRequired(w, sess, adapter.SystemID(), oidcErr)
			return
		}

		status := http.StatusUnauthorized
		if !isAuthError(err) {
			status = http.StatusBadGateway
		}
		writeError(w, status, err.Error())
		return
	}

	h.storeAuthAndRespond(w, sess, adapter.SystemID(), auth)
}

// handleOIDCRequired is called when the adapter returns *OIDCRequiredError.
// It stores the pending state in the session (for later state verification)
// and replies HTTP 202 so the frontend knows to open the Azure login popup.
//
// The state nonce and redirect URI are stored server-side only.
// Only the auth URL and redirect URI are returned to the browser –
// the state is NOT included so the browser cannot forge the second request.
func (h *AuthHandler) handleOIDCRequired(
	w http.ResponseWriter,
	sess *session.Session,
	systemID string,
	oidcErr *models.OIDCRequiredError,
) {
	// Persist state + context for the upcoming code-exchange call.
	sess.SetOIDCPending(systemID, &models.OIDCPendingState{
		State:       oidcErr.State,
		RedirectURI: oidcErr.RedirectURI,
		BaseURL:     oidcErr.BaseURL,
		Company:     oidcErr.Company,
	})

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "oidc_required",
		"authUrl": oidcErr.AuthURL,
		// Return the redirect URI so the frontend knows which URL the popup
		// will land on and can set up its postMessage listener correctly.
		"redirectUri": oidcErr.RedirectURI,
	})
}

// handleOIDCCodeExchange verifies the OIDC state nonce, injects the stored
// redirect URI, and calls Authenticate again to complete the token exchange.
func (h *AuthHandler) handleOIDCCodeExchange(
	w http.ResponseWriter,
	r *http.Request,
	sess *session.Session,
	adapter adapters.Adapter,
	fields map[string]string,
	code string,
) {
	pending := sess.GetOIDCPending(adapter.SystemID())
	if pending == nil {
		writeError(w, http.StatusBadRequest,
			"no OIDC flow in progress for this session – please start from the login form")
		return
	}

	// ── State verification (CSRF guard) ───────────────────────────────────
	// The frontend extracts the state from the Azure redirect URL and echoes
	// it back.  We compare it against what we stored to ensure the response
	// is from the same flow we initiated.
	receivedState := fields["_oidcState"]
	if receivedState == "" || receivedState != pending.State {
		// Clear the stale pending state so the user can start over.
		sess.ClearOIDCPending(adapter.SystemID())
		writeError(w, http.StatusBadRequest,
			"OIDC state mismatch – potential CSRF attempt; please try logging in again")
		return
	}

	// Inject the redirect URI and stored context into the fields map so the
	// adapter can construct the X-OIDC-Code credential without the frontend
	// needing to remember or re-send these values.
	fields["_oidcRedirectUri"] = pending.RedirectURI
	fields["baseUrl"] = pending.BaseURL
	fields["company"] = pending.Company

	auth, err := adapter.Authenticate(r.Context(), fields)
	if err != nil {
		sess.ClearOIDCPending(adapter.SystemID())
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Code exchange succeeded – clean up the pending state.
	sess.ClearOIDCPending(adapter.SystemID())
	h.storeAuthAndRespond(w, sess, adapter.SystemID(), auth)
}

// storeAuthAndRespond saves the AuthResult in the session and returns a
// sanitised AuthStatus to the caller (no tokens).
func (h *AuthHandler) storeAuthAndRespond(
	w http.ResponseWriter,
	sess *session.Session,
	systemID string,
	auth *models.AuthResult,
) {
	sess.SetAuth(systemID, auth)

	sess.Mu.Lock()
	if !containsString(sess.SelectedSystems, systemID) {
		sess.SelectedSystems = append(sess.SelectedSystems, systemID)
	}
	sess.Mu.Unlock()

	writeJSON(w, http.StatusOK, models.AuthStatus{
		SystemID:      systemID,
		Authenticated: true,
		ExpiresAt:     auth.ExpiresAt,
	})
}

// handleLogout removes the auth and any pending OIDC state for the system.
func (h *AuthHandler) handleLogout(w http.ResponseWriter, sess *session.Session, systemID string) {
	sess.Mu.Lock()
	delete(sess.Auth, systemID)
	updated := make([]string, 0, len(sess.SelectedSystems))
	for _, s := range sess.SelectedSystems {
		if s != systemID {
			updated = append(updated, s)
		}
	}
	sess.SelectedSystems = updated
	sess.Mu.Unlock()

	sess.ClearOIDCPending(systemID)

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func extractPathSegment(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	seg := strings.TrimPrefix(path, prefix)
	if i := strings.Index(seg, "/"); i >= 0 {
		seg = seg[:i]
	}
	return seg
}

func isAuthError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "invalid") ||
		strings.Contains(msg, "rejected")
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
