package store

import (
	"github.com/aiqueneldar/time-sync/backend/internal/models"
	"github.com/google/uuid"
)

var store Store

type Store map[uuid.UUID]Session

type Session map[uuid.UUID]models.Auth

func GetStore() *Store {
	if store != nil {
		return &store
	}
	store = Store{}
	return &store
}

func (s Store) GetSession(sessionID uuid.UUID) *Session {
	var session Session
	if _, ok := s[sessionID]; !ok {
		store[sessionID] = Session{}
		session = s[sessionID]
	} else {
		session = s[sessionID]
	}
	return &session
}

func (se Session) NewAuth(auth models.Auth) uuid.UUID {
	authID := uuid.New()
	se[authID] = auth
	return authID
}

func (se Session) GetAuth(authID uuid.UUID) (*models.Auth, bool) {
	if auth, ok := se[authID]; ok {
		return &auth, true
	} else {
		return nil, false
	}
}
