// Package handlers contains all HTTP handler functions for the TimeSync API.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
)

// SystemsHandler handles GET /api/systems.
// It returns the list of all registered time-reporting systems with their
// authentication field definitions so the frontend can render login forms.
type SystemsHandler struct {
	registry *adapters.Registry
}

// NewSystemsHandler creates a SystemsHandler.
func NewSystemsHandler(registry *adapters.Registry) *SystemsHandler {
	return &SystemsHandler{registry: registry}
}

// ServeHTTP handles GET /api/systems.
func (h *SystemsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	systems := h.registry.All()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"systems": systems,
	})
}

// ─── Shared JSON helper ────────────────────────────────────────────────────

// writeJSON serialises v to JSON and writes it with the given status code.
// It always sets Content-Type: application/json.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point the status has already been sent; log only.
		_ = err
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
