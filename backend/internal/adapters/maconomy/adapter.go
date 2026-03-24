// Package maconomy implements the Adapter interface for Deltek Maconomy ERP.
//
// Authentication: Maconomy uses a proprietary "Reconnect" scheme.
//  1. POST /containers/{company}/api/authentication  with X-Basic credentials
//  2. Parse the reconnect token from the response
//  3. All subsequent requests use  Authorization: X-Reconnect <token>
//
// Timesheet API: Maconomy exposes timesheets via its Container Web Service.
// Week timesheet lines live under the "timesheets" container.
//
// Reference: Maconomy RESTful Web Services Programmer's Guide (Deltek, v2.6+)
package maconomy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/models"
)

// Adapter implements adapters.Adapter for Deltek Maconomy.
type Adapter struct {
	// BaseURL is the root URL of the Maconomy RESTful web services,
	// e.g. https://maconomy.example.com/maconomy-restapi
	BaseURL string

	// Company is the Maconomy company name used in API paths.
	Company string

	httpClient *http.Client
}

// New creates a configured Maconomy adapter.
// baseURL must not have a trailing slash.
func New(baseURL, company string) *Adapter {
	return &Adapter{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Company: company,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ─── Identity ──────────────────────────────────────────────────────────────

func (a *Adapter) SystemID() string    { return "maconomy" }
func (a *Adapter) SystemName() string  { return "Deltek Maconomy" }
func (a *Adapter) Description() string { return "Deltek Maconomy ERP time-reporting" }

// AuthFields returns the credential fields shown in the frontend login form.
func (a *Adapter) AuthFields() []models.AuthField {
	return []models.AuthField{
		{
			Key:         "baseUrl",
			Label:       "Maconomy URL",
			Type:        models.AuthFieldTypeURL,
			Placeholder: "https://maconomy.company.com/maconomy-restapi",
			Required:    true,
			HelpText:    "The root URL of your Maconomy REST API",
		},
		{
			Key:         "company",
			Label:       "Company Name",
			Type:        models.AuthFieldTypeText,
			Placeholder: "mycompany",
			Required:    true,
			HelpText:    "The Maconomy company identifier used in API paths",
		},
		{
			Key:      "username",
			Label:    "Username",
			Type:     models.AuthFieldTypeText,
			Required: true,
		},
		{
			Key:      "password",
			Label:    "Password",
			Type:     models.AuthFieldTypePassword,
			Required: true,
		},
	}
}

// ─── Authentication ────────────────────────────────────────────────────────

// authRoot returns the URL for the Authentication Web Service root.
func (a *Adapter) authRoot(baseURL, company string) string {
	return fmt.Sprintf("%s/containers/%s/api/authentication", baseURL, company)
}

// Authenticate performs the Maconomy X-Basic → X-Reconnect token exchange.
//
// Flow (per Maconomy REST API Programmer's Guide §2.8):
//  1. GET  <authRoot>  →  may return reconnect info
//  2. POST <authRoot>  with Authorization: X-Basic <b64(user:pwd)>
//  3. Extract "Maconomy-Reconnect" from response headers
func (a *Adapter) Authenticate(ctx context.Context, fields map[string]string) (*models.AuthResult, error) {
	baseURL := strings.TrimRight(fields["baseUrl"], "/")
	company := fields["company"]
	username := fields["username"]
	password := fields["password"]

	if baseURL == "" || company == "" || username == "" || password == "" {
		return nil, fmt.Errorf("maconomy: all fields are required")
	}

	authURL := a.authRoot(baseURL, company)

	// Step 1: GET the root to initialise the session and obtain any initial
	// reconnect seed (required since Maconomy 2.6.2).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("maconomy: build auth GET: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	initResp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maconomy: auth GET failed: %w", err)
	}
	defer initResp.Body.Close()
	io.Copy(io.Discard, initResp.Body) // drain body

	// Step 2: POST with Basic credentials.
	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("maconomy: build auth POST: %w", err)
	}
	req.Header.Set("Authorization", "X-Basic "+credentials)
	req.Header.Set("Accept", "application/json")

	// Carry the reconnect seed from the GET response if present.
	if seed := initResp.Header.Get("Maconomy-Reconnect"); seed != "" {
		req.Header.Set("Maconomy-Reconnect", seed)
	}

	authResp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maconomy: auth POST failed: %w", err)
	}
	defer authResp.Body.Close()

	body, _ := io.ReadAll(authResp.Body)

	if authResp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("maconomy: authentication failed – check username and password")
	}
	if authResp.StatusCode >= 400 {
		return nil, fmt.Errorf("maconomy: authentication error %d: %s", authResp.StatusCode, string(body))
	}

	// Step 3: Extract the reconnect token.
	reconnectToken := authResp.Header.Get("Maconomy-Reconnect")
	if reconnectToken == "" {
		// Some versions embed it in the JSON body.
		var bodyMap map[string]interface{}
		if json.Unmarshal(body, &bodyMap) == nil {
			if t, ok := bodyMap["reconnectToken"].(string); ok {
				reconnectToken = t
			}
		}
	}

	if reconnectToken == "" {
		return nil, fmt.Errorf("maconomy: no reconnect token received – check server version")
	}

	return &models.AuthResult{
		SystemID:    "maconomy",
		TokenType:   "reconnect",
		AccessToken: reconnectToken,
		// Maconomy reconnect tokens don't carry explicit expiry;
		// use a conservative 8-hour window to prompt re-auth.
		ExpiresAt: time.Now().Add(8 * time.Hour),
		Extra: map[string]string{
			"baseUrl": baseURL,
			"company": company,
		},
	}, nil
}

