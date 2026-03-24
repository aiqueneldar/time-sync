// Package models defines all shared data structures used across the TimeSync backend.
// These models form the contract between adapters, the session store, the sync engine,
// and the HTTP API handlers.
package models

import "time"

// ─────────────────────────────────────────────
// Authentication models
// ─────────────────────────────────────────────

// AuthFieldType describes the kind of input widget to render on the frontend.
type AuthFieldType string

const (
	AuthFieldTypeText     AuthFieldType = "text"
	AuthFieldTypePassword AuthFieldType = "password"
	AuthFieldTypeURL      AuthFieldType = "url"
)

// AuthField describes a single credential field required by an adapter.
// The frontend uses this list to dynamically build the login form for each system.
type AuthField struct {
	Key         string        `json:"key"`         // machine-readable identifier
	Label       string        `json:"label"`       // human-readable label
	Type        AuthFieldType `json:"type"`        // input type hint
	Placeholder string        `json:"placeholder"` // example value
	Required    bool          `json:"required"`
	HelpText    string        `json:"helpText,omitempty"`
}

// AuthResult holds the authentication state for one system in one session.
// This is stored only in-memory; it is NEVER written to disk or a database.
type AuthResult struct {
	SystemID     string            `json:"systemId"`
	TokenType    string            `json:"tokenType"`    // "bearer" | "reconnect"
	AccessToken  string            `json:"accessToken"`  // never sent to frontend
	RefreshToken string            `json:"refreshToken"` // never sent to frontend
	ExpiresAt    time.Time         `json:"expiresAt"`
	Extra        map[string]string `json:"extra,omitempty"` // adapter-specific extras
}

// IsExpired returns true if the access token has expired (with a 30-second buffer).
func (a *AuthResult) IsExpired() bool {
	return time.Now().After(a.ExpiresAt.Add(-30 * time.Second))
}

// AuthStatus is the sanitised view sent to the frontend – no tokens exposed.
type AuthStatus struct {
	SystemID      string    `json:"systemId"`
	Authenticated bool      `json:"authenticated"`
	ExpiresAt     time.Time `json:"expiresAt,omitempty"`
}

// ─────────────────────────────────────────────
// System / adapter metadata models
// ─────────────────────────────────────────────

// SystemInfo describes a registered time-reporting system.
// Returned by GET /api/systems so the frontend knows what is available.
type SystemInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	LogoURL     string      `json:"logoUrl,omitempty"`
	AuthFields  []AuthField `json:"authFields"`
}

// ─────────────────────────────────────────────
// Time-entry models
// ─────────────────────────────────────────────

// DayHours maps each weekday to the number of hours reported.
// Keys: "monday" … "sunday"
type DayHours map[string]float64

// SystemRow is a bookable target row inside a specific backend system
// (a Maconomy job line, a Fieldglass work-order task, etc.).
type SystemRow struct {
	ID          string            `json:"id"`
	SystemID    string            `json:"systemId"`
	Code        string            `json:"code"`        // short reference code
	Name        string            `json:"name"`        // human-readable label
	Description string            `json:"description"` // optional longer description
	Extra       map[string]string `json:"extra,omitempty"`
}

// RowMapping links one frontend TimeEntryRow to a backend SystemRow.
type RowMapping struct {
	SystemID    string `json:"systemId"`
	SystemRowID string `json:"systemRowId"`
}

// TimeEntryRow represents one row in the user-visible time-entry table.
// It carries the user's hours plus zero or more backend mappings.
type TimeEntryRow struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"` // user-defined description
	Hours    DayHours     `json:"hours"` // keyed by lowercase weekday name
	Mappings []RowMapping `json:"mappings"`
}

// WeekDate identifies a calendar week.
type WeekDate struct {
	Year int `json:"year"`
	Week int `json:"week"` // ISO week number
}

// TimesheetInput is the full payload sent by the frontend on Sync.
type TimesheetInput struct {
	Week WeekDate       `json:"week"`
	Rows []TimeEntryRow `json:"rows"`
}

// ─────────────────────────────────────────────
// Adapter-internal time entry
// ─────────────────────────────────────────────

// SystemTimeEntry is the normalised entry passed from the sync engine to an adapter.
// The adapter is responsible for translating this into its own wire format.
type SystemTimeEntry struct {
	RowID       string // SystemRow.ID to book against
	RowCode     string // SystemRow.Code (some APIs need the code, not the ID)
	WeekStart   time.Time
	DailyHours  [7]float64 // index 0 = Monday … 6 = Sunday
	Description string
}

// SubmitResult carries the outcome of one adapter's SubmitEntries call.
type SubmitResult struct {
	SystemID string
	Success  bool
	Message  string
	Details  []string // per-entry messages
}

// ─────────────────────────────────────────────
// Sync-status models
// ─────────────────────────────────────────────

// SyncState enumerates the possible states of a sync operation.
type SyncState string

const (
	SyncStatePending SyncState = "pending"
	SyncStateSyncing SyncState = "syncing"
	SyncStateSynced  SyncState = "synced"
	SyncStateError   SyncState = "error"
)

// SyncStatus tracks the synchronisation state for one system within one session.
type SyncStatus struct {
	SystemID  string    `json:"systemId"`
	State     SyncState `json:"state"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SyncReport aggregates statuses for all systems in a session.
type SyncReport struct {
	SessionID string       `json:"sessionId"`
	Statuses  []SyncStatus `json:"statuses"`
}
