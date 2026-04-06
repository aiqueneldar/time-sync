// Package maconomy implements the Adapter interface for Deltek Maconomy ERP.
//
// # Authentication – OIDC Authorization Code Flow
//
// Maconomy can be configured to delegate authentication to an external OpenID
// Connect provider (e.g. Microsoft Azure / Entra ID).  When this is the case the
// login sequence is:
//
//  1. GET  <authRoot>                             – discover available auth schemes
//     and OpenID provider metadata.
//  2. Front-end opens a browser popup to the     – user authenticates with Azure.
//     identity provider's authorization endpoint.
//  3. Azure redirects the popup back to the      – SPA captures `?code=` and posts
//     registered redirect URI with `?code=`.       the code to the parent window.
//  4. Back-end receives the code and constructs:
//     Authorization: X-OIDC-Code <base64("<redirectURI>:<code>")>
//     and POSTs it to the Maconomy auth root to obtain a reconnect token.
//  5. The reconnect token is stored in the in-memory session.  All subsequent
//     API requests use  Authorization: X-Reconnect <token>.
//
// Reference: Maconomy RESTful Web Services Programmer's Guide §4.2.3
package maconomy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/models"
)

// Adapter implements adapters.Adapter for Deltek Maconomy using OIDC auth.
type Adapter struct {
	httpClient   *http.Client
	baseURL      string
	company      string
	api_path     string
	client_id    string
	return_uri   string
	accepts      map[string]string
	contentTypes map[string]string
}

// New creates a Maconomy adapter.
// The baseURL and company parameters are accepted for interface compatibility
// but users supply them per-session via the UI auth fields.
func New(baseURL, company, api_basepath, oauth_client_id, oauth_redirect_url string) *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		company:    company,
		api_path:   getDef(api_basepath, "maconomy-api"),
		client_id:  getDef(oauth_client_id, ""),
		return_uri: getDef(oauth_redirect_url, "http://localhost/"),
		accepts: map[string]string{
			"auth":           "application/vnd.deltek.maconomy.authentication+json; charset=utf-8; version=2.0;",
			"environment":    "application/vnd.deltek.maconomy.environment+json; charset=utf-8; version=2.0",
			"containersv6.1": "application/vnd.deltek.maconomy.containers+json; charset=utf-8; version=6.1",
		},
		contentTypes: map[string]string{
			"container": "application/vnd.deltek.maconomy.containers+json",
		},
	}
}

// ─── Identity ──────────────────────────────────────────────────────────────

func (a *Adapter) SystemID() string    { return "maconomy" }
func (a *Adapter) SystemName() string  { return "Deltek Maconomy" }
func (a *Adapter) Description() string { return "Deltek Maconomy ERP – OIDC" }

// AuthFields returns the two fields the UI collects before the OIDC popup.
// The frontend shows these as a normal form; once submitted the backend
// discovers the OIDC config and responds with HTTP 202 to trigger the popup.
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
	}
}

// ─── OIDC discovery types ──────────────────────────────────────────────────

// maconomyAuthDiscovery is the JSON shape returned by GET <authRoot>.
type maconomyAuthDiscovery struct {
	Authentication *struct {
		UseDomainCredentialsForBasicAuthentication bool `json:"useDomainCredentialsForBasicAuthentication"`
		Schemes                                    map[string]struct {
			Name string `json:"name"`
		} `json:"schemes"`
		OpenIDProviders []maconomyOIDCProvider `json:"openIDProviders"`
	} `json:"authentication"`
}

