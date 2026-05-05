// Package composed glues the Couchbase (persistent) and Redis (cache +
// leaderboard) clients into a single store.Store value. main.go consumes
// this — every other layer of the server only sees the unified interface.
package composed

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Persistent is the subset of store.Store implemented by the Couchbase client.
type Persistent interface {
	UpsertUserOnFirstAuth(ctx context.Context, claims store.AuthClaims) (store.User, error)
	GetUser(ctx context.Context, uid string) (store.User, error)
	TouchLastSeen(ctx context.Context, uid string, at time.Time) error
	GetPuzzle(ctx context.Context, date string) (store.Puzzle, error)
	PutPuzzle(ctx context.Context, p store.Puzzle) error
	GetAttempt(ctx context.Context, uid, date string) (store.Attempt, error)
	UpsertAttempt(ctx context.Context, a store.Attempt) error
	GetMatch(ctx context.Context, matchID string) (store.Match, error)
	UpsertMatch(ctx context.Context, m store.Match) error
	ListUserMatches(ctx context.Context, uid string, n int) ([]store.Match, error)
	Export(ctx context.Context, w io.Writer) error
	Ping(ctx context.Context) error
	Close() error
}

// Cache is the subset of store.Store implemented by the Redis client.
type Cache interface {
	SubmitScore(ctx context.Context, board, uid string, score int64) error
	TopN(ctx context.Context, board string, n int) ([]store.Rank, error)
	MarkOnline(ctx context.Context, uid string, ttl time.Duration) error
	IsOnline(ctx context.Context, uid string) (bool, error)
	CacheGet(ctx context.Context, key string) ([]byte, bool, error)
	CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error
	Ping(ctx context.Context) error
	Close() error
}

// Store routes every store.Store method to the appropriate backend.
type Store struct {
	persistent Persistent
	cache      Cache
}

// New wires the two halves. Both must be non-nil.
func New(p Persistent, c Cache) (*Store, error) {
	if p == nil || c == nil {
		return nil, fmt.Errorf("composed: both Persistent and Cache required")
	}
	return &Store{persistent: p, cache: c}, nil
}

// ─── persistent passthrough ────────────────────────────────────────────

func (s *Store) UpsertUserOnFirstAuth(ctx context.Context, claims store.AuthClaims) (store.User, error) {
	return s.persistent.UpsertUserOnFirstAuth(ctx, claims)
}
func (s *Store) GetUser(ctx context.Context, uid string) (store.User, error) {
	return s.persistent.GetUser(ctx, uid)
}
func (s *Store) TouchLastSeen(ctx context.Context, uid string, at time.Time) error {
	return s.persistent.TouchLastSeen(ctx, uid, at)
}
func (s *Store) GetPuzzle(ctx context.Context, date string) (store.Puzzle, error) {
	return s.persistent.GetPuzzle(ctx, date)
}
func (s *Store) PutPuzzle(ctx context.Context, p store.Puzzle) error {
	return s.persistent.PutPuzzle(ctx, p)
}
func (s *Store) GetAttempt(ctx context.Context, uid, date string) (store.Attempt, error) {
	return s.persistent.GetAttempt(ctx, uid, date)
}
func (s *Store) UpsertAttempt(ctx context.Context, a store.Attempt) error {
	return s.persistent.UpsertAttempt(ctx, a)
}
func (s *Store) GetMatch(ctx context.Context, matchID string) (store.Match, error) {
	return s.persistent.GetMatch(ctx, matchID)
}
func (s *Store) UpsertMatch(ctx context.Context, m store.Match) error {
	return s.persistent.UpsertMatch(ctx, m)
}
func (s *Store) ListUserMatches(ctx context.Context, uid string, n int) ([]store.Match, error) {
	return s.persistent.ListUserMatches(ctx, uid, n)
}
func (s *Store) Export(ctx context.Context, w io.Writer) error {
	return s.persistent.Export(ctx, w)
}

// ─── cache passthrough ────────────────────────────────────────────────

func (s *Store) SubmitScore(ctx context.Context, board, uid string, score int64) error {
	return s.cache.SubmitScore(ctx, board, uid, score)
}
func (s *Store) TopN(ctx context.Context, board string, n int) ([]store.Rank, error) {
	return s.cache.TopN(ctx, board, n)
}
func (s *Store) MarkOnline(ctx context.Context, uid string, ttl time.Duration) error {
	return s.cache.MarkOnline(ctx, uid, ttl)
}
func (s *Store) IsOnline(ctx context.Context, uid string) (bool, error) {
	return s.cache.IsOnline(ctx, uid)
}
func (s *Store) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	return s.cache.CacheGet(ctx, key)
}
func (s *Store) CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return s.cache.CacheSet(ctx, key, val, ttl)
}

// ─── lifecycle ────────────────────────────────────────────────────────

// Ping pings both backends; first error wins.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.persistent.Ping(ctx); err != nil {
		return fmt.Errorf("composed: persistent: %w", err)
	}
	if err := s.cache.Ping(ctx); err != nil {
		return fmt.Errorf("composed: cache: %w", err)
	}
	return nil
}

// Close closes both; reports the first error but always attempts both.
func (s *Store) Close() error {
	pErr := s.persistent.Close()
	cErr := s.cache.Close()
	if pErr != nil {
		return pErr
	}
	return cErr
}

// Compile-time assertion: Store implements store.Store.
var _ store.Store = (*Store)(nil)
