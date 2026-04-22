package handlers

import (
	"github.com/aiqueneldar/time-sync/backend/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetSessionUUID(c *gin.Context) (uuid.UUID, error) {
	sessionID, err := c.Cookie("X-Session-ID")
	if err != nil {
		return uuid.New(), err
	}
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return uuid.New(), err
	}
	return sessionUUID, nil
}

func GetSession(sessionUUID uuid.UUID) *store.Session {
	return store.GetStore().GetSession(sessionUUID)
}
