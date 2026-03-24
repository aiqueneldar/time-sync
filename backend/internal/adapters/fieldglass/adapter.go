// Package fieldglass implements the Adapter interface for SAP Fieldglass.
//
// Authentication: Fieldglass uses OAuth 2.0 Client Credentials flow.
//  1. POST /api/oauth2/v2.0/token  with Basic auth (clientId:secret)
//     form body: grant_type=client_credentials&response_type=token
//  2. Use the returned access_token as  Authorization: Bearer <token>
//     plus the  X-ApplicationKey  header on every request.
//
// Timesheet API: Fieldglass exposes time-sheet entries via its v1 REST API.
// Workers submit time via time-sheet instances linked to work orders.
//
// Reference: SAP Fieldglass REST API Integration General Reference Guide
package fieldglass

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/models"
)

// Adapter implements adapters.Adapter for SAP Fieldglass.
type Adapter struct {
	httpClient *http.Client
}

// New creates a new Fieldglass adapter.
func New() *Adapter {
	return &Adapter{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ─── Identity ──────────────────────────────────────────────────────────────

func (a *Adapter) SystemID() string    { return "fieldglass" }
func (a *Adapter) SystemName() string  { return "SAP Fieldglass" }
func (a *Adapter) Description() string { return "SAP Fieldglass VMS time-reporting" }

// AuthFields returns the credential fields for the frontend login form.
// Fieldglass requires:
//   - Environment URL  (tenant-specific, e.g. https://www.fieldglass.net)
//   - Client ID        (OAuth2 client ID / Fieldglass username)
//   - Client Secret    (OAuth2 client secret / application password or license key)
//   - Application Key  (optional but required by many tenants)
func (a *Adapter) AuthFields() []models.AuthField {
	return []models.AuthField{
		{
			Key:         "envUrl",
			Label:       "Fieldglass Environment URL",
			Type:        models.AuthFieldTypeURL,
			Placeholder: "https://www.fieldglass.net",
			Required:    true,
			HelpText:    "Your tenant-specific Fieldglass environment URL",
		},
		{
			Key:      "clientId",
			Label:    "Client ID / Username",
			Type:     models.AuthFieldTypeText,
			Required: true,
			HelpText: "OAuth2 client ID (your Fieldglass username)",
		},
		{
			Key:      "clientSecret",
			Label:    "Client Secret / Password",
			Type:     models.AuthFieldTypePassword,
			Required: true,
			HelpText: "OAuth2 client secret or application password",
		},
		{
			Key:      "apiKey",
			Label:    "Application Key (X-ApplicationKey)",
			Type:     models.AuthFieldTypeText,
			Required: false,
			HelpText: "Optional API key from Fieldglass Configuration Manager",
		},
	}
}

// ─── Authentication ────────────────────────────────────────────────────────

// fieldglassTokenResponse is the JSON shape of a successful OAuth2 token response.
type fieldglassTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Authenticate performs the OAuth2 client-credentials token exchange.
//
// POST {envUrl}/api/oauth2/v2.0/token
//
//	Authorization: Basic <base64(clientId:clientSecret)>
//	Content-Type:  application/x-www-form-urlencoded
//	Body:          grant_type=client_credentials&response_type=token
func (a *Adapter) Authenticate(ctx context.Context, fields map[string]string) (*models.AuthResult, error) {
	envURL := strings.TrimRight(fields["envUrl"], "/")
	clientID := fields["clientId"]
	clientSecret := fields["clientSecret"]
	apiKey := fields["apiKey"]

	if envURL == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("fieldglass: envUrl, clientId and clientSecret are required")
	}

	tokenURL := fmt.Sprintf("%s/api/oauth2/v2.0/token", envURL)

	formData := url.Values{}
	formData.Set("grant_type", "client_credentials")
	formData.Set("response_type", "token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("fieldglass: build token request: %w", err)
	}

	// Basic auth: clientId:clientSecret
	credentials := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	req.Header.Set("Authorization", "Basic "+credentials)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("X-ApplicationKey", apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fieldglass: token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("fieldglass: authentication failed – check clientId and clientSecret")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fieldglass: token error %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp fieldglassTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("fieldglass: decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("fieldglass: empty access token in response")
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600 // default 1 hour
	}

	extra := map[string]string{
		"envUrl": envURL,
	}
	if apiKey != "" {
		extra["apiKey"] = apiKey
	}

	return &models.AuthResult{
		SystemID:     "fieldglass",
		TokenType:    "bearer",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		Extra:        extra,
	}, nil
}

// RefreshAuth obtains a new access token using a refresh token if one is available.
// If no refresh token is present the user must re-authenticate.
func (a *Adapter) RefreshAuth(ctx context.Context, auth *models.AuthResult) (*models.AuthResult, error) {
	if auth.RefreshToken == "" {
		return nil, fmt.Errorf("fieldglass: no refresh token – please log in again")
	}

	envURL := auth.Extra["envUrl"]
	apiKey := auth.Extra["apiKey"]
	tokenURL := fmt.Sprintf("%s/api/oauth2/v2.0/token", envURL)

	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("refresh_token", auth.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("fieldglass: build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("X-ApplicationKey", apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fieldglass: refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fieldglass: refresh error %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp fieldglassTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("fieldglass: decode refresh response: %w", err)
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}

	auth.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		auth.RefreshToken = tokenResp.RefreshToken
	}
	auth.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return auth, nil
}

// ValidateAuth sends a lightweight request to check the token is still live.
func (a *Adapter) ValidateAuth(ctx context.Context, auth *models.AuthResult) (bool, error) {
	if auth.IsExpired() {
		return false, nil
	}

	envURL := auth.Extra["envUrl"]
	pingURL := fmt.Sprintf("%s/api/v1/ping", envURL) // lightweight ping

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return false, err
	}
	a.setAuthHeaders(req, auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	// 401 means invalid; anything else (including 404 for missing ping endpoint) is "valid enough"
	return resp.StatusCode != http.StatusUnauthorized, nil
}

// ─── Data retrieval ────────────────────────────────────────────────────────

// fieldglassWorkOrder is a raw work order record from the Fieldglass API.
type fieldglassWorkOrder struct {
	ID          string `json:"id"`
	WorkOrderID string `json:"workOrderId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	JobPosting  struct {
		Title string `json:"title"`
	} `json:"jobPosting"`
}

// fieldglassWorkOrdersResponse wraps the paged list response.
type fieldglassWorkOrdersResponse struct {
	Data  []fieldglassWorkOrder `json:"data"`
	Total int                   `json:"total"`
}

// GetAvailableRows fetches the worker's active work orders from Fieldglass.
// These work orders are the bookable targets for time entry.
func (a *Adapter) GetAvailableRows(ctx context.Context, auth *models.AuthResult) ([]models.SystemRow, error) {
	envURL := auth.Extra["envUrl"]

	// GET /api/v1/workOrders?status=active returns the worker's current assignments.
	apiURL := fmt.Sprintf("%s/api/v1/workOrders?status=active&limit=100", envURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("fieldglass: build work-orders request: %w", err)
	}
	a.setAuthHeaders(req, auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fieldglass: get work-orders: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fieldglass: work-orders %d: %s", resp.StatusCode, body)
	}

	var result fieldglassWorkOrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("fieldglass: decode work-orders: %w", err)
	}

	rows := make([]models.SystemRow, 0, len(result.Data))
	for _, wo := range result.Data {
		name := wo.Title
		if name == "" {
			name = wo.JobPosting.Title
		}
		rows = append(rows, models.SystemRow{
			ID:          wo.ID,
			SystemID:    "fieldglass",
			Code:        wo.WorkOrderID,
			Name:        name,
			Description: wo.Description,
		})
	}
	return rows, nil
}

// ─── Submission ────────────────────────────────────────────────────────────

// fieldglassTimesheetPost is the JSON body for creating a Fieldglass timesheet.
type fieldglassTimesheetPost struct {
	WorkOrderID string                    `json:"workOrderId"`
	PeriodStart string                    `json:"periodStart"` // YYYY-MM-DD
	PeriodEnd   string                    `json:"periodEnd"`
	Lines       []fieldglassTimesheetLine `json:"lines"`
}

// fieldglassTimesheetLine carries one day's hours.
type fieldglassTimesheetLine struct {
	Date  string  `json:"date"` // YYYY-MM-DD
	Hours float64 `json:"hours"`
}

// SubmitEntries creates or updates time-sheet records in Fieldglass.
// Each SystemTimeEntry translates to one work-order timesheet for the week.
func (a *Adapter) SubmitEntries(ctx context.Context, auth *models.AuthResult, entries []models.SystemTimeEntry) (*models.SubmitResult, error) {
	envURL := auth.Extra["envUrl"]
	apiURL := fmt.Sprintf("%s/api/v1/timesheets", envURL)

	result := &models.SubmitResult{SystemID: "fieldglass", Success: true}

	for _, entry := range entries {
		// Build daily lines for Mon-Sun.
		var lines []fieldglassTimesheetLine
		for i, hours := range entry.DailyHours {
			if hours > 0 {
				day := entry.WeekStart.AddDate(0, 0, i)
				lines = append(lines, fieldglassTimesheetLine{
					Date:  day.Format("2006-01-02"),
					Hours: hours,
				})
			}
		}

		periodEnd := entry.WeekStart.AddDate(0, 0, 6)
		payload := fieldglassTimesheetPost{
			WorkOrderID: entry.RowID,
			PeriodStart: entry.WeekStart.Format("2006-01-02"),
			PeriodEnd:   periodEnd.Format("2006-01-02"),
			Lines:       lines,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			result.Success = false
			result.Details = append(result.Details, fmt.Sprintf("marshal %s: %v", entry.RowCode, err))
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
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
			result.Details = append(result.Details, fmt.Sprintf("submitted work-order %s OK", entry.RowCode))
		}
	}

	if result.Success {
		result.Message = "All entries submitted to Fieldglass"
	} else {
		result.Message = "Some entries failed – see details"
	}
	return result, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// setAuthHeaders attaches Bearer token and optional API key to a request.
func (a *Adapter) setAuthHeaders(req *http.Request, auth *models.AuthResult) {
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	req.Header.Set("Accept", "application/json")
	if apiKey := auth.Extra["apiKey"]; apiKey != "" {
		req.Header.Set("X-ApplicationKey", apiKey)
	}
}
