package handlers

import (
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication operations for a specific system.

type AuthHandler struct {
	registry *adapters.Registry
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(registry *adapters.Registry) *AuthHandler {
	return &AuthHandler{registry: registry}
}

func (h *AuthHandler) HandleStatus(c *gin.Context) {
}

func (h *AuthHandler) HandleLogout(c *gin.Context) {
}

// HandleAuthenticate covers both the initial credential POST and the OIDC
// code-exchange POST.  The distinction is made by the presence of _oidcCode
// in the request body.
func (h *AuthHandler) HandleAuthenticate(c *gin.Context) {
}

// storeAuthAndRespond saves the AuthResult in the session and returns a
// sanitised AuthStatus to the caller (no tokens).
func (h *AuthHandler) storeAuthAndRespond(
	w http.ResponseWriter,
	systemID string,
	auth *models.Auth,
) {
}
