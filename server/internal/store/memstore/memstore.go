// Package memstore is the in-memory implementation of store.Store.
//
// Two purposes:
//
//  1. Unit-test backbone — handlers and the WS hub can be exercised without
//     spinning up Couchbase + Redis.
//  2. Migration-readiness witness — having a second backend behind the same
//     interface is what proves the seam holds.
//
// Concurrency: a single mutex guards everything. This is a test fixture and
// a fallback, not a hot path. Don't add goroutine optimizations.
package memstore

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Store is a goroutine-safe in-memory store.Store implementation.
type Store struct {
	mu sync.Mutex

	users    map[string]store.User    // by uid
	puzzles  map[string]store.Puzzle  // by date
	attempts map[string]store.Attempt // by "uid::date"
	matches  map[string]store.Match   // by id

	// Redis-equivalent state.
	leaderboards map[string]map[string]int64 // board → uid → score
	presence     map[string]time.Time        // uid → expires-at
	cache        map[string]cacheEntry       // key → (val, expires-at)

	closed bool
	now    func() time.Time
}

type cacheEntry struct {
	val    []byte
	expiry time.Time
}

// New returns an empty memstore.
func New() *Store {
	return &Store{
		users:        make(map[string]store.User),
		puzzles:      make(map[string]store.Puzzle),
		attempts:     make(map[string]store.Attempt),
		matches:      make(map[string]store.Match),
		leaderboards: make(map[string]map[string]int64),
		presence:     make(map[string]time.Time),
		cache:        make(map[string]cacheEntry),
		now:          time.Now,
	}
}

// SetClock overrides the time source. Test-only.
func (s *Store) SetClock(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

func attemptKey(uid, date string) string { return uid + "::" + date }

// ─── persistent ops ────────────────────────────────────────────────────

func (s *Store) UpsertUserOnFirstAuth(_ context.Context, claims store.AuthClaims) (store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.User{}, store.ErrClosed
	}

	now := s.now()
	if u, ok := s.users[claims.UID]; ok {
		// Existing user — refresh mutable fields, preserve beta provenance.
		u.Email = claims.Email
		u.DisplayName = claims.DisplayName
		u.Provider = claims.Provider
		u.LastSeen = now
		s.users[claims.UID] = u
		return u, nil
	}
	u := store.User{
		UID:          claims.UID,
		Email:        claims.Email,
		DisplayName:  claims.DisplayName,
		Provider:     claims.Provider,
		IsBetaTester: true,
		BetaSignupAt: now,
		CreatedAt:    now,
		LastSeen:     now,
	}
	s.users[claims.UID] = u
	return u, nil
}

func (s *Store) GetUser(_ context.Context, uid string) (store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.User{}, store.ErrClosed
	}
	u, ok := s.users[uid]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func (s *Store) TouchLastSeen(_ context.Context, uid string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	u, ok := s.users[uid]
	if !ok {
		return store.ErrNotFound
	}
	u.LastSeen = at
	s.users[uid] = u
	return nil
}

func (s *Store) GetPuzzle(_ context.Context, date string) (store.Puzzle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.Puzzle{}, store.ErrClosed
	}
	p, ok := s.puzzles[date]
	if !ok {
		return store.Puzzle{}, store.ErrNotFound
	}
	return p, nil
}

func (s *Store) PutPuzzle(_ context.Context, p store.Puzzle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	s.puzzles[p.Date] = p
	return nil
}

func (s *Store) GetAttempt(_ context.Context, uid, date string) (store.Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.Attempt{}, store.ErrClosed
	}
	a, ok := s.attempts[attemptKey(uid, date)]
	if !ok {
		return store.Attempt{}, store.ErrNotFound
	}
	return a, nil
}

func (s *Store) UpsertAttempt(_ context.Context, a store.Attempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	s.attempts[attemptKey(a.UID, a.PuzzleDate)] = a
	return nil
}

func (s *Store) GetMatch(_ context.Context, matchID string) (store.Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.Match{}, store.ErrClosed
	}
	m, ok := s.matches[matchID]
	if !ok {
		return store.Match{}, store.ErrNotFound
	}
	return m, nil
}

func (s *Store) UpsertMatch(_ context.Context, m store.Match) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	s.matches[m.ID] = m
	return nil
}

func (s *Store) ListUserMatches(_ context.Context, uid string, n int) ([]store.Match, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, store.ErrClosed
	}
	var out []store.Match
	for _, m := range s.matches {
		for _, p := range m.Players {
			if p == uid {
				out = append(out, m)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

// Export writes one JSONL row per persistent doc, prefixed with the collection
// name so a future importer can fan out by type without a separate header.
func (s *Store) Export(_ context.Context, w io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}

	enc := json.NewEncoder(w)
	emit := func(collection string, doc any) error {
		return enc.Encode(map[string]any{"collection": collection, "doc": doc})
	}

	// Stable iteration order makes the export reproducible — useful for tests.
	for _, uid := range sortedKeys(s.users) {
		if err := emit("users", s.users[uid]); err != nil {
			return err
		}
	}
	for _, d := range sortedKeys(s.puzzles) {
		if err := emit("puzzles", s.puzzles[d]); err != nil {
			return err
		}
	}
	for _, k := range sortedKeys(s.attempts) {
		if err := emit("attempts", s.attempts[k]); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(s.matches) {
		if err := emit("matches", s.matches[id]); err != nil {
			return err
		}
	}
	return nil
}

// ─── cache + leaderboards ──────────────────────────────────────────────

func (s *Store) SubmitScore(_ context.Context, board, uid string, score int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	b, ok := s.leaderboards[board]
	if !ok {
		b = make(map[string]int64)
		s.leaderboards[board] = b
	}
	// GT semantics — only update on strictly higher score.
	if cur, exists := b[uid]; exists && cur >= score {
		return nil
	}
	b[uid] = score
	return nil
}

func (s *Store) TopN(_ context.Context, board string, n int) ([]store.Rank, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, store.ErrClosed
	}
	b := s.leaderboards[board]
	out := make([]store.Rank, 0, len(b))
	for uid, sc := range b {
		out = append(out, store.Rank{UID: uid, Score: sc})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].UID < out[j].UID // stable tie-break
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (s *Store) MarkOnline(_ context.Context, uid string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	s.presence[uid] = s.now().Add(ttl)
	return nil
}

func (s *Store) IsOnline(_ context.Context, uid string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false, store.ErrClosed
	}
	exp, ok := s.presence[uid]
	if !ok {
		return false, nil
	}
	if s.now().After(exp) {
		delete(s.presence, uid)
		return false, nil
	}
	return true, nil
}

func (s *Store) CacheGet(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false, store.ErrClosed
	}
	e, ok := s.cache[key]
	if !ok {
		return nil, false, nil
	}
	if !e.expiry.IsZero() && s.now().After(e.expiry) {
		delete(s.cache, key)
		return nil, false, nil
	}
	out := make([]byte, len(e.val))
	copy(out, e.val)
	return out, true, nil
}

func (s *Store) CacheSet(_ context.Context, key string, val []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	stored := make([]byte, len(val))
	copy(stored, val)
	e := cacheEntry{val: stored}
	if ttl > 0 {
		e.expiry = s.now().Add(ttl)
	}
	s.cache[key] = e
	return nil
}

func (s *Store) Ping(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return store.ErrClosed
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// sortedKeys returns the keys of m in lexicographic order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Compile-time assertion: Store implements store.Store.
var _ store.Store = (*Store)(nil)
