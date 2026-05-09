package ws

import (
	"strings"
	"sync"
	"testing"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// newTestConn builds a minimal Conn wired to a hub — no real WebSocket.
// send channel is buffered so enqueue doesn't block in unit tests.
func newTestConn() *Conn {
	_, cancel := nopCancel()
	return &Conn{
		hub:        NewHub(),
		send:       make(chan []byte, sendBufSize),
		cancelRead: cancel,
	}
}

func nopCancel() (struct{}, func()) {
	return struct{}{}, func() {}
}

// readErrorType drains the send channel and returns the MessageType of the
// first envelope found (or MESSAGE_TYPE_UNSPECIFIED if channel is empty).
func readFirstType(c *Conn) dleaguev1.MessageType {
	select {
	case raw := <-c.send:
		var env dleaguev1.Envelope
		if err := proto.Unmarshal(raw, &env); err != nil {
			return dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED
		}
		return env.GetType()
	default:
		return dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED
	}
}

// Malformed proto bytes → client receives MESSAGE_TYPE_ERROR, conn stays open.
func TestHandleFrameMalformedProto(t *testing.T) {
	c := newTestConn()
	c.handleFrame([]byte{0xAA, 0xBB, 0xCC})

	got := readFirstType(c)
	if got != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("expected MESSAGE_TYPE_ERROR, got %v", got)
	}
	// send channel still open (conn not cancelled) — verify by checking it
	// accepts another write without panic.
	select {
	case c.send <- []byte{}:
	default:
		t.Fatal("send channel unexpectedly full after single error envelope")
	}
}

// Oversized request_id → MESSAGE_TYPE_ERROR, not logged raw.
func TestHandleFrameOversizedRequestID(t *testing.T) {
	c := newTestConn()
	bigID := strings.Repeat("x", maxReqIDLen+1)
	data, err := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: bigID,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	c.handleFrame(data)

	got := readFirstType(c)
	if got != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("expected MESSAGE_TYPE_ERROR, got %v", got)
	}
}

// Conn.handleFrame returns a valid Pong for a well-formed Ping.
func TestHandleFramePingReturnsPong(t *testing.T) {
	c := newTestConn()
	pingBody, _ := proto.Marshal(&dleaguev1.Ping{ClientUnixMs: 1_000_000})
	data, _ := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "req1",
		Payload:   pingBody,
	})
	c.handleFrame(data)

	got := readFirstType(c)
	if got != dleaguev1.MessageType_MESSAGE_TYPE_PONG {
		t.Fatalf("expected MESSAGE_TYPE_PONG, got %v", got)
	}
}

// Conn.handleFrame for unspecified type produces no send frame.
func TestHandleFrameUnspecifiedTypeNoResponse(t *testing.T) {
	c := newTestConn()
	data, _ := proto.Marshal(&dleaguev1.Envelope{
		Type: dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
	})
	c.handleFrame(data)

	got := readFirstType(c)
	if got != dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED {
		t.Fatalf("expected nothing in send, got type %v", got)
	}
}

// Concurrent enqueue via send channel under -race.
func TestConcurrentEnqueue(t *testing.T) {
	c := newTestConn()

	pingBody, _ := proto.Marshal(&dleaguev1.Ping{ClientUnixMs: 42})
	data, _ := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "race",
		Payload:   pingBody,
	})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			c.handleFrame(data)
		}()
	}
	wg.Wait()
	// Drain whatever landed in the buffer — we just want -race to not flag anything.
	drained := 0
	for {
		select {
		case <-c.send:
			drained++
		default:
			goto done
		}
	}
done:
	t.Logf("drained %d frames from send channel", drained)
}

// Hub.register/unregister update Count under concurrent-safe locking.
func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()
	if h.Count() != 0 {
		t.Fatalf("initial count = %d, want 0", h.Count())
	}
	c1, c2 := &Conn{}, &Conn{}
	if err := h.register(c1); err != nil {
		t.Fatalf("register c1: %v", err)
	}
	if err := h.register(c2); err != nil {
		t.Fatalf("register c2: %v", err)
	}
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
