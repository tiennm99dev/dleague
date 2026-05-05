// Package store is the dleague data layer.
//
// `Store` is the migration seam: any concrete impl (memstore for tests,
// composed{couchbase,redis} for prod) plugs in identically. Per the plan,
// `gocb` lives only inside `internal/store/couchbase/`, `go-redis` only
// inside `internal/store/redis/`. The composed impl (`store/composed`) wires
// the two together so the rest of the server sees one interface.
package store

import (
	"context"
	"io"
	"time"
)

// Store is the unified data interface used by HTTP handlers and the WS hub.
type Store interface {
	// Persistent (Couchbase-backed in prod).
	UpsertUserOnFirstAuth(ctx context.Context, claims AuthClaims) (User, error)
	GetUser(ctx context.Context, uid string) (User, error)
	TouchLastSeen(ctx context.Context, uid string, at time.Time) error

	GetPuzzle(ctx context.Context, date string) (Puzzle, error)
	PutPuzzle(ctx context.Context, p Puzzle) error

	GetAttempt(ctx context.Context, uid, date string) (Attempt, error)
	UpsertAttempt(ctx context.Context, a Attempt) error

	GetMatch(ctx context.Context, matchID string) (Match, error)
	UpsertMatch(ctx context.Context, m Match) error
	ListUserMatches(ctx context.Context, uid string, n int) ([]Match, error)

	// Export streams every persistent doc as JSONL to w. Migration escape
	// hatch — Phase 12's `cmd/dleague-export` wraps this.
	Export(ctx context.Context, w io.Writer) error

	// Cache + leaderboards (Redis-backed in prod).
	SubmitScore(ctx context.Context, board, uid string, score int64) error
	TopN(ctx context.Context, board string, n int) ([]Rank, error)

	MarkOnline(ctx context.Context, uid string, ttl time.Duration) error
	IsOnline(ctx context.Context, uid string) (bool, error)

	CacheGet(ctx context.Context, key string) ([]byte, bool, error)
	CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// Lifecycle.
	Ping(ctx context.Context) error
	Close() error
}

// AuthClaims is the minimal set of identity facts taken from a verified
// Firebase ID token, used to upsert the local user record.
type AuthClaims struct {
	UID         string
	Email       string
	DisplayName string
	Provider    string // "password" | "google.com" | "anonymous" | …
}

// User is the persistent profile + beta-tester ledger entry.
type User struct {
	UID           string    `json:"uid"`
	Email         string    `json:"email,omitempty"`
	DisplayName   string    `json:"displayName,omitempty"`
	Provider      string    `json:"provider"`
	IsBetaTester  bool      `json:"isBetaTester"`
	BetaSignupAt  time.Time `json:"betaSignupAt"`
	CreatedAt     time.Time `json:"createdAt"`
	LastSeen      time.Time `json:"lastSeen"`
}

// Puzzle is the daily puzzle definition. ID is the date in YYYY-MM-DD.
type Puzzle struct {
	Date       string `json:"date"`
	Word       string `json:"word"`
	Hint       string `json:"hint,omitempty"`
	Difficulty int    `json:"difficulty"`
}

// Attempt is one user's run at one daily puzzle.
type Attempt struct {
	UID         string    `json:"uid"`
	PuzzleDate  string    `json:"puzzleDate"`
	Guesses     []string  `json:"guesses"`
	Won         bool      `json:"won"`
	Score       int64     `json:"score"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
	InProgress  bool      `json:"inProgress"`
}

// Match is a head-to-head game between players.
type Match struct {
	ID         string    `json:"id"`
	Players    []string  `json:"players"`
	Mode       string    `json:"mode"` // "async" | "sync"
	PuzzleDate string    `json:"puzzleDate"`
	State      string    `json:"state"` // "pending" | "active" | "ended"
	Turns      int       `json:"turns"`
	Winner     string    `json:"winner,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	EndedAt    time.Time `json:"endedAt,omitempty"`
}

// Rank is a leaderboard row.
type Rank struct {
	UID   string `json:"uid"`
	Score int64  `json:"score"`
}
