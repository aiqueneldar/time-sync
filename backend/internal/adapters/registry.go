package adapters

import (
	"fmt"
	"sync"

	"github.com/aiqueneldar/time-sync/backend/internal/models"
)

// Registry holds all registered Adapter implementations.
// It is safe for concurrent read access after initial registration.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]Adapter),
	}
}

// Register adds an adapter to the registry. It panics on duplicate IDs so that
// programming errors are caught at startup rather than silently ignored.
func (r *Registry) Register(a Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := a.SystemID()
	if _, exists := r.adapters[id]; exists {
		panic(fmt.Sprintf("adapter registry: duplicate system ID %q", id))
	}
	r.adapters[id] = a
}

// Get returns the adapter for the given system ID.
// Returns nil, false if no adapter is registered for that ID.
func (r *Registry) Get(systemID string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[systemID]
	return a, ok
}

// All returns metadata for every registered adapter, suitable for the
// GET /api/systems response. Adapters are returned in registration order
// is not guaranteed – the frontend should sort if needed.
func (r *Registry) All() []models.SystemInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]models.SystemInfo, 0, len(r.adapters))
	for _, a := range r.adapters {
		infos = append(infos, models.SystemInfo{
			ID:          a.SystemID(),
			Name:        a.SystemName(),
			Description: a.Description(),
			AuthFields:  a.AuthFields(),
		})
	}
	return infos
}

// IDs returns the set of registered system IDs.
func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.adapters))
	for id := range r.adapters {
		ids = append(ids, id)
	}
	return ids
}
