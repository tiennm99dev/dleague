package ws

import (
	"context"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	"github.com/tiennm99/dleague/server/internal/store"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// TestGameHandler_WordleMoveHappyPath verifies a valid WORDLE_MOVE produces a WORDLE_STATE response.
func TestGameHandler_WordleMoveHappyPath(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user-123",
	}

	// Set up test deps with a small dictionary and answers list.
	deps := &GameDeps{
		Dictionary: []string{"HELLO", "WORLD", "TESTS"},
		Answers:    []string{"TESTS"},
		DailyRepo:  &mockDailyPuzzleStore{},
	}

	// Create envelope with WORDLE_MOVE payload.
	movePayload, err := proto.Marshal(&dleaguev1.WordleMove{Guess: "HELLO"})
	if err != nil {
		t.Fatalf("marshal move: %v", err)
	}
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_WORDLE_MOVE,
		RequestId: "move-1",
		Payload:   movePayload,
	}

	// Dispatch the move.
	response, err := handleGameMove(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleGameMove: %v", err)
	}
	if response == nil {
		t.Fatal("response is nil")
	}
	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_WORDLE_STATE {
		t.Fatalf("response type = %v, want WORDLE_STATE", response.GetType())
	}

	// Unmarshal and verify state.
	var state dleaguev1.WordleState
	if err := proto.Unmarshal(response.GetPayload(), &state); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if len(state.GetGuesses()) != 1 || state.GetGuesses()[0] != "HELLO" {
		t.Fatalf("state.guesses = %v, want [HELLO]", state.GetGuesses())
	}
}

// TestGameHandler_InvalidWord verifies a word not in the dictionary returns a 400 error.
func TestGameHandler_InvalidWord(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user-123",
	}

	deps := &GameDeps{
		Dictionary: []string{"HELLO", "WORLD"},
		Answers:    []string{"HELLO"},
		DailyRepo:  &mockDailyPuzzleStore{},
	}

	// Try to submit a word not in the dictionary.
	movePayload, _ := proto.Marshal(&dleaguev1.WordleMove{Guess: "NOTAWORD"})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_WORDLE_MOVE,
		RequestId: "move-2",
		Payload:   movePayload,
	}

	response, err := handleGameMove(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleGameMove: %v", err)
	}
	if response == nil {
		t.Fatal("response is nil")
	}
	if response.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_ERROR {
		t.Fatalf("response type = %v, want ERROR", response.GetType())
	}

	// Verify error code is 400.
	var errMsg dleaguev1.Error
	if err := proto.Unmarshal(response.GetPayload(), &errMsg); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errMsg.GetCode() != 400 {
		t.Fatalf("error code = %d, want 400", errMsg.GetCode())
	}
}

// TestGameHandler_WrongLength verifies a guess with wrong length is rejected.
func TestGameHandler_WrongLength(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "test-user-123",
	}

	deps := &GameDeps{
		Dictionary: []string{"HELLO", "WORLD"},
		Answers:    []string{"HELLO"},
		DailyRepo:  &mockDailyPuzzleStore{},
	}

	// Submit a 4-letter word.
	movePayload, _ := proto.Marshal(&dleaguev1.WordleMove{Guess: "TEST"})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_WORDLE_MOVE,
		RequestId: "move-3",
		Payload:   movePayload,
	}

	response, err := handleGameMove(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleGameMove: %v", err)
	}
	if response == nil {
		t.Fatal("response is nil")
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

// TestGameHandler_UnauthenticatedRejected verifies unauthenticated conns get a 401 error.
func TestGameHandler_UnauthenticatedRejected(t *testing.T) {
	h := newTestHub()
	c := &Conn{
		hub:    h,
		userID: "", // unauthenticated
	}

	deps := &GameDeps{
		Dictionary: []string{"HELLO"},
		Answers:    []string{"HELLO"},
		DailyRepo:  &mockDailyPuzzleStore{},
	}

	movePayload, _ := proto.Marshal(&dleaguev1.WordleMove{Guess: "HELLO"})
	env := &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_WORDLE_MOVE,
		RequestId: "move-4",
		Payload:   movePayload,
	}

	response, err := handleGameMove(context.Background(), c, env, deps)
	if err != nil {
		t.Fatalf("handleGameMove: %v", err)
	}
	if response == nil {
		t.Fatal("response is nil")
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

// ── Mock dependencies ────────────────────────────────────────────────────────

// mockDailyPuzzleStore is a test stub for wordle.DailyPuzzleStore.
type mockDailyPuzzleStore struct{}

func (m *mockDailyPuzzleStore) GetByDate(ctx context.Context, date string) (*store.DailyPuzzle, error) {
	// Return a fixed puzzle for all dates in tests.
	return &store.DailyPuzzle{Solution: "TESTS"}, nil
}

func (m *mockDailyPuzzleStore) Upsert(ctx context.Context, p store.DailyPuzzle) error {
	return nil
}

var _ wordle.DailyPuzzleStore = (*mockDailyPuzzleStore)(nil)
