package handlers

import (
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/session"
)

// TimesheetsHandler handles GET /api/timesheets/{systemID}/rows.
// It fetches the bookable rows (jobs, work orders …) from the given system
// using the authenticated session token.
type TimesheetsHandler struct {
	registry *adapters.Registry
	sessions *session.Store
}

// NewTimesheetsHandler creates a TimesheetsHandler.
func NewTimesheetsHandler(registry *adapters.Registry, sessions *session.Store) *TimesheetsHandler {
	return &TimesheetsHandler{registry: registry, sessions: sessions}
}

func (h *TimesheetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Path: /api/timesheets/{systemID}/rows
	systemID := extractPathSegment(r.URL.Path, "/api/timesheets/")
	if systemID == "" {
		writeError(w, http.StatusBadRequest, "missing systemId in path")
		return
	}

	adapter, ok := h.registry.Get(systemID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown system: "+systemID)
		return
	}

	sess := h.sessions.Get(r.Header.Get("X-Session-ID"))
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "session not found")
		return
	}

	auth := sess.GetAuth(systemID)
	if auth == nil || auth.IsExpired() {
		writeError(w, http.StatusUnauthorized, "not authenticated with "+systemID)
		return
	}

	rows, err := adapter.GetAvailableRows(r.Context(), auth)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch rows from "+adapter.SystemName()+": "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"systemId": systemID,
		"rows":     rows,
	})
}
