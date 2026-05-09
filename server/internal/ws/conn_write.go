package ws

import (
	"context"
	"log"
	"time"

	"github.com/coder/websocket"
)

const (
	pingInterval = 30 * time.Second // how often to send a WS-level ping
	pingTimeout  = 90 * time.Second // how long to wait for the pong
)

// writeLoop drains the send channel and writes frames to the WebSocket.
// It also ticks a WS-level ping every pingInterval; if the peer does not
// respond within pingTimeout the connection is treated as dead and cancelRead
// is called to stop the readLoop.
//
// writeLoop exits when ctx is done or when the send channel is closed.
func (c *Conn) writeLoop(ctx context.Context, cancelRead context.CancelFunc) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case frame, ok := <-c.send:
			if !ok {
				// send channel closed; nothing more to write.
				return
			}
			if err := c.ws.Write(ctx, websocket.MessageBinary, frame); err != nil {
				if ctx.Err() == nil {
					log.Printf("ws write: %v", err)
				}
				cancelRead()
				return
			}

		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := c.ws.Ping(pingCtx)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("ws ping: %v", err)
				}
				cancelRead()
				return
			}
		}
	}
}
