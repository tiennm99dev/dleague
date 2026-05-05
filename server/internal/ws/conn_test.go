package ws

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// Conn.handle propagates Envelope unmarshal errors.
func TestConnHandleEnvelopeUnmarshalError(t *testing.T) {
	c := &Conn{hub: NewHub()}
	if err := c.handle(context.Background(), []byte{0xAA, 0xBB, 0xCC}); err == nil {
		t.Fatal("expected error for malformed envelope")
	}
}

// Conn.handle returns nil for unspecified type (no response, no error).
func TestConnHandleUnspecifiedType(t *testing.T) {
	c := &Conn{hub: NewHub()}
	data, err := proto.Marshal(&dleaguev1.Envelope{Type: dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := c.handle(context.Background(), data); err != nil {
		t.Fatalf("handle unspecified: %v", err)
	}
}

// Conn.handle propagates Ping payload unmarshal errors from dispatch.
func TestConnHandleDispatchError(t *testing.T) {
	c := &Conn{hub: NewHub()}
	env, err := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "test",
		Payload:   []byte{0xFF, 0xFE},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := c.handle(context.Background(), env); err == nil {
		t.Fatal("expected error from dispatch")
	}
}

// Hub.register / unregister update Count under concurrent-safe locking.
func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()
	if h.Count() != 0 {
		t.Fatalf("initial count = %d, want 0", h.Count())
	}
	c1, c2 := &Conn{}, &Conn{}
	h.register(c1)
	h.register(c2)
	if h.Count() != 2 {
		t.Fatalf("after 2 registers: count = %d", h.Count())
	}
	h.unregister(c1)
	h.unregister(c2)
	if h.Count() != 0 {
		t.Fatalf("after unregister: count = %d", h.Count())
	}
}

// handlePing rejects payloads that aren't valid Ping protobufs.
func TestHandlePingUnmarshalError(t *testing.T) {
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "test",
		Payload:   []byte{0xFF, 0xFE},
	}
	if _, err := handlePing(env, 0); err == nil {
		t.Fatal("expected error for invalid ping payload")
	}
}
