// Package adapters defines the Adapter interface that every time-reporting system
// integration must satisfy. Adding a new system is as simple as:
//  1. Create a new sub-package under adapters/
//  2. Implement the Adapter interface
//  3. Register the adapter in registry.go
//
// No other files need to be changed.
package adapters

import (
	"context"

	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Adapter is the universal contract for a time-reporting system integration.
// Each method receives a context so that request cancellations propagate correctly.
type Adapter interface {
	// ─── Identity ──────────────────────────────────────────────────────────

	// SystemID returns the unique, stable machine-readable identifier for this
	// system (e.g. "maconomy", "fieldglass"). Used as map keys and API path segments.
	SystemID() string

	// SystemName returns the human-readable display name (e.g. "Deltek Maconomy").
	SystemName() string

	// Description returns a short description shown in the system-selection UI.
	Description() string

	// ─── Authentication ────────────────────────────────────────────────────

	// Authenticate exchanges user-supplied credentials for an AuthResult
	// (access token, reconnect token, etc.). The result is stored in the
	// in-memory session store – never written to disk.
	Authenticate(c *gin.Context, fields map[string]string) (uuid.UUID, error)

	// ValidateAuth performs a lightweight check (e.g. token introspection or
	// a cheap API call) to confirm the stored auth is still valid.
	// Returns (true, nil) if valid, (false, nil) if cleanly expired.
	ValidateAuth(c *gin.Context, authID uuid.UUID) (bool, error)

	// ─── Data retrieval ────────────────────────────────────────────────────

	// GetAvailableRows fetches the list of bookable rows from the system
	// (jobs, tasks, work orders …). These are shown below the time-entry
	// table so users can choose which backend row to map to.
	GetAvailableRows(c *gin.Context, authID uuid.UUID) ([]models.SystemRow, error)

	// ─── Submission ────────────────────────────────────────────────────────

	// SubmitEntries converts the normalised SystemTimeEntry slice into the
	// wire format expected by the system and submits it.
	SubmitEntries(ctx context.Context, auth *models.Auth, entries []models.SystemTimeEntry) (*models.SubmitResult, error)
}