// RefreshAuth re-authenticates. Maconomy reconnect tokens cannot be refreshed
// without the original credentials, so we return an error signalling the frontend
// to re-prompt the user.
func (a *Adapter) RefreshAuth(_ context.Context, _ *models.AuthResult) (*models.AuthResult, error) {
	return nil, fmt.Errorf("maconomy: session expired – please log in again")
}

// ValidateAuth issues a lightweight request to the Maconomy auth endpoint
// using the reconnect token to check validity.
func (a *Adapter) ValidateAuth(ctx context.Context, auth *models.AuthResult) (bool, error) {
	if auth.IsExpired() {
		return false, nil
	}

	baseURL := auth.Extra["baseUrl"]
	company := auth.Extra["company"]
	authURL := a.authRoot(baseURL, company)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "X-Reconnect "+auth.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	return resp.StatusCode < 400, nil
}

// ─── Data retrieval ────────────────────────────────────────────────────────

// maconomyTimesheetLine is the raw JSON shape returned by the timesheets container.
type maconomyTimesheetLine struct {
	JobNumber    string `json:"jobNumber"`
	TaskName     string `json:"taskName"`
	Description  string `json:"description"`
	ActivityCode string `json:"activityCode"`
}

// maconomyFilterResponse wraps a paged list of timesheet lines.
type maconomyFilterResponse struct {
	Panes struct {
		Filter struct {
			Records []struct {
				Data maconomyTimesheetLine `json:"data"`
			} `json:"records"`
		} `json:"filter"`
	} `json:"panes"`
}

// GetAvailableRows fetches active jobs/tasks from Maconomy's timesheet container.
// The exact filter URL follows the Container Web Service path convention.
func (a *Adapter) GetAvailableRows(ctx context.Context, auth *models.AuthResult) ([]models.SystemRow, error) {
	baseURL := auth.Extra["baseUrl"]
	company := auth.Extra["company"]

	// Filter the time sheet lines container for entries the current user can book.
	url := fmt.Sprintf("%s/containers/%s/timesheets/filter", baseURL, company)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("maconomy: build rows request: %w", err)
	}
	a.setAuthHeaders(req, auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maconomy: get rows: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("maconomy: get rows %d: %s", resp.StatusCode, body)
	}

	var result maconomyFilterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("maconomy: decode rows: %w", err)
	}

	rows := make([]models.SystemRow, 0, len(result.Panes.Filter.Records))
	for _, rec := range result.Panes.Filter.Records {
		d := rec.Data
		rows = append(rows, models.SystemRow{
			ID:          d.JobNumber + ":" + d.ActivityCode,
			SystemID:    "maconomy",
			Code:        d.JobNumber,
			Name:        d.TaskName,
			Description: d.Description,
			Extra: map[string]string{
				"activityCode": d.ActivityCode,
			},
		})
	}
	return rows, nil
}

