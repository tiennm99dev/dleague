package ws

import "sync"

// Queue is an in-memory FIFO matchmaking queue keyed by game ID.
// When two players are waiting for the same game, PopPair returns them.
// Single-region MVP; Redis pub/sub deferred to v2.
// All methods are goroutine-safe.
type Queue struct {
	mu      sync.Mutex
	entries map[string][]*Conn // gameID → ordered list of waiting conns
}

// NewQueue creates an empty Queue.
func NewQueue() *Queue {
	return &Queue{entries: make(map[string][]*Conn)}
}

// Push adds conn to the tail of the queue for gameID.
func (q *Queue) Push(gameID string, c *Conn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries[gameID] = append(q.entries[gameID], c)
}

// PopPair removes and returns the first two waiting conns for gameID.
// Returns ok=false when fewer than two players are queued.
// Caller must hold no locks; this method acquires its own.
func (q *Queue) PopPair(gameID string) (a, b *Conn, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.entries[gameID]) < 2 {
		return nil, nil, false
	}
	list := q.entries[gameID]
	a, b = list[0], list[1]
	q.entries[gameID] = list[2:]
	return a, b, true
}

// Remove removes conn from all game queues.
// O(n) per game; acceptable for MVP queue sizes.
func (q *Queue) Remove(c *Conn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for gameID, list := range q.entries {
		filtered := list[:0]
		for _, entry := range list {
			if entry != c {
				filtered = append(filtered, entry)
			}
		}
		if len(filtered) == 0 {
			delete(q.entries, gameID)
		} else {
			q.entries[gameID] = filtered
		}
	}
}
