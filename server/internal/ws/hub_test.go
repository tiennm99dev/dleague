package ws

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// newTestHub creates a Hub with nil verifier and nil userRepo — safe for tests
// that do not exercise auth paths.
func newTestHub() *Hub {
	return NewHub(nil, nil)
}

func TestDispatchPingProducesPong(t *testing.T) {
	h := newTestHub()

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
	// PING does not require auth — pass a conn with empty userID.
	c := &Conn{hub: h}
	out, err := h.dispatch(context.Background(), in, c, serverNow)
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
	h := newTestHub()
	c := &Conn{hub: h}
	in := &dleaguev1.Envelope{Type: dleaguev1.MessageType_MESSAGE_TYPE_UNSPECIFIED}
	out, err := h.dispatch(context.Background(), in, c, 0)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if out != nil {
		t.Fatalf("out = %v, want nil", out)
	}
}

// TestDispatchRequiresAuthReturnsError401 verifies that a message type that
// requires authentication is rejected with an ERROR{401} when userID is empty.
func TestDispatchRequiresAuthReturnsError401(t *testing.T) {
	// Use a message type that requiresAuth returns true for.
	// MESSAGE_TYPE_UNSPECIFIED returns false, so pick a numeric value outside
	// the known-unauth set. Use value 99 (unknown future type — requiresAuth
	// default branch returns true).
	h := newTestHub()
	c := &Conn{hub: h, userID: ""} // unauthenticated
	in := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType(99),
		RequestId: "authtest",
	}
	out, err := h.dispatch(context.Background(), in, c, 0)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if out == nil {
		t.Fatal("expected ERROR envelope, got nil")
	}
	if out.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("type = %v, want ERROR", out.GetType())
	}
	if out.GetRequestId() != "authtest" {
		t.Fatalf("request_id = %q, want 'authtest'", out.GetRequestId())
	}

	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(out.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errMsg.GetCode() != 401 {
		t.Fatalf("code = %d, want 401", errMsg.GetCode())
	}
}

// Hub cap: 3rd register on a hub with MaxConns=2 must fail with ErrAtCapacity.
func TestHubMaxConnsRejectsOverCap(t *testing.T) {
	h := &Hub{conns: map[*Conn]struct{}{}, MaxConns: 2}

	c1, c2, c3 := &Conn{}, &Conn{}, &Conn{}
	if err := h.register(c1); err != nil {
		t.Fatalf("register c1: %v", err)
	}
	if err := h.register(c2); err != nil {
		t.Fatalf("register c2: %v", err)
	}
	err := h.register(c3)
	if !errors.Is(err, ErrAtCapacity) {
		t.Fatalf("expected ErrAtCapacity, got %v", err)
	}
	if h.Count() != 2 {
		t.Fatalf("count = %d after rejected register, want 2", h.Count())
	}
}

// Slot opens again after unregister clears a spot.
func TestHubMaxConnsFreesSlotOnUnregister(t *testing.T) {
	h := &Hub{conns: map[*Conn]struct{}{}, MaxConns: 2}

	c1, c2, c3 := &Conn{}, &Conn{}, &Conn{}
	_ = h.register(c1)
	_ = h.register(c2)
	h.unregister(c1)
	if err := h.register(c3); err != nil {
		t.Fatalf("register after unregister: %v", err)
	}
	if h.Count() != 2 {
		t.Fatalf("count = %d, want 2", h.Count())
	}
}

// Concurrent register/unregister under -race with a cap that allows some
// registrations to succeed and others to be rejected.
func TestHubConcurrentRegisterUnregisterRace(t *testing.T) {
	h := &Hub{conns: map[*Conn]struct{}{}, MaxConns: 10}

	const workers = 40
	conns := make([]*Conn, workers)
	for i := range conns {
		conns[i] = &Conn{}
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_ = h.register(conns[idx]) // may return ErrAtCapacity — that's fine
		}(i)
	}
	wg.Wait()

	// Drain all registered conns concurrently.
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			h.unregister(conns[idx])
		}(i)
	}
	wg.Wait()

	if h.Count() != 0 {
		t.Fatalf("count = %d after full drain, want 0", h.Count())
	}
}
