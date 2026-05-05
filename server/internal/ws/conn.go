package ws

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

const (
	readLimit    = 1 << 20
	writeTimeout = 10 * time.Second
	idleTimeout  = 60 * time.Second
)

// Conn is one client WebSocket connection. The hub owns its lifecycle.
type Conn struct {
	ws  *websocket.Conn
	hub *Hub
}

// UpgradeOptions controls WebSocket Accept behaviour. Zero value enforces the
// nhooyr default same-origin policy. To allow cross-origin clients, populate
// AllowedOrigins (host:port matched case-insensitively).
type UpgradeOptions struct {
	AllowedOrigins []string
}

// UpgradeHandler returns an http.HandlerFunc that upgrades to WebSocket and
// drives the read loop until the client disconnects or an error occurs.
func UpgradeHandler(hub *Hub, opts UpgradeOptions) http.HandlerFunc {
	accept := websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
		OriginPatterns:  opts.AllowedOrigins,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &accept)
		if err != nil {
			log.Printf("ws accept: %v", err)
			return
		}
		c.SetReadLimit(readLimit)

		conn := &Conn{ws: c, hub: hub}
		hub.register(conn)
		defer hub.unregister(conn)
		defer c.CloseNow()

		conn.readLoop(r.Context())
	}
}

func (c *Conn) readLoop(ctx context.Context) {
	for {
		readCtx, cancel := context.WithTimeout(ctx, idleTimeout)
		mt, data, err := c.ws.Read(readCtx)
		cancel()
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
		if err := c.handle(ctx, data); err != nil {
			log.Printf("ws handle: %v", err)
			return
		}
	}
}

func (c *Conn) handle(ctx context.Context, data []byte) error {
	var env dleaguev1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		return err
	}
	logRecv(&env)

	resp, err := c.hub.dispatch(&env, time.Now().UnixMilli())
	if err != nil {
		return err
	}
	if resp == nil {
		return nil
	}

	out, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	logSend(resp)

	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.ws.Write(writeCtx, websocket.MessageBinary, out)
}
