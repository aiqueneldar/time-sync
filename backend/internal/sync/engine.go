// Package sync implements the asynchronous synchronisation engine.
//
// When the user presses Sync, the engine:
//  1. Marks every targeted system as "syncing".
//  2. Fans out one goroutine per system.
//  3. Each goroutine converts the user's time entries into the adapter's
//     format and calls SubmitEntries.
//  4. Status updates (syncing → synced / error) are written to the session's
//     status channel, which the SSE handler fans out to the browser.
//
// Multiple concurrent syncs for the same session are safe: a new Sync call
// incorporates any additional time entries and re-dispatches all systems.
package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/aiqueneldar/time-sync/backend/internal/session"
)

// Engine coordinates sync jobs across adapter instances.
type Engine struct {
	registry *adapters.Registry
	sessions *session.Store
}

// New creates a new Engine.
func New(registry *adapters.Registry, sessions *session.Store) *Engine {
	return &Engine{
		registry: registry,
		sessions: sessions,
	}
}

// Dispatch triggers an asynchronous sync for the given session.
// It returns immediately; callers should subscribe to the SSE endpoint for updates.
//
// The input is the full timesheet state from the frontend.  The engine resolves
// which systems are involved by inspecting the row mappings and the session's
// authenticated adapters.
func (e *Engine) Dispatch(sess *session.Session, input *models.TimesheetInput) {
	// Persist the latest timesheet state in the session.
	sess.Mu.Lock()
	sess.Timesheet = input
	sess.Mu.Unlock()

	// Determine which systems have at least one mapped entry.
	systemEntries := e.buildSystemEntries(input)

	for systemID, entries := range systemEntries {
		// Only sync systems the user is authenticated with.
		auth := sess.GetAuth(systemID)
		if auth == nil {
			sess.SetSyncStatus(models.SyncStatus{
				SystemID:  systemID,
				State:     models.SyncStateError,
				Message:   "Not authenticated – please log in first",
				UpdatedAt: time.Now(),
			})
			continue
		}

		adapter, ok := e.registry.Get(systemID)
		if !ok {
			continue
		}

		// Mark as syncing immediately so the UI updates before the goroutine starts.
		sess.SetSyncStatus(models.SyncStatus{
			SystemID:  systemID,
			State:     models.SyncStateSyncing,
			Message:   fmt.Sprintf("Syncing %d entries to %s…", len(entries), adapter.SystemName()),
			UpdatedAt: time.Now(),
		})

		// Copy loop variables for the goroutine closure.
		sid := systemID
		ent := entries
		adp := adapter
		au := auth

		go e.syncSystem(sess, sid, adp, au, ent)
	}
}

// syncSystem performs the actual API call for one system and updates the session status.
func (e *Engine) syncSystem(
	sess *session.Session,
	systemID string,
	adp adapters.Adapter,
	auth *models.AuthResult,
	entries []models.SystemTimeEntry,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Refresh the token if it has expired.
	if auth.IsExpired() {
		refreshed, err := adp.RefreshAuth(ctx, auth)
		if err != nil {
			sess.SetSyncStatus(models.SyncStatus{
				SystemID:  systemID,
				State:     models.SyncStateError,
				Message:   fmt.Sprintf("Token expired and refresh failed: %v", err),
				UpdatedAt: time.Now(),
			})
			return
		}
		sess.SetAuth(systemID, refreshed)
		auth = refreshed
	}

	result, err := adp.SubmitEntries(ctx, auth, entries)
	if err != nil {
		sess.SetSyncStatus(models.SyncStatus{
			SystemID:  systemID,
			State:     models.SyncStateError,
			Message:   fmt.Sprintf("Sync failed: %v", err),
			UpdatedAt: time.Now(),
		})
		return
	}

	state := models.SyncStateSynced
	if !result.Success {
		state = models.SyncStateError
	}
	sess.SetSyncStatus(models.SyncStatus{
		SystemID:  systemID,
		State:     state,
		Message:   result.Message,
		UpdatedAt: time.Now(),
	})
}

// buildSystemEntries groups time entries by system ID.
// Each TimeEntryRow can map to multiple systems; this function inverts that
// mapping so each adapter receives only its own entries.
func (e *Engine) buildSystemEntries(input *models.TimesheetInput) map[string][]models.SystemTimeEntry {
	// Derive Monday of the given ISO week.
	weekStart := isoWeekStart(input.Week.Year, input.Week.Week)

	result := make(map[string][]models.SystemTimeEntry)

	for _, row := range input.Rows {
		// Convert DayHours map → fixed [7]float64 array (Mon=0 … Sun=6).
		var dailyHours [7]float64
		weekdays := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
		for i, day := range weekdays {
			dailyHours[i] = row.Hours[day]
		}

		for _, mapping := range row.Mappings {
			entry := models.SystemTimeEntry{
				RowID:       mapping.SystemRowID,
				WeekStart:   weekStart,
				DailyHours:  dailyHours,
				Description: row.Label,
			}
			result[mapping.SystemID] = append(result[mapping.SystemID], entry)
		}
	}

	return result
}

// isoWeekStart returns the Monday of the given ISO year/week.
func isoWeekStart(year, week int) time.Time {
	// January 4 is always in week 1 of its ISO year.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	// Weekday of Jan 4: Monday=0 in our convention.
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7 so Monday is still 1
	}
	// Monday of week 1.
	week1Monday := jan4.AddDate(0, 0, -(weekday - 1))
	// Monday of the target week.
	return week1Monday.AddDate(0, 0, (week-1)*7)
}
