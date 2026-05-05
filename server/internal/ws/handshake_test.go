package ws_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"

	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/ws"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

type fakeVerifier struct {
	uid string
	err error
}

func (f *fakeVerifier) Verify(_ context.Context, tok string) (store.AuthClaims, error) {
	if f.err != nil {
		return store.AuthClaims{}, f.err
	}
	uid := f.uid
	if uid == "" {
		uid = "uid-" + tok
	}
	return store.AuthClaims{UID: uid}, nil
}

// startServer wires the WS upgrade handler with the given verifier and
// returns the test server's WS URL.
func startServer(t *testing.T, v ws.TokenVerifier) (*httptest.Server, string) {
	t.Helper()
	hub := ws.NewHub()
	srv := httptest.NewServer(ws.UpgradeHandler(hub, ws.UpgradeOptions{Verifier: v}))
	t.Cleanup(srv.Close)
	return srv, strings.Replace(srv.URL, "http://", "ws://", 1)
}

func sendAuth(t *testing.T, ctx context.Context, c *websocket.Conn, token string) {
	t.Helper()
	req := &dleaguev1.AuthRequest{IdToken: token, Version: 1}
	payload, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal AuthRequest: %v", err)
	}
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REQUEST,
		RequestId: "auth-1",
		Payload:   payload,
	}
	out, err := proto.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, out); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readEnvelope(t *testing.T, ctx context.Context, c *websocket.Conn) *dleaguev1.Envelope {
	t.Helper()
	mt, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.MessageBinary {
		t.Fatalf("frame type = %d, want binary", mt)
	}
	var env dleaguev1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &env
}

func TestHandshakeValidTokenAcksAndPingWorks(t *testing.T) {
	_, wsURL := startServer(t, &fakeVerifier{uid: "u1"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	sendAuth(t, ctx, c, "good-token")
	resp := readEnvelope(t, ctx, c)
	if resp.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_AUTH_RESPONSE {
		t.Fatalf("type = %v, want AUTH_RESPONSE", resp.GetType())
	}
	var ar dleaguev1.AuthResponse
	if err := proto.Unmarshal(resp.GetPayload(), &ar); err != nil {
		t.Fatalf("unmarshal AuthResponse: %v", err)
	}
	if !ar.GetOk() || ar.GetUid() != "u1" {
		t.Fatalf("AuthResponse = %+v", &ar)
	}

	// Post-handshake ping must round-trip normally.
	pingPayload, _ := proto.Marshal(&dleaguev1.Ping{ClientUnixMs: 1700000000000})
	pingEnv, _ := proto.Marshal(&dleaguev1.Envelope{
		Type: dleaguev1.MessageType_MESSAGE_TYPE_PING, RequestId: "p", Payload: pingPayload,
	})
	if err := c.Write(ctx, websocket.MessageBinary, pingEnv); err != nil {
		t.Fatalf("post-handshake write: %v", err)
	}
	pong := readEnvelope(t, ctx, c)
	if pong.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_PONG {
		t.Fatalf("type = %v, want PONG", pong.GetType())
	}
}

func TestHandshakeInvalidTokenClosesConn(t *testing.T) {
	_, wsURL := startServer(t, &fakeVerifier{err: errors.New("bad token")})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	sendAuth(t, ctx, c, "expired-token")

	// Server sends AUTH_RESPONSE{ok:false} then closes.
	resp := readEnvelope(t, ctx, c)
	var ar dleaguev1.AuthResponse
	if err := proto.Unmarshal(resp.GetPayload(), &ar); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ar.GetOk() {
		t.Fatalf("expected ok=false, got %+v", &ar)
	}

	// Subsequent read must fail (connection closed).
	if _, _, err := c.Read(ctx); err == nil {
		t.Fatal("expected close error after bad-token rejection")
	}
}

func TestHandshakeTimeoutClosesConn(t *testing.T) {
	// Verifier never invoked because the client never sends AUTH.
	_, wsURL := startServer(t, &fakeVerifier{})

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	// Wait for handshake timeout (5s) + slack.
	deadline := time.Now().Add(ws.HandshakeTimeout + 2*time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := c.Read(ctx); err != nil {
			return // got the expected close
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("expected connection close after handshake timeout")
}

func TestHandshakeDisabledFallsBackToPingPong(t *testing.T) {
	// When Verifier is nil, the handler should preserve the Phase-1 behaviour
	// — no handshake, immediate dispatch.
	hub := ws.NewHub()
	srv := httptest.NewServer(ws.UpgradeHandler(hub, ws.UpgradeOptions{}))
	defer srv.Close()
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	pingPayload, _ := proto.Marshal(&dleaguev1.Ping{ClientUnixMs: 1700000000000})
	env, _ := proto.Marshal(&dleaguev1.Envelope{
		Type: dleaguev1.MessageType_MESSAGE_TYPE_PING, RequestId: "p", Payload: pingPayload,
	})
	if err := c.Write(ctx, websocket.MessageBinary, env); err != nil {
		t.Fatalf("write: %v", err)
	}
	pong := readEnvelope(t, ctx, c)
	if pong.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_PONG {
		t.Fatalf("type = %v, want PONG", pong.GetType())
	}
}
