package handlers

import (
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
)

// TimesheetsHandler handles GET /api/timesheets/{systemID}/rows.
// It fetches the bookable rows (jobs, work orders …) from the given system
// using the authenticated session token.
type TimesheetsHandler struct {
	registry *adapters.Registry
}

// NewTimesheetsHandler creates a TimesheetsHandler.
func NewTimesheetsHandler(registry *adapters.Registry) *TimesheetsHandler {
	return &TimesheetsHandler{registry: registry}
}

func (h *TimesheetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}
