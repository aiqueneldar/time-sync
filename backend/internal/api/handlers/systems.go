// Package handlers contains all HTTP handler functions for the TimeSync API.
package handlers

import (
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/gin-gonic/gin"
)

// SystemsHandler handles GET /api/systems.
// It returns the list of all registered time-reporting systems with their
// authentication field definitions so the frontend can render login forms.
type SystemsHandler struct {
	registry *adapters.Registry
}

// NewSystemsHandler creates a SystemsHandler.
func NewSystemsHandler(registry *adapters.Registry) *SystemsHandler {
	return &SystemsHandler{registry: registry}
}

func (h *SystemsHandler) Handler(c *gin.Context) {
	systems := h.registry.All()
	c.JSON(http.StatusOK, gin.H{"systems": systems})
}
