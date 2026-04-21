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
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/models"
)

// Engine coordinates sync jobs across adapter instances.
type Engine struct {
	registry *adapters.Registry
}

// New creates a new Engine.
func New(registry *adapters.Registry) *Engine {
	return &Engine{
		registry: registry,
	}
}

// Dispatch triggers an asynchronous sync for the given session.
// It returns immediately; callers should subscribe to the SSE endpoint for updates.
//
// The input is the full timesheet state from the frontend.  The engine resolves
// which systems are involved by inspecting the row mappings and the session's
// authenticated adapters.
func (e *Engine) Dispatch(input *models.TimesheetInput) {

}

// syncSystem performs the actual API call for one system and updates the session status.
func (e *Engine) syncSystem(
	systemID string,
	adp adapters.Adapter,
	auth *models.Auth,
	entries []models.SystemTimeEntry,
) {
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
