package ws

import "sync"

// RoomsRegistry is a concurrent map of active sync match rooms keyed by matchID.
type RoomsRegistry struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

// NewRoomsRegistry creates an empty registry.
func NewRoomsRegistry() *RoomsRegistry {
	return &RoomsRegistry{rooms: make(map[string]*Room)}
}

// Add registers a room. Overwrites any existing room with the same matchID.
func (r *RoomsRegistry) Add(matchID string, room *Room) {
	r.mu.Lock()
	r.rooms[matchID] = room
	r.mu.Unlock()
}

// Get returns the room for matchID, or nil if not found.
func (r *RoomsRegistry) Get(matchID string) *Room {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rooms[matchID]
}

// Remove deletes the room for matchID (no-op if not found).
func (r *RoomsRegistry) Remove(matchID string) {
	r.mu.Lock()
	delete(r.rooms, matchID)
	r.mu.Unlock()
}

// All returns a snapshot slice of all active rooms (safe for ranging outside lock).
func (r *RoomsRegistry) All() []*Room {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Room, 0, len(r.rooms))
	for _, room := range r.rooms {
		out = append(out, room)
	}
	return out
}
