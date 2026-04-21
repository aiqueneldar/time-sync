package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	synce "github.com/aiqueneldar/time-sync/backend/internal/sync"
)

// SyncHandler handles:
//
//	POST /api/sync          – kick off an asynchronous sync
//	GET  /api/sync/status   – SSE stream of SyncStatus updates
//	GET  /api/sync/status/poll – one-shot status poll (SSE fallback)
type SyncHandler struct {
	engine *synce.Engine
}

// NewSyncHandler creates a SyncHandler.
func NewSyncHandler(engine *synce.Engine) *SyncHandler {
	return &SyncHandler{engine: engine}
}

func (h *SyncHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/sync":
	case "/api/sync/status":
		if r.Method == http.MethodGet {
			h.handleSSE(w, r)
		} else {
			//writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	case "/api/sync/status/poll":
		if r.Method == http.MethodGet {
			h.handlePoll(w, r)
		} else {
			//writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		//writeError(w, http.StatusNotFound, "not found")
	}
}

// handleSSE opens a Server-Sent Events stream and forwards SyncStatus updates
// from the session's status channel to the browser.
//
// The connection is kept alive with a heartbeat comment every 15 seconds.
// The browser will automatically reconnect if the connection drops.
func (h *SyncHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	// SSE requires the flusher interface.
	flusher, ok := w.(http.Flusher)
	if !ok {
		//writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	// Session is either in the Header X-Session-ID or as a query parameter named session
	var sessionID string
	if sessionID = r.Header.Get("X-Session-ID"); sessionID == "" {
		sessionID = r.URL.Query().Get("session")
	}

	// SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	// Send a snapshot of current statuses first.

	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected.
			return

		case <-heartbeat.C:
			// Heartbeat comment keeps the connection alive through proxies.
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
		log.Println("SEE sent")
	}
}

// handlePoll returns the current sync statuses as a plain JSON snapshot.
// Used as a fallback when SSE is not available (e.g. older proxies).
func (h *SyncHandler) handlePoll(w http.ResponseWriter, r *http.Request) {

	//writeJSON(w, http.StatusOK, models.SyncReport{
	//	SessionID: sess.ID,
	//	Statuses:  sess.GetAllSyncStatuses(),
	//})
}

// writeSSEEvent encodes v as JSON and writes it as an SSE event.
func writeSSEEvent(w http.ResponseWriter, event string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
}
