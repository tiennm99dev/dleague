// Package ws is the WebSocket hub for dleague.
//
// All gameplay messages travel through one /ws connection per client. The hub
// owns connection lifecycle (register/unregister) and dispatches incoming
// Envelope messages by MessageType.
package ws

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/tiennm99/dleague/server/internal/auth"
	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// ErrAtCapacity is returned by register when the hub has reached MaxConns.
var ErrAtCapacity = errors.New("ws: at connection capacity")

// Hub tracks active WebSocket connections.
type Hub struct {
	mu    sync.RWMutex
	conns map[*Conn]struct{}

	// MaxConns is the maximum number of concurrent connections.
	// Zero means unlimited (for tests / dev convenience).
	MaxConns int

	// verifier verifies Firebase ID tokens. May be nil in tests.
	verifier *auth.Verifier

	// userRepo persists user profiles on first auth. May be nil in tests.
	userRepo *store.UserRepo

	// GameDeps holds game handler dependencies (word lists, daily repo).
	// May be nil when running without a database (dev / unit tests).
	GameDeps *GameDeps
}

// NewHub creates a Hub with the given verifier and user repo.
// Both may be nil — tests that do not exercise auth paths pass nil for both.
// The sync PvP in-memory state (Queue, Rooms, GraceTimers) is initialised here.
func NewHub(verifier *auth.Verifier, userRepo *store.UserRepo) *Hub {
	return &Hub{
		conns:    map[*Conn]struct{}{},
		verifier: verifier,
		userRepo: userRepo,
	}
}

// CloseAll sends a 503 error to every connected client and closes all
// connections. Called from main on SIGTERM before http.Server.Shutdown so
// active WS clients receive a clean error frame before the socket is torn down.
func (h *Hub) CloseAll(reason string) {
	h.mu.RLock()
	conns := make([]*Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		c.enqueue(errorEnvelope("", 503, reason))
		c.cancelRead()
	}
}

// register adds conn to the hub. Returns ErrAtCapacity if MaxConns > 0 and the
// limit has already been reached.
func (h *Hub) register(c *Conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.MaxConns > 0 && len(h.conns) >= h.MaxConns {
		return ErrAtCapacity
	}
	h.conns[c] = struct{}{}
	return nil
}

func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
}

// Count returns the active connection count (diagnostic).
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// dispatch routes one inbound Envelope. Returns the response Envelope to send,
// or nil if the message produces no response.
//
// c is the originating connection; handlers may read c.userID and other auth
// fields. The auth gate is enforced here before any handler is called.
func (h *Hub) dispatch(ctx context.Context, env *dleaguev1.Envelope, c *Conn, serverNowMS int64) (*dleaguev1.Envelope, error) {
	// Auth gate: reject messages that require authentication when the connection
	// has no verified user identity.
	if requiresAuth(env.GetType()) && c.UserID() == "" {
		return errorEnvelope(env.GetRequestId(), 401, "unauthenticated"), nil
	}

	switch env.GetType() {
	case dleaguev1.MessageType_MESSAGE_TYPE_PING:
		return handlePing(env, serverNowMS)
	case dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REFRESH:
		return handleAuthRefresh(ctx, c, env)
	case dleaguev1.MessageType_MESSAGE_TYPE_WORDLE_MOVE:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleGameMove(ctx, c, env, h.GameDeps)

	// Phase 08: async PvP + leaderboard.
	case dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_CREATE:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleChallengeCreate(ctx, c, env, h.GameDeps)
	case dleaguev1.MessageType_MESSAGE_TYPE_CHALLENGE_JOIN:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleChallengeJoin(ctx, c, env, h.GameDeps)
	case dleaguev1.MessageType_MESSAGE_TYPE_ATTEMPT_SUBMIT:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleAttemptSubmit(ctx, c, env, h.GameDeps)
	case dleaguev1.MessageType_MESSAGE_TYPE_LEADERBOARD_QUERY:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleLeaderboardQuery(ctx, c, env, h.GameDeps)

	// Phase 09: sync PvP.
	case dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_JOIN:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleQueueJoin(ctx, c, env, h.GameDeps)
	case dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_LEAVE:
		if h.GameDeps == nil {
			return nil, nil
		}
		return handleQueueLeave(ctx, c, env, h.GameDeps)
	case dleaguev1.MessageType_MESSAGE_TYPE_MATCH_MOVE:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleMatchMove(ctx, c, env, h.GameDeps)
	case dleaguev1.MessageType_MESSAGE_TYPE_MATCH_REJOIN:
		if h.GameDeps == nil {
			return errorEnvelope(env.GetRequestId(), 503, "game service unavailable"), nil
		}
		return handleMatchRejoin(ctx, c, env, h.GameDeps)

	default:
		log.Printf("ws dispatch: unhandled type=%v request_id=%q", env.GetType(), env.GetRequestId())
		return nil, nil
	}
}
