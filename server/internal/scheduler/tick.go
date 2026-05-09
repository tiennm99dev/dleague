// Package scheduler provides background tick jobs for dleague.
// It runs until its context is cancelled (SIGTERM via main's signal context).
package scheduler

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

// LeaderboardRefresher is the subset of LeaderboardRepo used by the scheduler.
type LeaderboardRefresher interface {
	Refresh(ctx context.Context, gameID, period, date string) error
}

// MatchSweeper is the subset of MatchRepo used by the scheduler.
type MatchSweeper interface {
	SweepExpired(ctx context.Context) error
}

// Repos holds the repository interfaces required by the scheduler.
type Repos struct {
	Leaderboard LeaderboardRefresher
	Match       MatchSweeper
}

// Config holds tunable intervals. Zero values fall back to the defaults.
type Config struct {
	LeaderboardInterval time.Duration // default 5 min
	SweepInterval       time.Duration // default 15 min
}

func (c Config) leaderboardInterval() time.Duration {
	if c.LeaderboardInterval > 0 {
		return c.LeaderboardInterval
	}
	return 5 * time.Minute
}

func (c Config) sweepInterval() time.Duration {
	if c.SweepInterval > 0 {
		return c.SweepInterval
	}
	return 15 * time.Minute
}

// Run starts the background tick loops and blocks until ctx is cancelled.
// Call as a goroutine: go scheduler.Run(ctx, cfg, repos).
func Run(ctx context.Context, cfg Config, repos Repos) {
	lbTicker := time.NewTicker(cfg.leaderboardInterval())
	sweepTicker := time.NewTicker(cfg.sweepInterval())
	defer lbTicker.Stop()
	defer sweepTicker.Stop()

	log.Printf("scheduler: started (lb=%s sweep=%s)",
		cfg.leaderboardInterval(), cfg.sweepInterval())

	for {
		select {
		case <-ctx.Done():
			log.Printf("scheduler: stopped")
			return

		case t := <-lbTicker.C:
			date := t.UTC().Format("2006-01-02")
			tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := repos.Leaderboard.Refresh(tickCtx, "wordle", "daily", date); err != nil {
				if errors.Is(err, store.ErrLeaderboardTooLarge) {
					log.Printf("scheduler: WARN leaderboard refresh skipped (too many matches): %v", err)
				} else {
					log.Printf("scheduler: leaderboard refresh error: %v", err)
				}
			} else {
				log.Printf("scheduler: leaderboard refreshed for wordle/daily/%s", date)
			}
			cancel()

		case <-sweepTicker.C:
			tickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := repos.Match.SweepExpired(tickCtx); err != nil {
				log.Printf("scheduler: sweep expired matches error: %v", err)
			} else {
				log.Printf("scheduler: swept expired matches")
			}
			cancel()
		}
	}
}
