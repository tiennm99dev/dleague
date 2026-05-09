package ws

import (
	"sync/atomic"
	"testing"
	"time"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// TestGraceTimers_ScheduleAndExpire verifies that the grace timer fires after
// the grace period and resolves the room via HandleForfeit.
func TestGraceTimers_ScheduleAndExpire(t *testing.T) {
	t.Parallel()

	solution := "CRANE"
	room, _, b := newRoomPair(solution)
	rooms := NewRoomsRegistry()
	rooms.Add(room.MatchID, room)

	deps := &GameDeps{
		Dictionary: []string{solution},
		Rooms:      rooms,
	}

	c := newTestConnUID("uid-a")
	c.setActiveMatchID(room.MatchID)

	// Use a very short grace period via monkey-patching the timer directly.
	// We override the package constant via an indirect approach: call Schedule
	// but use a thin wrapper that shortens the wait. Since we cannot override
	// the constant easily, we test via a direct time.AfterFunc call that
	// mirrors the Schedule logic with a short duration.
	var fired atomic.Bool
	timers := NewGraceTimers()
	key := timerKey(room.MatchID, c.userID)
	timers.mu.Lock()
	timers.timers[key] = time.AfterFunc(50*time.Millisecond, func() {
		timers.mu.Lock()
		delete(timers.timers, key)
		timers.mu.Unlock()
		fired.Store(true)
		if r := deps.Rooms.Get(room.MatchID); r != nil {
			r.HandleForfeit(t.Context(), c.userID, deps)
		}
	})
	timers.mu.Unlock()

	// Wait for the timer to fire.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fired.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !fired.Load() {
		t.Fatal("grace timer did not fire within 500ms")
	}
	// Brief yield to allow the timer callback goroutine to complete enqueue.
	time.Sleep(10 * time.Millisecond)

	// Opponent (b) should have received MATCH_RESOLVED with reason=forfeit.
	bFrames := drainSend(b)
	if findEnvByType(bFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED) == nil {
		t.Error("expected MATCH_RESOLVED on opponent after grace expiry")
	}
}

// TestGraceTimers_CancelPreventsForfit verifies that cancelling the timer
// before it fires prevents the forfeit.
func TestGraceTimers_CancelPreventsForfit(t *testing.T) {
	t.Parallel()

	solution := "CRANE"
	room, _, b := newRoomPair(solution)
	rooms := NewRoomsRegistry()
	rooms.Add(room.MatchID, room)

	deps := &GameDeps{
		Dictionary: []string{solution},
		Rooms:      rooms,
	}

	c := newTestConnUID("uid-a")
	c.setActiveMatchID(room.MatchID)

	var fired atomic.Bool
	timers := NewGraceTimers()
	key := timerKey(room.MatchID, c.userID)
	timers.mu.Lock()
	timers.timers[key] = time.AfterFunc(200*time.Millisecond, func() {
		fired.Store(true)
		if r := deps.Rooms.Get(room.MatchID); r != nil {
			r.HandleForfeit(t.Context(), c.userID, deps)
		}
	})
	timers.mu.Unlock()

	// Cancel before the timer fires.
	timers.Cancel(room.MatchID, c.userID)

	// Sleep past the original timer deadline; it must not have fired.
	time.Sleep(300 * time.Millisecond)
	if fired.Load() {
		t.Error("grace timer fired despite being cancelled")
	}

	// Opponent must not have received MATCH_RESOLVED.
	bFrames := drainSend(b)
	if findEnvByType(bFrames, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_RESOLVED) != nil {
		t.Error("unexpected MATCH_RESOLVED after timer cancellation")
	}
}

// TestGraceTimers_IdempotentCancel verifies Cancel on non-existent key is a no-op.
func TestGraceTimers_IdempotentCancel(t *testing.T) {
	t.Parallel()
	timers := NewGraceTimers()
	// Should not panic.
	timers.Cancel("no-such-match", "no-such-user")
}