// maconomyOIDCProvider holds the per-provider metadata.
type maconomyOIDCProvider struct {
	AuthorizationEndpoint string `json:"authorizationEndpoint"`
	TokenEndpoint         string `json:"tokenEndpoint"`
	RedirectURI           string `json:"redirectURI"`
	ClientID              string `json:"clientID"`
	Links                 *struct {
		AuthorizationURL *struct {
			Template string `json:"template"`
			Rel      string `json:"rel"`
		} `json:"authorization-url"`
		AuthorizationEndpoint *struct {
			HREF string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"authorization-endpoint"`
		RedirectURI *struct {
			HREF string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"redirect-uri"`
	} `json:"links"`
}

// ─── Authentication ────────────────────────────────────────────────────────

// Authenticate is the unified entry point called by the auth handler.
//
// Two distinct invocations:
//
//  1. Initial call – fields contains {baseUrl, company} only.
//     The adapter discovers the OIDC config and returns an *OIDCRequiredError*
//     so the handler can respond with HTTP 202 and the authorization URL.
//
//  2. Code exchange – fields also contains {_oidcCode, _oidcRedirectUri}.
//     The adapter builds the X-OIDC-Code header and exchanges the code with
//     Maconomy for a reconnect token.
func (a *Adapter) Authenticate(ctx context.Context, fields map[string]string) (*models.AuthResult, error) {
	baseURL := a.baseURL
	company := a.company
	if strings.TrimRight(fields["baseUrl"], "/") != "" {
		baseURL = strings.TrimRight(fields["baseUrl"], "/")
	}
	if fields["company"] != "" {
		company = fields["company"]
	}

	// ── Code exchange (second call) ────────────────────────────────────────
	if code := fields["_oidcCode"]; code != "" {
		redirectURI := fields["_oidcRedirectUri"]
		if redirectURI == "" {
			return nil, fmt.Errorf("maconomy: _oidcRedirectUri is required for code exchange")
		}
		return a.exchangeOIDCCode(ctx, baseURL, company, code, redirectURI)
	}

	// ── Discovery (first call) ─────────────────────────────────────────────
	return a.discoverAndInitiateOIDC(ctx, baseURL, company)
}

// discoverAndInitiateOIDC calls the Maconomy auth root, reads the OIDC
// provider metadata, constructs the authorization URL, and returns an
// *OIDCRequiredError so the handler can reply HTTP 202 to the frontend.
func (a *Adapter) discoverAndInitiateOIDC(ctx context.Context, baseURL, company string) (*models.AuthResult, error) {
	authRoot := fmt.Sprintf("%s/%s/auth/%s/", baseURL, a.api_path, company)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authRoot, nil)
	if err != nil {
		return nil, fmt.Errorf("maconomy: build discovery request: %w", err)
	}
	req.Header.Set("Accept", a.accepts["auth"])

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maconomy: discovery request failed – check Maconomy URL: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("maconomy: discovery returned %d – check baseUrl and company: %s",
			resp.StatusCode, string(body))
	}

	var discovery maconomyAuthDiscovery
	if err := json.Unmarshal(body, &discovery); err != nil {
		return nil, fmt.Errorf("maconomy: decode discovery response: %w", err)
	}

	// Confirm OIDC is one of the advertised schemes.
	if _, ok := discovery.Authentication.Schemes["x-oidc-code"]; !ok {
		names := make([]string, 0, len(discovery.Authentication.Schemes))
		for k := range discovery.Authentication.Schemes {
			names = append(names, k)
		}
		return nil, fmt.Errorf(
			"maconomy: instance does not advertise x-oidc-code; available: %s",
			strings.Join(names, ", "))
	}

	if len(discovery.Authentication.OpenIDProviders) == 0 {
		return nil, fmt.Errorf("maconomy: no OpenID providers in discovery response")
	}

	// Use the first provider (Maconomy only ever configures one).
	provider := discovery.Authentication.OpenIDProviders[0]
	// replace discovered values with config values
	if a.client_id != "" {
		provider.ClientID = a.client_id
	}

	if a.return_uri != "" {
		provider.RedirectURI = a.return_uri
	}

	// Generate a cryptographically random state nonce for CSRF protection.
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("maconomy: generate OIDC state: %w", err)
	}

	authURL := buildAuthURL(provider, state, "")

	// Return OIDCRequiredError – the auth handler converts this into HTTP 202.
	return nil, &models.OIDCRequiredError{
		AuthURL:     authURL,
		RedirectURI: "http://localhost/",
		State:       state,
		BaseURL:     baseURL,
		Company:     company,
	}
}

