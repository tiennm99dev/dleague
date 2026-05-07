// Package store is the dleague data layer.
//
// `Store` is the migration seam: any concrete impl (memstore for tests,
// mongodb for prod) plugs in identically. Per the plan,
// `go.mongodb.org/mongo-driver/v2` lives only inside `internal/store/mongodb/`.
// `make grep-isolation` enforces that boundary in CI.
package store

import (
	"context"
	"io"
	"time"
)

// Store is the unified data interface used by HTTP handlers and the WS hub.
type Store interface {
	// Persistent — durable documents.
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

	// Cache + leaderboards + presence — TTL-backed indexes in MongoDB.
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

// Entity types carry both `json` and `bson` tags. JSON drives REST + Export
// JSONL; BSON drives the MongoDB store.

// User is the persistent profile + beta-tester ledger entry.
type User struct {
	UID          string    `json:"uid"                    bson:"uid"`
	Email        string    `json:"email,omitempty"        bson:"email,omitempty"`
	DisplayName  string    `json:"displayName,omitempty"  bson:"displayName,omitempty"`
	Provider     string    `json:"provider"               bson:"provider"`
	IsBetaTester bool      `json:"isBetaTester"           bson:"isBetaTester"`
	BetaSignupAt time.Time `json:"betaSignupAt"           bson:"betaSignupAt"`
	CreatedAt    time.Time `json:"createdAt"              bson:"createdAt"`
	LastSeen     time.Time `json:"lastSeen"               bson:"lastSeen"`
}

// Puzzle is the daily puzzle definition. The date string (YYYY-MM-DD) is the
// natural key — `_id` in MongoDB.
type Puzzle struct {
	Date       string `json:"date"           bson:"_id"`
	Word       string `json:"word"           bson:"word"`
	Hint       string `json:"hint,omitempty" bson:"hint,omitempty"`
	Difficulty int    `json:"difficulty"     bson:"difficulty"`
}

// Attempt is one user's run at one daily puzzle.
type Attempt struct {
	UID         string    `json:"uid"                   bson:"uid"`
	PuzzleDate  string    `json:"puzzleDate"            bson:"puzzleDate"`
	Guesses     []string  `json:"guesses"               bson:"guesses"`
	Won         bool      `json:"won"                   bson:"won"`
	Score       int64     `json:"score"                 bson:"score"`
	CompletedAt time.Time `json:"completedAt,omitempty" bson:"completedAt,omitempty"`
	InProgress  bool      `json:"inProgress"            bson:"inProgress"`
}

// Match is a head-to-head game between players.
type Match struct {
	ID         string    `json:"id"                bson:"_id"`
	Players    []string  `json:"players"           bson:"players"`
	Mode       string    `json:"mode"              bson:"mode"` // "async" | "sync"
	PuzzleDate string    `json:"puzzleDate"        bson:"puzzleDate"`
	State      string    `json:"state"             bson:"state"` // "pending" | "active" | "ended"
	Turns      int       `json:"turns"             bson:"turns"`
	Winner     string    `json:"winner,omitempty"  bson:"winner,omitempty"`
	CreatedAt  time.Time `json:"createdAt"         bson:"createdAt"`
	EndedAt    time.Time `json:"endedAt,omitempty" bson:"endedAt,omitempty"`
}

// Rank is a leaderboard row.
type Rank struct {
	UID   string `json:"uid"   bson:"uid"`
	Score int64  `json:"score" bson:"score"`
}
