// Package api assembles the HTTP router and wires all handlers and middleware.
package api

import (
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/api/handlers"
	"github.com/aiqueneldar/time-sync/backend/internal/api/middleware"
	"github.com/aiqueneldar/time-sync/backend/internal/session"
	synce "github.com/aiqueneldar/time-sync/backend/internal/sync"
)

// NewRouter constructs and returns the root HTTP handler.
// All routes are prefixed with /api/.
func NewRouter(
	registry *adapters.Registry,
	sessions *session.Store,
	engine *synce.Engine,
	allowedOrigins []string,
) http.Handler {
	mux := http.NewServeMux()

	// ── Handlers ──────────────────────────────────────────────────────────

	systemsH := handlers.NewSystemsHandler(registry)
	authH := handlers.NewAuthHandler(registry, sessions)
	timesheetsH := handlers.NewTimesheetsHandler(registry, sessions)
	syncH := handlers.NewSyncHandler(sessions, engine)

	// ── Routes ────────────────────────────────────────────────────────────

	// GET /api/systems  – list all registered systems (no session required)
	mux.Handle("/api/systems", systemsH)

	// /api/auth/{systemID}  – requires session header
	mux.Handle("/api/auth/", middleware.Chain(
		authH,
		middleware.RequireSession,
	))

	// /api/timesheets/{systemID}/rows  – requires session + JSON
	mux.Handle("/api/timesheets/", middleware.Chain(
		timesheetsH,
		middleware.RequireSession,
	))

	// /api/sync  and  /api/sync/status  – requires session
	mux.Handle("/api/sync", middleware.Chain(
		syncH,
		middleware.RequireSession,
		middleware.RequireJSON,
	))
	mux.Handle("/api/sync/status", middleware.Chain(
		syncH,
		middleware.RequireSession,
	))
	mux.Handle("/api/sync/status/poll", middleware.Chain(
		syncH,
		middleware.RequireSession,
	))

	// Health check endpoint – no authentication required.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// ── Global middleware ──────────────────────────────────────────────────
	// Applied outside-in: CORS → SecurityHeaders → mux
	return middleware.Chain(
		mux,
		middleware.SecurityHeaders,
		middleware.CORS(allowedOrigins),
	)
}
