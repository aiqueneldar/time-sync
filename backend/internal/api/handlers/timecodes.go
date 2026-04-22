package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/aiqueneldar/time-sync/backend/internal/adapters"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TimecodesHandler struct {
	registry *adapters.Registry
}

// NewTimesheetsHandler creates a TimesheetsHandler.
func NewTimecodesHandler(registry *adapters.Registry) *TimecodesHandler {
	return &TimecodesHandler{registry: registry}
}

func (h *TimecodesHandler) GetTimecodes(c *gin.Context) {
	sessID, err := GetSessionUUID(c)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	sess := GetSession(sessID)

	authID := c.Params.ByName("authId")
	authUUID, err := uuid.Parse(authID)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	auth, ok := sess.GetAuth(authUUID)
	if !ok {
		c.AbortWithError(http.StatusBadRequest, errors.New(fmt.Sprintf("Could not fins Auth instance with UUID: %s", authUUID)))
		return
	}
	adapter, ok := h.registry.Get(auth.AdapterID)
	if !ok {
		c.AbortWithError(http.StatusInternalServerError, errors.New(fmt.Sprintf("Could not find adapter with ID %s", auth.AdapterID)))
		return
	}
	codes, err := adapter.GetAvailableRows(c, authUUID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, []string{})
		return
	}
	c.JSON(http.StatusOK, codes)
}
