package scheduler_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/scheduler"
)

// stubRefresher counts Refresh calls.
type stubRefresher struct {
	calls atomic.Int64
}

func (s *stubRefresher) Refresh(_ context.Context, _, _, _ string) error {
	s.calls.Add(1)
	return nil
}

// stubSweeper counts SweepExpired calls.
type stubSweeper struct {
	calls atomic.Int64
}

func (s *stubSweeper) SweepExpired(_ context.Context) error {
	s.calls.Add(1)
	return nil
}

// TestTickFires verifies that Run invokes Refresh and SweepExpired at least
// once within a short window using stub implementations (no Mongo required).
func TestTickFires(t *testing.T) {
	t.Parallel()

	refresher := &stubRefresher{}
	sweeper := &stubSweeper{}

	cfg := scheduler.Config{
		LeaderboardInterval: 20 * time.Millisecond,
		SweepInterval:       30 * time.Millisecond,
	}
	repos := scheduler.Repos{
		Leaderboard: refresher,
		Match:       sweeper,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx, cfg, repos)
		close(done)
	}()

	<-done

	if refresher.calls.Load() == 0 {
		t.Error("expected Refresh to be called at least once")
	}
	if sweeper.calls.Load() == 0 {
		t.Error("expected SweepExpired to be called at least once")
	}
}

// TestRunStopsOnContextCancel verifies that Run exits promptly after cancel.
func TestRunStopsOnContextCancel(t *testing.T) {
	t.Parallel()

	cfg := scheduler.Config{
		LeaderboardInterval: 1 * time.Hour, // won't fire
		SweepInterval:       1 * time.Hour,
	}
	repos := scheduler.Repos{
		Leaderboard: &stubRefresher{},
		Match:       &stubSweeper{},
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx, cfg, repos)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Error("Run did not stop within 500ms after context cancel")
	}
}