// exchangeOIDCCode constructs the X-OIDC-Code credential and exchanges it
// with Maconomy for a reconnect token.
//
// Wire format (§4.2.3):
//
//	Authorization: X-OIDC-Code <base64("<" + redirectURI + ">:" + code)>
func (a *Adapter) exchangeOIDCCode(ctx context.Context, baseURL, company, code, redirectURI string) (*models.AuthResult, error) {
	authRoot := fmt.Sprintf("%s/%s/auth/%s/", baseURL, a.api_path, company)

	// Encode the credential:  <redirectURI>:code  →  base64
	rawCredential := fmt.Sprintf("<%s>:%s", redirectURI, code)
	encodedCredential := base64.StdEncoding.EncodeToString([]byte(rawCredential))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authRoot, nil)
	if err != nil {
		return nil, fmt.Errorf("maconomy: build code-exchange request: %w", err)
	}
	req.Header.Set("Authorization", "X-OIDC-Code "+encodedCredential)
	req.Header.Set("Accept", a.accepts["auth"])
	req.Header.Set("maconomy-authentication", "X-Reconnect")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("maconomy: code-exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("maconomy: OIDC code rejected – it may have expired or been used already")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("maconomy: code-exchange returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Extract the reconnect token from the response header or body.
	reconnectToken := resp.Header.Get("Maconomy-Reconnect")
	if reconnectToken == "" {
		var bodyMap map[string]interface{}
		if json.Unmarshal(respBody, &bodyMap) == nil {
			if t, ok := bodyMap["reconnectToken"].(string); ok {
				reconnectToken = t
			}
		}
	}
	if reconnectToken == "" {
		return nil, fmt.Errorf("maconomy: no reconnect token in code-exchange response")
	}

	return &models.AuthResult{
		SystemID:  "maconomy",
		TokenType: "reconnect",
		// Reconnect tokens have no explicit expiry; use 8 hours conservatively.
		AccessToken: reconnectToken,
		ExpiresAt:   time.Now().Add(8 * time.Hour),
		Extra: map[string]string{
			"baseUrl": baseURL,
			"company": company,
		},
	}, nil
}

// ─── Token management ──────────────────────────────────────────────────────

// RefreshAuth is not supported: reconnect tokens cannot be refreshed without
// repeating the full OIDC flow.
func (a *Adapter) RefreshAuth(_ context.Context, _ *models.AuthResult) (*models.AuthResult, error) {
	return nil, fmt.Errorf("maconomy: session expired – please log in again")
}

// ValidateAuth checks that the reconnect token is still accepted by Maconomy.
func (a *Adapter) ValidateAuth(ctx context.Context, auth *models.AuthResult) (bool, error) {
	if auth.IsExpired() {
		return false, nil
	}
	baseURL := auth.Extra["baseUrl"]
	company := auth.Extra["company"]
	api := a.api_path
	authRoot := fmt.Sprintf("%s/%s/containers/%s/auth", baseURL, api, company)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authRoot, nil)
	if err != nil {
		return false, err
	}
	a.setAuthHeaders(req, auth)
	req.Header.Set("Accept", a.accepts["auth"])

	resp, err := a.httpClient.Do(req)
	if resp != nil {
		resetToken(resp, auth)
	}
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode < 400, nil
}

// ─── Data retrieval ────────────────────────────────────────────────────────

// maconomyContainerFields
type maconomyContainerFields struct {
	Panes struct {
		Card struct {
			Fields []string `json:"fields"`
		} `json:"card"`
		Table struct {
			Fields []string `json:"fields"`
		} `json:"table"`
	} `json:"panes"`
}

type maconomyEmployee struct {
	User struct {
		EmployeeInfo struct {
			Name1 struct {
				String struct {
					Value string `json:"value"`
				} `json:"string"`
			} `json:"name1"`
		} `json:"employeeinfo"`
		Info struct {
			EmployeeNumber struct {
				String struct {
					Value string `json:"value"`
				} `json:"string"`
			} `json:"employeenumber"`
		} `json:"info"`
	} `json:"user"`
}

