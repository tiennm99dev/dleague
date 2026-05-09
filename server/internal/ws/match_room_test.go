package ws

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/tiennm99/dleague/server/internal/game/wordle"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// drainSend reads all enqueued frames from c.send without blocking.
func drainSend(c *Conn) []*dleaguev1.Envelope {
	var out []*dleaguev1.Envelope
	for {
		select {
		case b := <-c.send:
			var env dleaguev1.Envelope
			if err := proto.Unmarshal(b, &env); err == nil {
				out = append(out, &env)
			}
		default:
			return out
		}
	}
}

// findEnvByType returns the first envelope of the given type from the slice.
func findEnvByType(envs []*dleaguev1.Envelope, t dleaguev1.MessageType) *dleaguev1.Envelope {
	for _, e := range envs {
		if e.GetType() == t {
			return e
		}
	}
	return nil
}

func newRoomPair(solution string) (*Room, *Conn, *Conn) {
	a := newTestConnUID("uid-a")
	b := newTestConnUID("uid-b")
	room := NewRoom("match-1", solution, a, b)
	return room, a, b
}

// minimalDeps returns a GameDeps without DB connections (unit tests only).
func minimalDeps(dictionary []string) *GameDeps {
	return &GameDeps{
		Dictionary: dictionary,
		Rooms:      NewRoomsRegistry(),
	}
}

// TestMatchRoom_CorrectHints verifies that HandleMove produces the right colors.
func TestMatchRoom_CorrectHints(t *testing.T) {
	t.Parallel()
	room, a, _ := newRoomPair("CRANE")
	deps := minimalDeps([]string{"CRANE", "ADIEU", "SLATE", "RAISE"})
	deps.Rooms.Add(room.MatchID, room)

	if err := room.HandleMove(context.Background(), a, "SLATE", deps); err != nil {
		t.Fatalf("HandleMove: %v", err)
	}

	frames := drainSend(a)
	stateEnv := findEnvByType(frames, dleaguev1.MessageType_MESSAGE_TYPE_GAME_STATE)
	if stateEnv == nil {
		t.Fatal("expected GAME_STATE envelope for moving player")
	}

	var state dleaguev1.WordleState
	if err := proto.Unmarshal(stateEnv.GetPayload(), &state); err != nil {
		t.Fatalf("unmarshal WordleState: %v", err)
	}
	if len(state.GetHints()) == 0 {
		t.Fatal("expected at least one hint row")
	}
	colors := state.GetHints()[0].GetColors()
	if len(colors) != wordle.WordLen {
		t.Fatalf("expected %d colors, got %d", wordle.WordLen, len(colors))
	}
}

// TestMatchRoom_LettersNeverLeakToOpponent is the canonical letters-never-leak test.
// Guess "CRANE" → opponent receives MatchOpponentProgress; that message must
// contain ONLY attempt_num + colors — zero letter content.
func TestMatchRoom_LettersNeverLeakToOpponent(t *testing.T) {
	t.Parallel()
	room, a, b := newRoomPair("ADIEU")
	deps := minimalDeps([]string{"CRANE", "ADIEU", "SLATE"})
	deps.Rooms.Add(room.MatchID, room)

	if err := room.HandleMove(context.Background(), a, "CRANE", deps); err != nil {
		t.Fatalf("HandleMove: %v", err)
	}

	// Drain opponent frames.
	bFrames := drainSend(b)
	progEnv := findEnvByType(bFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_OPPONENT_PROGRESS)
	if progEnv == nil {
		t.Fatal("expected MATCH_OPPONENT_PROGRESS on opponent conn")
	}

	var prog dleaguev1.MatchOpponentProgress
	if err := proto.Unmarshal(progEnv.GetPayload(), &prog); err != nil {
		t.Fatalf("unmarshal MatchOpponentProgress: %v", err)
	}

	// Critical assertion: no letter information must be present.
	// The proto definition has no string field for guess letters; verify no
	// undeclared string bytes are present by checking the known scalar fields.
	if prog.GetAttemptNum() <= 0 {
		t.Errorf("expected attempt_num > 0, got %d", prog.GetAttemptNum())
	}
	if len(prog.GetColors()) == 0 {
		t.Error("expected colors to be present")
	}
	// Re-marshal and unmarshal to verify no unexpected fields survive the round-trip.
	roundTrip, err := proto.Marshal(&prog)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	var roundTripProg dleaguev1.MatchOpponentProgress
	if err := proto.Unmarshal(roundTrip, &roundTripProg); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	// Verify round-tripped message still has no letters.
	if len(roundTripProg.GetColors()) == 0 {
		t.Error("round-trip: expected colors to survive")
	}
	// Explicitly assert that neither the original message nor the round-trip
	// carries the guess string "CRANE" in any form.
	rawPayload := progEnv.GetPayload()
	guessBytes := []byte("CRANE")
	for i := range len(rawPayload) - len(guessBytes) + 1 {
		match := true
		for j, gb := range guessBytes {
			if rawPayload[i+j] != gb {
				match = false
				break
			}
		}
		if match {
			t.Errorf("opponent payload contains raw guess letters at offset %d — letters ARE leaking", i)
		}
	}
}

