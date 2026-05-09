package ws

import (
	"context"
	"log"
	"sync"
	"time"
)

const disconnectGracePeriod = 30 * time.Second

// GraceTimers tracks outstanding disconnect grace timers keyed by matchID+userID.
// When a player reconnects and sends MATCH_REJOIN the timer is cancelled.
type GraceTimers struct {
	mu     sync.Mutex
	timers map[string]*time.Timer // key: matchID+":"+userID
}

// NewGraceTimers returns an empty GraceTimers.
func NewGraceTimers() *GraceTimers {
	return &GraceTimers{timers: make(map[string]*time.Timer)}
}

// timerKey returns the canonical map key for a match+player pair.
func timerKey(matchID, userID string) string {
	return matchID + ":" + userID
}

// Schedule starts a 30-second grace timer for the disconnecting player c.
// When the timer fires without being cancelled, the room's HandleForfeit is
// called and the room is resolved.
func (g *GraceTimers) Schedule(c *Conn, deps *GameDeps) {
	matchID := c.getActiveMatchID()
	if matchID == "" {
		return
	}
	key := timerKey(matchID, c.UserID())

	g.mu.Lock()
	// Cancel any prior timer for this player (idempotent re-schedule).
	if old, ok := g.timers[key]; ok {
		old.Stop()
	}
	t := time.AfterFunc(disconnectGracePeriod, func() {
		g.mu.Lock()
		delete(g.timers, key)
		g.mu.Unlock()

		if deps == nil || deps.Rooms == nil {
			return
		}
		room := deps.Rooms.Get(matchID)
		if room == nil {
			return // match already resolved
		}
		uid := c.UserID()
		log.Printf("ws disconnect: grace expired matchID=%q loser=%s → forfeit", matchID, RedactUID(uid))
		room.HandleForfeit(context.Background(), uid, deps)
	})
	g.timers[key] = t
	g.mu.Unlock()
}

// Cancel stops the grace timer for matchID+userID (called on MATCH_REJOIN).
// No-op if no timer is registered.
func (g *GraceTimers) Cancel(matchID, userID string) {
	key := timerKey(matchID, userID)
	g.mu.Lock()
	if t, ok := g.timers[key]; ok {
		t.Stop()
		delete(g.timers, key)
	}
	g.mu.Unlock()
}
