package ws

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

func TestDispatchPingProducesPong(t *testing.T) {
	h := NewHub()

	clientNow := time.Now().UnixMilli()
	pingBody, err := proto.Marshal(&dleaguev1.Ping{ClientUnixMs: clientNow})
	if err != nil {
		t.Fatalf("marshal ping: %v", err)
	}
	in := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "test",
		Payload:   pingBody,
	}

	serverNow := clientNow + 10
	out, err := h.dispatch(in, serverNow)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if out == nil {
		t.Fatal("dispatch ping returned nil response")
	}
	if out.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_PONG {
		t.Fatalf("type = %v, want PONG", out.GetType())
	}
	if out.GetRequestId() != "test" {
		t.Fatalf("request_id = %q, want 'test'", out.GetRequestId())
	}

	var pong dleaguev1.Pong
	if err := proto.Unmarshal(out.GetPayload(), &pong); err != nil {
		t.Fatalf("unmarshal pong: %v", err)
	}
	if pong.GetClientUnixMs() != clientNow {
		t.Fatalf("pong.client_unix_ms = %d, want %d", pong.GetClientUnixMs(), clientNow)
	}
	if pong.GetServerUnixMs() != serverNow {
		t.Fatalf("pong.server_unix_ms = %d, want %d", pong.GetServerUnixMs(), serverNow)
	}
}

func TestDispatchUnknownTypeReturnsNil(t *testing.T) {
	h := NewHub()
	in := &dleaguev1.Envelope{Type: dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED}
	out, err := h.dispatch(in, 0)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %v, want nil", out)
	}
}
