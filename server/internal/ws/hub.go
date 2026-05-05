// Package ws is the WebSocket hub for dleague.
//
// All gameplay messages travel through one /ws connection per client. The hub
// owns connection lifecycle (register/unregister) and dispatches incoming
// Envelope messages by MessageType. At Phase 1 only Ping is implemented.
package ws

import (
	"log"
	"sync"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// Hub tracks active WebSocket connections.
type Hub struct {
	mu    sync.RWMutex
	conns map[*Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{conns: map[*Conn]struct{}{}}
}

func (h *Hub) register(c *Conn) {
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
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
func (h *Hub) dispatch(env *dleaguev1.Envelope, serverNowMS int64) (*dleaguev1.Envelope, error) {
	switch env.GetType() {
	case dleaguev1.MessageType_MESSAGE_TYPE_PING:
		return handlePing(env, serverNowMS)
	default:
		log.Printf("ws dispatch: unhandled type=%v request_id=%q", env.GetType(), env.GetRequestId())
		return nil, nil
	}
}
