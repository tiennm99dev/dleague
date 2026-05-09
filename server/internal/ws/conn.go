package ws

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"
	"github.com/coder/websocket"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

const (
	readLimit   = 1 << 20 // 1 MiB
	sendBufSize = 64      // bounded send channel capacity
	maxReqIDLen = 128     // request_id byte length cap
)

// Conn is one client WebSocket connection. The hub owns its lifecycle.
type Conn struct {
	ws         *websocket.Conn
	hub        *Hub
	send       chan []byte        // outbound frames; cap sendBufSize
	cancelRead context.CancelFunc // cancels the readLoop context
}

// UpgradeOptions controls WebSocket Accept behaviour. Zero value enforces the
// coder/websocket default same-origin policy. To allow cross-origin clients, populate
// AllowedOrigins (host:port matched case-insensitively).
type UpgradeOptions struct {
	AllowedOrigins []string
}

// UpgradeHandler returns an http.HandlerFunc that upgrades to WebSocket and
// drives the read/write loops until the client disconnects or an error occurs.
func UpgradeHandler(hub *Hub, opts UpgradeOptions) http.HandlerFunc {
	accept := websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
		OriginPatterns:  opts.AllowedOrigins,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Pre-accept cap check: avoids wasting the upgrade handshake.
		if hub.MaxConns > 0 {
			hub.mu.RLock()
			count := len(hub.conns)
			hub.mu.RUnlock()
			if count >= hub.MaxConns {
				http.Error(w, "too many connections", http.StatusTooManyRequests)
				return
			}
		}

		c, err := websocket.Accept(w, r, &accept)
		if err != nil {
			log.Printf("ws accept: %v", err)
			return
		}
		c.SetReadLimit(readLimit)

		readCtx, cancelRead := context.WithCancel(r.Context())
		conn := &Conn{
			ws:         c,
			hub:        hub,
			send:       make(chan []byte, sendBufSize),
			cancelRead: cancelRead,
		}

		if err := hub.register(conn); err != nil {
			// Race between pre-check and accept: reject gracefully.
			cancelRead()
			_ = c.Close(websocket.StatusTryAgainLater, "at capacity")
			return
		}
		defer hub.unregister(conn)
		defer func() { _ = c.CloseNow() }()

		// writeLoop runs in a separate goroutine; it owns all ws.Write calls and
		// the ping ticker. When it exits it cancels the readLoop context.
		writeDone := make(chan struct{})
		go func() {
			defer close(writeDone)
			conn.writeLoop(readCtx, cancelRead)
		}()

		conn.readLoop(readCtx)
		cancelRead() // ensure writeLoop exits if readLoop returns first

		<-writeDone // wait for writeLoop before CloseNow runs
	}
}

// readLoop reads inbound frames until the context is cancelled or an error occurs.
// It no longer writes to the WebSocket directly; responses are enqueued to send.
func (c *Conn) readLoop(ctx context.Context) {
	for {
		mt, data, err := c.ws.Read(ctx)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("ws read: %v", err)
			}
			return
		}
		if mt != websocket.MessageBinary {
			log.Printf("ws non-binary frame discarded (type=%d)", mt)
			continue
		}
		c.handleFrame(data)
	}
}

// handleFrame processes one inbound binary frame. It enqueues any response to
// c.send; on send-channel overflow it cancels the connection.
func (c *Conn) handleFrame(data []byte) {
	var env dleaguev1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		log.Printf("ws unmarshal: %v", err)
		c.enqueue(errorEnvelope("", 400, "invalid envelope"))
		return
	}
	logRecv(&env)

	// request_id length cap: prevents log injection / oversized reflected IDs.
	if len(env.GetRequestId()) > maxReqIDLen {
		c.enqueue(errorEnvelope("", 400, "request_id too long"))
		return
	}

	resp, err := c.hub.dispatch(&env, time.Now().UnixMilli())
	if err != nil {
		log.Printf("ws dispatch request_id=%q: %v", env.GetRequestId(), err)
		c.enqueue(errorEnvelope(env.GetRequestId(), 500, "internal error"))
		return
	}
	if resp == nil {
		return
	}

	c.enqueue(resp)
}

// enqueue places serialized bytes onto the send channel. If the channel is full
// the connection is considered stuck and the read context is cancelled.
func (c *Conn) enqueue(env *dleaguev1.Envelope) {
	out, err := proto.Marshal(env)
	if err != nil {
		log.Printf("ws enqueue marshal: %v", err)
		return
	}
	logSend(env)
	select {
	case c.send <- out:
	default:
		log.Printf("ws send buffer full — closing connection")
		c.cancelRead()
	}
}
