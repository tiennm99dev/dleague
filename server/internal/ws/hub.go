// Package ws is the WebSocket hub for dleague.
//
// All gameplay messages travel through one /ws connection per client. The hub
// owns connection lifecycle (register/unregister), the room registry for
// sync-PvP, and dispatches incoming Envelope messages by MessageType.
package ws

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"

	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// Hub tracks active connections and rooms. Construct with NewHub for the
// stateless build (e.g. ping-pong only) or NewHubWithStore once a real
// store.Store is available — sync-PvP routes need the store for
// match lookup, persistence, and leaderboard updates.
type Hub struct {
	mu    sync.RWMutex
	conns map[*Conn]struct{}
	rooms map[string]*Room // matchID → room
	store store.Store      // optional; nil disables sync-PvP
}

func NewHub() *Hub {
	return &Hub{
		conns: map[*Conn]struct{}{},
		rooms: map[string]*Room{},
	}
}

func NewHubWithStore(s store.Store) *Hub {
	h := NewHub()
	h.store = s
	return h
}

func (h *Hub) register(c *Conn) {
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	delete(h.conns, c)
	if c.matchID != "" {
		if room, ok := h.rooms[c.matchID]; ok {
			room.remove(c)
			if room.empty() {
				delete(h.rooms, c.matchID)
			}
		}
	}
	h.mu.Unlock()
}

// Count returns the active connection count (diagnostic).
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// RoomCount returns the active room count (diagnostic).
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// dispatch routes one inbound Envelope. Returns the response Envelope to
// send back to `c`, or nil if the message produces no direct response (TURN
// fans out via hub.broadcastRoom, not the return value).
func (h *Hub) dispatch(c *Conn, env *dleaguev1.Envelope, serverNowMS int64) (*dleaguev1.Envelope, error) {
	switch env.GetType() {
	case dleaguev1.MessageType_MESSAGE_TYPE_PING:
		return handlePing(env, serverNowMS)
	case dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM:
		return h.handleJoinRoom(c, env)
	case dleaguev1.MessageType_MESSAGE_TYPE_TURN:
		return nil, h.handleTurn(c, env)
	case dleaguev1.MessageType_MESSAGE_TYPE_MATCH_END:
		return h.handleMatchEnd(c, env)
	default:
		log.Printf("ws dispatch: unhandled type=%v request_id=%q", env.GetType(), env.GetRequestId())
		return nil, nil
	}
}

// broadcastRoom writes env to every connection in the room except `exclude`.
// Errors per-conn are logged but don't abort the fan-out.
func (h *Hub) broadcastRoom(matchID string, env *dleaguev1.Envelope, exclude *Conn) {
	h.mu.RLock()
	room, ok := h.rooms[matchID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	out, err := proto.Marshal(env)
	if err != nil {
		log.Printf("ws broadcast marshal: %v", err)
		return
	}

	for _, peer := range room.snapshot() {
		if peer == exclude {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		if err := peer.ws.Write(ctx, websocket.MessageBinary, out); err != nil {
			log.Printf("ws broadcast peer write: %v", err)
		}
		cancel()
	}
}

// stale-room reaper threshold; left as a constant for future use.
const _roomStaleAfter = 5 * time.Minute
