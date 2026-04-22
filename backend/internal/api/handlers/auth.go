package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
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

func (h *AuthHandler) HandleAuth(c *gin.Context) {
	var fields map[string]string
	c.Bind(&fields)

	adapter, ok := h.registry.Get(fields["adapterId"])
	if !ok {
		c.AbortWithError(http.StatusBadRequest, errors.New("No adapter found"))
		log.Print(fields)
		return
	}

	authID, err := adapter.Authenticate(c, fields)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"authId": authID})
}

func (h *AuthHandler) HandleLogout(c *gin.Context) {
}