type Employee struct {
	EmployeeName   string
	EmployeeNumber string
}

type Favorites struct {
	Panes struct {
		Filter struct {
			Meta struct {
				RowCount int `json:"rowCount"`
			} `json:"meta"`
			Records []struct {
				Data struct {
					JobNumber string `json:"jobnumber"`
					TaskName  string `json:"taskname"`
					Favorite  string `json:"favorite"`
				}
			}
		} `json:"filter"`
	} `json:"panes"`
}

// GetAvailableRows fetches bookable jobs/tasks from the Maconomy timesheet container.
func (a *Adapter) GetAvailableRows(ctx context.Context, auth *models.AuthResult) ([]models.SystemRow, error) {
	employee, err := a.getEmployee(ctx, auth)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return nil, err
	}
	baseURL := a.baseURL
	company := a.company
	api := a.api_path
	url := fmt.Sprintf("%s/%s/containers/%s/timeregistration/search/table;foreignkey=jobfavorite", baseURL, api, company)
	// Do an anonymous struct to setup the body of the request
	data := struct {
		Data struct {
			EmployeeNumber string `json:"employeenumber"`
		} `json:"data"`
		Fields []string `json:"fields"`
	}{
		Data: struct {
			EmployeeNumber string `json:"employeenumber"`
		}{
			EmployeeNumber: employee.EmployeeNumber,
		},
		Fields: []string{"employeenumber", "favorite", "jobnumber", "taskname"},
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("Can't marshal body data for req: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	a.setAuthHeaders(req, auth)
	req.Header.Set("Content-Type", a.contentTypes["container"])
	req.Header.Set("Accept", a.accepts["containersv6.1"])

	resp, err := a.httpClient.Do(req)
	if resp != nil {
		resetToken(resp, auth)
	}
	defer resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("Communication error retreiving favorites: %w", err)
	}
	var favorites Favorites
	err = json.NewDecoder(resp.Body).Decode(&favorites)
	if err != nil {
		return nil, fmt.Errorf("Couldn't decode favorite JSON structure: %w", err)
	}
	rows := make([]models.SystemRow, 0, len(favorites.Panes.Filter.Records))
	for _, rec := range favorites.Panes.Filter.Records {
		d := rec.Data
		rows = append(rows, models.SystemRow{
			ID:          d.JobNumber,
			SystemID:    "maconomy",
			Code:        d.TaskName,
			Name:        d.Favorite,
			Description: d.Favorite,
			Extra:       nil,
		})
	}
	return rows, nil
}

func (a *Adapter) getEmployee(ctx context.Context, auth *models.AuthResult) (*Employee, error) {
	baseURL := a.baseURL
	company := a.company
	api := a.api_path

	url := fmt.Sprintf("%s/%s/environment/%s?variables=user.employeeinfo.name1,user.info.employeenumber", baseURL, api, company)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("Get employee req error: %w", err)
	}
	a.setAuthHeaders(req, auth)
	req.Header.Set("Content-Type", a.contentTypes["container"])
	req.Header.Set("Accept", a.accepts["environment"])
	resp, err := a.httpClient.Do(req)
	if resp != nil {
		resetToken(resp, auth)
	}
	if err != nil {
		return nil, fmt.Errorf("Get employee resp error: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Can't read employee resp: %w", err)
	}
	var macEmployee maconomyEmployee
	if err := json.Unmarshal(body, &macEmployee); err != nil {
		return nil, fmt.Errorf("Wrong employee resp: %w", err)
	}
	return &Employee{
		EmployeeName:   macEmployee.User.EmployeeInfo.Name1.String.Value,
		EmployeeNumber: macEmployee.User.Info.EmployeeNumber.String.Value,
	}, nil
}

// ─── Submission ────────────────────────────────────────────────────────────

