package ws

import (
	"sync"
	"time"
)

// Room is one sync-PvP match. Two conns max in v1 (1v1).
type Room struct {
	matchID   string
	mu        sync.Mutex
	conns     []*Conn
	createdAt time.Time
}

func newRoom(matchID string) *Room {
	return &Room{matchID: matchID, createdAt: time.Now()}
}

// add returns true if the conn was newly added; false if already present.
func (r *Room) add(c *Conn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.conns {
		if e == c {
			return false
		}
	}
	r.conns = append(r.conns, c)
	return true
}

func (r *Room) remove(c *Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.conns {
		if e == c {
			r.conns = append(r.conns[:i], r.conns[i+1:]...)
			return
		}
	}
}

// snapshot returns a copy of the conn slice safe for iteration without the
// room mutex held.
func (r *Room) snapshot() []*Conn {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Conn, len(r.conns))
	copy(out, r.conns)
	return out
}

func (r *Room) empty() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.conns) == 0
}