// ─── Submission ────────────────────────────────────────────────────────────

// maconomyTimesheetPost is the JSON body for creating a timesheet line.
type maconomyTimesheetPost struct {
	Data struct {
		JobNumber    string  `json:"jobNumber"`
		ActivityCode string  `json:"activityCode,omitempty"`
		Monday       float64 `json:"numberOf1,omitempty"` // Maconomy names days by index
		Tuesday      float64 `json:"numberOf2,omitempty"`
		Wednesday    float64 `json:"numberOf3,omitempty"`
		Thursday     float64 `json:"numberOf4,omitempty"`
		Friday       float64 `json:"numberOf5,omitempty"`
		Saturday     float64 `json:"numberOf6,omitempty"`
		Sunday       float64 `json:"numberOf7,omitempty"`
	} `json:"data"`
}

// SubmitEntries posts each time entry to the Maconomy timesheet container.
// In Maconomy the week is identified by an "instance key" derived from the
// week-start date.
func (a *Adapter) SubmitEntries(ctx context.Context, auth *models.AuthResult, entries []models.SystemTimeEntry) (*models.SubmitResult, error) {
	baseURL := auth.Extra["baseUrl"]
	company := auth.Extra["company"]

	result := &models.SubmitResult{SystemID: "maconomy", Success: true}

	for _, entry := range entries {
		weekKey := entry.WeekStart.Format("2006-01-02")
		url := fmt.Sprintf("%s/containers/%s/timesheets/%s/table", baseURL, company, weekKey)

		var payload maconomyTimesheetPost
		payload.Data.JobNumber = entry.RowCode
		if ac, ok := extractActivityCode(entry.RowID); ok {
			payload.Data.ActivityCode = ac
		}
		payload.Data.Monday = entry.DailyHours[0]
		payload.Data.Tuesday = entry.DailyHours[1]
		payload.Data.Wednesday = entry.DailyHours[2]
		payload.Data.Thursday = entry.DailyHours[3]
		payload.Data.Friday = entry.DailyHours[4]
		payload.Data.Saturday = entry.DailyHours[5]
		payload.Data.Sunday = entry.DailyHours[6]

		body, err := json.Marshal(payload)
		if err != nil {
			result.Success = false
			result.Details = append(result.Details, fmt.Sprintf("marshal %s: %v", entry.RowCode, err))
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			result.Success = false
			result.Details = append(result.Details, fmt.Sprintf("build request %s: %v", entry.RowCode, err))
			continue
		}
		a.setAuthHeaders(req, auth)
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.httpClient.Do(req)
		if err != nil {
			result.Success = false
			result.Details = append(result.Details, fmt.Sprintf("submit %s: %v", entry.RowCode, err))
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			result.Success = false
			result.Details = append(result.Details, fmt.Sprintf("submit %s status %d: %s", entry.RowCode, resp.StatusCode, respBody))
		} else {
			result.Details = append(result.Details, fmt.Sprintf("submitted %s OK", entry.RowCode))
		}
	}

	if result.Success {
		result.Message = "All entries submitted to Maconomy"
	} else {
		result.Message = "Some entries failed – see details"
	}
	return result, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// setAuthHeaders attaches the X-Reconnect authorization header to a request.
func (a *Adapter) setAuthHeaders(req *http.Request, auth *models.AuthResult) {
	req.Header.Set("Authorization", "X-Reconnect "+auth.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Maconomy-Reconnect", auth.AccessToken)
}

// extractActivityCode parses the activity code from a composite row ID "job:activity".
func extractActivityCode(rowID string) (string, bool) {
	parts := strings.SplitN(rowID, ":", 2)
	if len(parts) == 2 {
		return parts[1], true
	}
	return "", false
}
