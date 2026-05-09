package ws

import (
	"context"
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
		hub:        NewHub(nil, nil),
		send:       make(chan []byte, sendBufSize),
		cancelRead: cancel,
	}
}

func nopCancel() (struct{}, func()) {
	return struct{}{}, func() {}
}

// readFirstType drains the send channel and returns the MessageType of the
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
	c.handleFrame(context.Background(), []byte{0xAA, 0xBB, 0xCC})

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
	c.handleFrame(context.Background(), data)

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
	c.handleFrame(context.Background(), data)

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
	c.handleFrame(context.Background(), data)

	got := readFirstType(c)
	if got != dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED {
		t.Fatalf("expected nothing in send, got type %v", got)
	}
}

// A message type requiring auth with empty userID produces ERROR{401}.
func TestHandleFrameRequiresAuthUnauthenticated(t *testing.T) {
	c := newTestConn()
	c.userID = "" // unauthenticated

	// MessageType 99 is unknown — requiresAuth defaults to true.
	data, _ := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType(99),
		RequestId: "need-auth",
	})
	c.handleFrame(context.Background(), data)

	got := readFirstType(c)
	if got != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("expected MESSAGE_TYPE_ERROR for unauthenticated message, got %v", got)
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
			c.handleFrame(context.Background(), data)
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
	h := NewHub(nil, nil)
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

// --- extractFirebaseToken table-driven tests ---

func TestExtractFirebaseToken(t *testing.T) {
	cases := []struct {
		name      string
		header    string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "valid format",
			header:    "dleague.v1, fb.abc.def.ghi",
			wantToken: "abc.def.ghi",
		},
		{
			name:      "no space around comma",
			header:    "dleague.v1,fb.tok123",
			wantToken: "tok123",
		},
		{
			name:    "missing fb. entry",
			header:  "dleague.v1, other.protocol",
			wantErr: true,
		},
		{
			name:    "empty header",
			header:  "",
			wantErr: true,
		},
		{
			name:    "fb. prefix but empty token",
			header:  "dleague.v1, fb.",
			wantErr: true,
		},
		{
			name:      "multiple entries — first fb. wins",
			header:    "dleague.v1, fb.first.token, fb.second.token",
			wantToken: "first.token",
		},
		{
			name:      "only fb. entry",
			header:    "fb.solo.token",
			wantToken: "solo.token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractFirebaseToken(tc.header)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got token=%q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantToken {
				t.Fatalf("token = %q, want %q", got, tc.wantToken)
			}
		})
	}
}
