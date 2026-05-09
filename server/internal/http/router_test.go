package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/ws"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

func mustRouter(t *testing.T) http.Handler {
	t.Helper()
	r, err := NewRouter(t.TempDir(), ws.NewHub(nil, nil), ws.UpgradeOptions{}, nil, RouterOptions{})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

func TestHealthOK(t *testing.T) {
	r := mustRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body = %q, want 'ok'", string(body))
	}
}

func TestWSEndToEndPingPong(t *testing.T) {
	r := mustRouter(t)
	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.CloseNow()

	pingBody, err := proto.Marshal(&dleaguev1.Ping{ClientUnixMs: 1700000000000})
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	envIn, err := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "e2e",
		Payload:   pingBody,
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, envIn); err != nil {
		t.Fatalf("write: %v", err)
	}

	mt, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if mt != websocket.MessageBinary {
		t.Fatalf("frame type = %d, want binary", mt)
	}
	var envOut dleaguev1.Envelope
	if err := proto.Unmarshal(data, &envOut); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if envOut.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_PONG {
		t.Fatalf("response type = %v, want PONG", envOut.GetType())
	}
	if envOut.GetRequestId() != "e2e" {
		t.Fatalf("response request_id = %q, want 'e2e'", envOut.GetRequestId())
	}
}
