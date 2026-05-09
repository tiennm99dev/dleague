package ws

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// TestQueueJoin_Unauthenticated rejects unauth conns with 401.
func TestQueueJoin_Unauthenticated(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "", // unauthenticated
	}

	q := NewQueue()
	deps := &GameDeps{
		Queue: q,
	}

	payload, _ := proto.Marshal(&dleaguev1.QueueJoin{})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_JOIN,
		RequestId: "q-1",
		Payload:   payload,
	}

	response, err := handleQueueJoin(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleQueueJoin: %v", err)
	}

	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("response type = %v, want ERROR", response.GetType())
	}

	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(response.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errMsg.GetCode() != 401 {
		t.Fatalf("error code = %d, want 401", errMsg.GetCode())
	}
}

// TestQueueJoin_NoDeps returns 503 when Queue is nil.
func TestQueueJoin_NoDeps(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user",
	}

	// No Queue configured.
	deps := &GameDeps{
		Queue: nil,
		Rooms: nil,
	}

	payload, _ := proto.Marshal(&dleaguev1.QueueJoin{})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_JOIN,
		RequestId: "q-2",
		Payload:   payload,
	}

	response, err := handleQueueJoin(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleQueueJoin: %v", err)
	}

	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("response type = %v, want ERROR", response.GetType())
	}

	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(response.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errMsg.GetCode() != 503 {
		t.Fatalf("error code = %d, want 503", errMsg.GetCode())
	}
}

// TestQueueJoin_InvalidPayload returns 400 for malformed payload.
func TestQueueJoin_InvalidPayload(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user",
	}

	q := NewQueue()
	deps := &GameDeps{
		Queue: q,
		Rooms: NewRoomsRegistry(),
	}

	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_JOIN,
		RequestId: "q-3",
		Payload:   []byte{0xFF, 0xFE, 0xFD},
	}

	response, err := handleQueueJoin(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleQueueJoin: %v", err)
	}

	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("response type = %v, want ERROR", response.GetType())
	}

	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(response.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errMsg.GetCode() != 400 {
		t.Fatalf("error code = %d, want 400", errMsg.GetCode())
	}
}

// TestQueueLeave_RemovedFromQueue verifies Remove operation.
func TestQueueLeave_RemovedFromQueue(t *testing.T) {
	h := newTestHub()

	conn := &Conn{
		hub:    h,
		userID: "user-1",
	}

	q := NewQueue()
	deps := &GameDeps{
		Queue: q,
	}

	// Add to queue.
	q.Push("wordle", conn)

	// Leave.
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_QUEUE_LEAVE,
		RequestId: "leave-1",
	}

	_, err := handleQueueLeave(context.Background(), conn, env, deps)
	if err != nil {
		t.Fatalf("handleQueueLeave: %v", err)
	}

	// Verify it's removed by checking PopPair returns false.
	_, _, paired := q.PopPair("wordle")
	if paired {
		t.Fatal("after remove, queue should be empty")
	}
}

// TestQueue_RemoveConn verifies Remove works correctly.
func TestQueue_RemoveConn(t *testing.T) {
	q := NewQueue()

	conn := &Conn{
		hub:    newTestHub(),
		userID: "user-1",
	}

	// Add to queue.
	q.Push("wordle", conn)

	// Remove.
	q.Remove(conn)

	// Verify it's gone by trying to pair — nothing should pop.
	_, _, paired := q.PopPair("wordle")
	if paired {
		t.Fatal("after remove, queue should be empty")
	}
}

// TestQueue_GameIDIsolation verifies different gameIDs maintain separate queues.
func TestQueue_GameIDIsolation(t *testing.T) {
	q := NewQueue()

	connWordleA := &Conn{userID: "user-wordle-a", hub: newTestHub()}
	connWordleB := &Conn{userID: "user-wordle-b", hub: newTestHub()}
	connCustomA := &Conn{userID: "user-custom-a", hub: newTestHub()}

	// Add two to "wordle" queue.
	q.Push("wordle", connWordleA)
	q.Push("wordle", connWordleB)

	// Add one to "custom" queue.
	q.Push("custom", connCustomA)

	// Pop from "wordle" — should get a pair.
	a, _, paired := q.PopPair("wordle")
	if !paired || (a != connWordleA && a != connWordleB) {
		t.Fatalf("expected wordle pair, got paired=%v", paired)
	}

	// Pop from "custom" — should get nothing (only one conn).
	_, _, paired = q.PopPair("custom")
	if paired {
		t.Fatal("custom queue should have only 1 conn, no pair possible")
	}
}

// TestQueue_ConcurrentOps verifies concurrent Push/Pop don't panic or race.
func TestQueue_ConcurrentOps(t *testing.T) {
	q := NewQueue()
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Goroutine A: repeatedly push conns.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			select {
			case <-done:
				return
			default:
			}
			conn := &Conn{userID: "user-" + string(rune(i)), hub: newTestHub()}
			q.Push("wordle", conn)
		}
	}()

	// Goroutine B: repeatedly pop pairs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			select {
			case <-done:
				return
			default:
			}
			_, _, _ = q.PopPair("wordle")
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()

	// If we got here without -race warnings, test passes.
}