type maconomyTimesheetPost struct {
	Data struct {
		JobNumber    string  `json:"jobNumber"`
		ActivityCode string  `json:"activityCode,omitempty"`
		Monday       float64 `json:"numberOf1,omitempty"`
		Tuesday      float64 `json:"numberOf2,omitempty"`
		Wednesday    float64 `json:"numberOf3,omitempty"`
		Thursday     float64 `json:"numberOf4,omitempty"`
		Friday       float64 `json:"numberOf5,omitempty"`
		Saturday     float64 `json:"numberOf6,omitempty"`
		Sunday       float64 `json:"numberOf7,omitempty"`
	} `json:"data"`
}

// SubmitEntries posts time entries to the Maconomy timesheet container.
func (a *Adapter) SubmitEntries(ctx context.Context, auth *models.AuthResult, entries []models.SystemTimeEntry) (*models.SubmitResult, error) {
	baseURL := auth.Extra["baseUrl"]
	company := auth.Extra["company"]
	result := &models.SubmitResult{SystemID: "maconomy", Success: true}

	for _, entry := range entries {
		weekKey := entry.WeekStart.Format("2006-01-02")
		url := fmt.Sprintf("%s/containers/%s/timesheets/%s/table", baseURL, company, weekKey)

		var payload maconomyTimesheetPost
		payload.Data.JobNumber = entry.RowCode
		if parts := strings.SplitN(entry.RowID, ":", 2); len(parts) == 2 {
			payload.Data.ActivityCode = parts[1]
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
		a.setAuthHeaders(req, auth) //TODO: check if it is in deed `container` here
		req.Header.Set("Accept", a.accepts["container"])
		req.Header.Set("Content-Type", a.accepts["container"])

		resp, err := a.httpClient.Do(req)
		if resp != nil {
			resetToken(resp, auth)
		}
		if err != nil {
			result.Success = false
			result.Details = append(result.Details, fmt.Sprintf("submit %s: %v", entry.RowCode, err))
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			result.Success = false
			result.Details = append(result.Details,
				fmt.Sprintf("submit %s status %d: %s", entry.RowCode, resp.StatusCode, respBody))
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

// ─── Internal helpers ──────────────────────────────────────────────────────

func getDef(input, def string) string {
	if input != "" {
		return input
	} else {
		return def
	}
}

func (a *Adapter) setAuthHeaders(req *http.Request, auth *models.AuthResult) {
	req.Header.Set("Authorization", "X-Reconnect "+auth.AccessToken)
	req.Header.Set("maconomy-authentication", "X-Reconnect")
}

func resetToken(resp *http.Response, auth *models.AuthResult) {
	if resp.Header.Get("maconomy-reconnect") != "" {
		auth.AccessToken = resp.Header.Get("maconomy-reconnect")
	}
}

// buildAuthURL constructs the authorization URL.
func buildAuthURL(provider maconomyOIDCProvider, state string, return_uri string) string {
	if return_uri == "" && provider.RedirectURI != "" {
		return_uri = provider.RedirectURI
	}
	if provider.Links.AuthorizationURL != nil && provider.Links.AuthorizationURL.Template != "" {
		u := strings.ReplaceAll(
			provider.Links.AuthorizationURL.Template,
			"{redirect-uri}",
			return_uri,
		)
		sep := "&"
		if !strings.Contains(u, "?") {
			sep = "?"
		}
		return u + sep + "state=" + state
	}
	// Fallback: build manually from endpoint parts.
	return fmt.Sprintf("%s?client_id=%s&scope=openid&response_type=code&redirect_uri=%s&state=%s",
		provider.AuthorizationEndpoint, provider.ClientID, provider.RedirectURI, state)
}

// generateState produces a 32-character cryptographically random hex string
// used as the OIDC state parameter for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// IsOIDCRequiredError returns the typed error if err is an *OIDCRequiredError.
// Convenience wrapper for callers outside the models package.
func IsOIDCRequiredError(err error) (*models.OIDCRequiredError, bool) {
	var oidcErr *models.OIDCRequiredError
	if errors.As(err, &oidcErr) {
		return oidcErr, true
	}
	return nil, false
}