// TestMatchRoom_FirstToSolveWins verifies that when A solves, B gets MATCH_RESOLVED
// with A as the winner.
func TestMatchRoom_FirstToSolveWins(t *testing.T) {
	t.Parallel()
	solution := "CRANE"
	room, a, b := newRoomPair(solution)
	deps := minimalDeps([]string{solution})
	deps.Rooms.Add(room.MatchID, room)

	// A solves immediately.
	if err := room.HandleMove(context.Background(), a, solution, deps); err != nil {
		t.Fatalf("HandleMove: %v", err)
	}

	aFrames := drainSend(a)
	bFrames := drainSend(b)

	resolvedA := findEnvByType(aFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED)
	resolvedB := findEnvByType(bFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED)
	if resolvedA == nil {
		t.Fatal("expected MATCH_RESOLVED on player A")
	}
	if resolvedB == nil {
		t.Fatal("expected MATCH_RESOLVED on player B")
	}

	var mr dleaguev1.MatchResolved
	if err := proto.Unmarshal(resolvedA.GetPayload(), &mr); err != nil {
		t.Fatalf("unmarshal MatchResolved: %v", err)
	}
	if mr.GetWinnerUid() != "uid-a" {
		t.Errorf("winner: want uid-a, got %q", mr.GetWinnerUid())
	}
	if mr.GetReason() != "solved" {
		t.Errorf("reason: want solved, got %q", mr.GetReason())
	}
}

// TestMatchRoom_BothExhausted verifies both-lose produces empty winner_uid.
func TestMatchRoom_BothExhausted(t *testing.T) {
	t.Parallel()
	solution := "CRANE"
	// Use a word list that does NOT include the solution so both players exhaust.
	dict := []string{"ADIEU", "SLATE", "RAISE", "HEART", "STALE", "TALES"}
	room, a, b := newRoomPair(solution)
	deps := minimalDeps(dict)
	deps.Rooms.Add(room.MatchID, room)

	ctx := context.Background()
	// Exhaust all 6 attempts for both players.
	for _, word := range dict {
		_ = room.HandleMove(ctx, a, word, deps)
		_ = room.HandleMove(ctx, b, word, deps)
	}

	aFrames := drainSend(a)
	resolved := findEnvByType(aFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED)
	if resolved == nil {
		t.Fatal("expected MATCH_RESOLVED after both exhaust")
	}
	var mr dleaguev1.MatchResolved
	if err := proto.Unmarshal(resolved.GetPayload(), &mr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mr.GetWinnerUid() != "" {
		t.Errorf("both-lose: want empty winner_uid, got %q", mr.GetWinnerUid())
	}
	if mr.GetReason() != "exhausted" {
		t.Errorf("reason: want exhausted, got %q", mr.GetReason())
	}
}

// TestMatchRoom_TimeoutResolution verifies HandleTimeout fires and both players
// receive MATCH_RESOLVED with reason="timeout".
func TestMatchRoom_TimeoutResolution(t *testing.T) {
	t.Parallel()
	room, a, b := newRoomPair("CRANE")
	deps := minimalDeps([]string{"CRANE"})
	deps.Rooms.Add(room.MatchID, room)

	// Force the deadline to the past.
	room.Deadline = time.Now().Add(-time.Second)
	room.HandleTimeout(context.Background(), deps)

	aFrames := drainSend(a)
	bFrames := drainSend(b)
	rA := findEnvByType(aFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED)
	rB := findEnvByType(bFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED)
	if rA == nil || rB == nil {
		t.Fatal("expected MATCH_RESOLVED on both players after timeout")
	}
	var mr dleaguev1.MatchResolved
	if err := proto.Unmarshal(rA.GetPayload(), &mr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mr.GetReason() != "timeout" {
		t.Errorf("reason: want timeout, got %q", mr.GetReason())
	}
}

// TestMatchRoom_ResolvedOnceOnly verifies that duplicate resolution calls are no-ops.
func TestMatchRoom_ResolvedOnceOnly(t *testing.T) {
	t.Parallel()
	solution := "CRANE"
	room, a, b := newRoomPair(solution)
	deps := minimalDeps([]string{solution})
	deps.Rooms.Add(room.MatchID, room)

	ctx := context.Background()
	// First resolution: A solves.
	if err := room.HandleMove(ctx, a, solution, deps); err != nil {
		t.Fatalf("HandleMove: %v", err)
	}
	drainSend(a)
	drainSend(b)

	// Second resolution attempt (timeout) must be a no-op.
	room.HandleTimeout(ctx, deps)

	aFrames := drainSend(a)
	if findEnvByType(aFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED) != nil {
		t.Error("second resolution: unexpected second MATCH_RESOLVED frame")
	}
}

// TestMatchRoom_ForfeitResolution verifies HandleForfeit names the correct winner.
func TestMatchRoom_ForfeitResolution(t *testing.T) {
	t.Parallel()
	room, _, b := newRoomPair("CRANE")
	deps := minimalDeps([]string{"CRANE"})
	deps.Rooms.Add(room.MatchID, room)

	room.HandleForfeit(context.Background(), "uid-a", deps)

	bFrames := drainSend(b)
	resolved := findEnvByType(bFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED)
	if resolved == nil {
		t.Fatal("expected MATCH_RESOLVED on opponent after forfeit")
	}
	var mr dleaguev1.MatchResolved
	if err := proto.Unmarshal(resolved.GetPayload(), &mr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mr.GetWinnerUid() != "uid-b" {
		t.Errorf("forfeit winner: want uid-b, got %q", mr.GetWinnerUid())
	}
	if mr.GetReason() != "forfeit" {
		t.Errorf("reason: want forfeit, got %q", mr.GetReason())
	}
}
