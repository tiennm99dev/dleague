package ws

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/store"
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

	// Auth fields — populated during UpgradeHandler and updated by AuthRefresh.
	userID         string    // Firebase UID; empty means unauthenticated
	isAnonymous    bool      // true when sign_in_provider == "anonymous"
	tokenExpiresAt time.Time // when the current ID token expires
}

// UpgradeOptions controls WebSocket Accept behaviour. Zero value enforces the
// coder/websocket default same-origin policy. To allow cross-origin clients,
// populate AllowedOrigins (host:port matched case-insensitively).
type UpgradeOptions struct {
	AllowedOrigins []string
}

// UpgradeHandler returns an http.HandlerFunc that upgrades to WebSocket,
// verifies the Firebase ID token from the Sec-WebSocket-Protocol header,
// and drives the read/write loops until the client disconnects.
//
// Expected header format:
//
//	Sec-WebSocket-Protocol: dleague.v1, fb.<idToken>
//
// A missing or malformed token causes a 401 response before any upgrade.
func UpgradeHandler(hub *Hub, opts UpgradeOptions) http.HandlerFunc {
	accept := websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
		OriginPatterns:  opts.AllowedOrigins,
		// Echo the dleague.v1 subprotocol so the client sees a successful negotiation.
		Subprotocols: []string{"dleague.v1"},
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

		// Extract and verify the Firebase ID token from Sec-WebSocket-Protocol.
		// When hub.verifier is nil (tests / dev without Firebase) auth is bypassed:
		// the connection proceeds with an empty userID (unauthenticated guest mode).
		var connUserID string
		var connAnonymous bool
		var connTokenExp time.Time

		if hub.verifier != nil {
			protoHeader := r.Header.Get("Sec-WebSocket-Protocol")
			idToken, err := extractFirebaseToken(protoHeader)
			if err != nil {
				http.Error(w, "missing or malformed fb. token in Sec-WebSocket-Protocol", http.StatusUnauthorized)
				return
			}

			token, err := hub.verifier.VerifyIDToken(r.Context(), idToken)
			if err != nil {
				http.Error(w, "invalid or expired Firebase ID token", http.StatusUnauthorized)
				return
			}
			connUserID = token.UID
			connAnonymous = isAnonymousToken(token)
			connTokenExp = time.Unix(token.Expires, 0)

			// Upsert the user document in Mongo on every connection (idempotent).
			if hub.userRepo != nil {
				profile := tokenToProfile(token.Claims)
				profile.IsAnonymous = connAnonymous
				if uErr := hub.userRepo.UpsertByUID(r.Context(), token.UID, profile); uErr != nil {
					log.Printf("ws upsert user %q: %v", token.UID, uErr)
					// Non-fatal: connection proceeds even when the DB write fails.
				}
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
			ws:             c,
			hub:            hub,
			send:           make(chan []byte, sendBufSize),
			cancelRead:     cancelRead,
			userID:         connUserID,
			isAnonymous:    connAnonymous,
			tokenExpiresAt: connTokenExp,
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

// tokenToProfile maps Firebase ID token claims to a store.UserProfile.
// Anonymous users produce an empty DisplayName (no email/name claim).
func tokenToProfile(claims map[string]interface{}) store.UserProfile {
	p := store.UserProfile{}
	if name, ok := claims["name"].(string); ok {
		p.DisplayName = name
	}
	// email_verified claim indicates a verified provider (Google, email+link, etc.).
	if verified, ok := claims["email_verified"].(bool); ok {
		p.Verified = verified
	}
	return p
}

// extractFirebaseToken parses the Sec-WebSocket-Protocol header and returns
// the ID token from the first element prefixed with "fb.".
//
// Expected format: "dleague.v1, fb.<idToken>"
// Returns an error when no "fb." element is found or the token part is empty.
func extractFirebaseToken(protoHeader string) (string, error) {
	if protoHeader == "" {
		return "", fmt.Errorf("Sec-WebSocket-Protocol header is empty")
	}
	for _, part := range strings.Split(protoHeader, ",") {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(trimmed, "fb.") {
			tok := strings.TrimPrefix(trimmed, "fb.")
			if tok == "" {
				return "", fmt.Errorf("fb. prefix present but token is empty")
			}
			return tok, nil
		}
	}
	return "", fmt.Errorf("no fb.<token> entry in Sec-WebSocket-Protocol: %q", protoHeader)
}

// readLoop reads inbound frames until the context is cancelled or an error occurs.
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
		c.handleFrame(ctx, data)
	}
}

// handleFrame processes one inbound binary frame. It enqueues any response to
// c.send; on send-channel overflow it cancels the connection.
func (c *Conn) handleFrame(ctx context.Context, data []byte) {
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

	resp, err := c.hub.dispatch(ctx, &env, c, time.Now().UnixMilli())
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
