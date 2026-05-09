package ws

import (
	"sync"
	"time"
)

// queueEntry pairs a connection with its enqueue timestamp for TTL eviction.
type queueEntry struct {
	conn       *Conn
	enqueuedAt time.Time
}

// Queue is an in-memory FIFO matchmaking queue keyed by game ID.
// When two players are waiting for the same game, PopPair returns them.
// Single-region MVP; Redis pub/sub deferred to v2.
// All methods are goroutine-safe.
type Queue struct {
	mu      sync.Mutex
	entries map[string][]*queueEntry // gameID → ordered list of waiting entries
}

// queueTTL is the maximum time a connection may sit in the queue before being
// evicted with an error frame. Phase 09 M6 fix.
const queueTTL = 60 * time.Second

// NewQueue creates an empty Queue.
func NewQueue() *Queue {
	return &Queue{entries: make(map[string][]*queueEntry)}
}

// Push adds conn to the tail of the queue for gameID.
func (q *Queue) Push(gameID string, c *Conn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries[gameID] = append(q.entries[gameID], &queueEntry{conn: c, enqueuedAt: time.Now()})
}

// PopPair removes and returns the first two waiting conns for gameID.
// Returns ok=false when fewer than two players are queued.
// Self-pairs (same userID) are rejected — Phase 09 M2 fix.
// Caller must hold no locks; this method acquires its own.
func (q *Queue) PopPair(gameID string) (a, b *Conn, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	list := q.entries[gameID]
	if len(list) < 2 {
		return nil, nil, false
	}
	// Reject self-pair: if both front entries belong to the same user, leave
	// them in the queue — a different opponent must join first.
	if list[0].conn.userID != "" && list[0].conn.userID == list[1].conn.userID {
		return nil, nil, false
	}
	a, b = list[0].conn, list[1].conn
	q.entries[gameID] = list[2:]
	return a, b, true
}

// EvictExpired removes queue entries older than queueTTL, calling notify for
// each evicted connection so an ERROR frame can be sent to the client.
// Intended to be called periodically (e.g. every 5 seconds) from a background
// goroutine. Phase 09 M6 fix.
func (q *Queue) EvictExpired(notify func(c *Conn)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	deadline := time.Now().Add(-queueTTL)
	for gameID, list := range q.entries {
		kept := list[:0]
		for _, e := range list {
			if e.enqueuedAt.Before(deadline) {
				if notify != nil {
					notify(e.conn)
				}
			} else {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(q.entries, gameID)
		} else {
			q.entries[gameID] = kept
		}
	}
}

// Remove removes conn from all game queues.
// O(n) per game; acceptable for MVP queue sizes.
func (q *Queue) Remove(c *Conn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for gameID, list := range q.entries {
		filtered := list[:0]
		for _, e := range list {
			if e.conn != c {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(q.entries, gameID)
		} else {
			q.entries[gameID] = filtered
		}
	}
}
