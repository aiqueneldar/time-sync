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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aiqueneldar/time-sync/backend/internal/config"
	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/aiqueneldar/time-sync/backend/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Adapter implements adapters.Adapter for Deltek Maconomy using OIDC auth.
type Adapter struct {
	httpClient   *http.Client
	accepts      map[string]string
	contentTypes map[string]string
}

// New creates a Maconomy adapter.
// The baseURL and company parameters are accepted for interface compatibility
// but users supply them per-session via the UI auth fields.
func New(cfg *config.Config) *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		accepts: map[string]string{
			"auth":           "application/vnd.deltek.maconomy.authentication+json; charset=utf-8; version=2.0;",
			"environment":    "application/vnd.deltek.maconomy.environment+json; charset=utf-8; version=2.0",
			"containers":     "application/vnd.deltek.maconomy.containers-v2+json",
			"containersv6.1": "application/vnd.deltek.maconomy.containers+json; charset=utf-8; version=6.1",
			"root":           "application/vnd.deltek.maconomy.root-v1+json",
		},
		contentTypes: map[string]string{
			"container": "application/vnd.deltek.maconomy.containers+json",
		},
	}
}

// ─── Identity ──────────────────────────────────────────────────────────────

func (a *Adapter) SystemID() string    { return "maconomy" }
func (a *Adapter) SystemName() string  { return "Deltek Maconomy" }
func (a *Adapter) Description() string { return "Deltek Maconomy ERP" }

func (a *Adapter) Authenticate(c *gin.Context, fields map[string]string) (uuid.UUID, error) {
	// Start to construct Auth object
	auth := models.Auth{
		TokenType: "reconnect",
		ExpiresAt: time.Now().Add(time.Hour * 1),
		Extra: map[string]string{
			"baseURL": fields["baseURL"],
			"apiURL":  fields["apiURL"],
			"company": fields["company"],
		},
	}
	// Get the current user session from the in-memory store, needs some type conversions
	sessionID, err := c.Cookie("sessionId")
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	store := store.GetStore()
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	session := store.GetSession(sessionUUID)

	switch fields["authType"] {
	case "token":
		auth.AccessToken = fields["token"]
	case "password":
		user := fields["username"]
		password := fields["password"]
		token, err := a.passwordAuth(c.Request.Context(), user, password, fields)
		if err != nil {
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		auth.AccessToken = token
	default:
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("No acceptable authType found"))
	}

	return session.NewAuth(auth), nil
}

func (a *Adapter) passwordAuth(ctx context.Context, user string, password string, fields map[string]string) (string, error) {
	url := fmt.Sprintf("%s/%s/auth/%s/", fields["baseURL"], fields["apiURL"], fields["company"]) // Build the URL to the Auth endpoint of Maconomy
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(user, password)                         // Maconomy specefies this as 'user:pass', in utf-8 and base64 encoded with Basic prepended.
	req.Header.Set("Maconomy-Authentication", "X-Reconnect") // Maconomys propreartery header for HTTP Reconnect
	req.Header.Set("Accept", a.accepts["auth"])
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	if token := resp.Header.Get("maconomy-reconnect"); token != "" {
		return token, nil
	}

	return "", fmt.Errorf("No Maconomy-Reconnect header returned")
}

// ─── Token management ──────────────────────────────────────────────────────

// ValidateAuth checks that the reconnect token is still accepted by Maconomy.
func (a *Adapter) ValidateAuth(c *gin.Context, authID uuid.UUID) (bool, error) {
	auth, err := getAuthFromSession(c, authID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("No auth session found"))
	}

	url := fmt.Sprintf("%s/%s/containers/%s/", auth.Extra["baseURL"], auth.Extra["apiURL"], auth.Extra["company"])
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
	}
	a.setAuthHeaders(req, auth)
	req.Header.Set("Accept", a.accepts["containers"])
	resp, err := a.httpClient.Do(req)
	if err != nil {
		c.AbortWithError(http.StatusUnauthorized, err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.AbortWithError(http.StatusUnauthorized, err)
	}
	if len(body) > 0 && resp.Header.Get("maconomy-reconnect") != "" {
		resetToken(resp, auth)
		return true, nil
	}

	return false, errors.New("No body or no Reconnect token received")
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

type maconomyFavorites struct {
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
func (a *Adapter) GetAvailableRows(c *gin.Context, authID uuid.UUID) ([]models.SystemRow, error) {
	auth, err := getAuthFromSession(c, authID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, errors.New("No auth session found"))
	}
	employee, err := a.getEmployee(c, auth)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return nil, err
	}
	url := fmt.Sprintf("%s/%s/containers/%s/timeregistration/search/table;foreignkey=jobfavorite", auth.Extra["baseURL"], auth.Extra["apiURL"], auth.Extra["company"])
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
	req, err := http.NewRequestWithContext(c, http.MethodPost, url, bytes.NewReader(body))
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
	var favorites maconomyFavorites
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

func (a *Adapter) getEmployee(c *gin.Context, auth *models.Auth) (*Employee, error) {
	url := fmt.Sprintf("%s/%s/environment/%s?variables=user.employeeinfo.name1,user.info.employeenumber", auth.Extra["baseURL"], auth.Extra["apiURL"], auth.Extra["company"])
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
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
func (a *Adapter) SubmitEntries(ctx context.Context, auth *models.Auth, entries []models.SystemTimeEntry) (*models.SubmitResult, error) {
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

func (a *Adapter) setAuthHeaders(req *http.Request, auth *models.Auth) {
	req.Header.Set("Authorization", "X-Reconnect "+auth.AccessToken)
	req.Header.Set("maconomy-authentication", "X-Reconnect")
}

func resetToken(resp *http.Response, auth *models.Auth) {
	if resp.Header.Get("maconomy-reconnect") != "" {
		auth.AccessToken = resp.Header.Get("maconomy-reconnect")
	}
}

func getAuthFromSession(c *gin.Context, authID uuid.UUID) (*models.Auth, error) {
	sessionID, err := c.Cookie("sessionId")
	if err != nil {
		return nil, err
	}
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, err
	}
	session := store.GetStore().GetSession(sessionUUID)
	auth, ok := session.GetAuth(authID)
	if !ok {
		return nil, errors.New("No auth session found")
	}
	return auth, nil
}
