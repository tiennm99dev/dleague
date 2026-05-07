package mongodb_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/mongodb"
)

// Live integration test — only runs when MONGODB_TEST_URI is set. Set it to
// a disposable Atlas cluster URI; the test isolates itself by writing to
// MONGODB_TEST_DB (default "dleague_test") and dropping that database at
// the end of each test.
//
// Example:
//
//	MONGODB_TEST_URI='mongodb+srv://user:pass@cluster.mongodb.net' \
//	  go test ./server/internal/store/mongodb/...
func newTestClient(t *testing.T) *mongodb.Client {
	t.Helper()
	uri := os.Getenv("MONGODB_TEST_URI")
	if uri == "" {
		t.Skip("MONGODB_TEST_URI unset; skipping live integration")
	}
	dbName := os.Getenv("MONGODB_TEST_DB")
	if dbName == "" {
		dbName = "dleague_test"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := mongodb.New(ctx, mongodb.Config{URI: uri, Database: dbName})
	if err != nil {
		t.Fatalf("mongodb.New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestPing(t *testing.T) {
	c := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestUpsertUserOnFirstAuth_Idempotent(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()

	uid := "test-uid-" + time.Now().UTC().Format("20060102150405.000")
	first, err := c.UpsertUserOnFirstAuth(ctx, store.AuthClaims{
		UID: uid, Email: "x@y", Provider: "password",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !first.IsBetaTester || first.BetaSignupAt.IsZero() {
		t.Fatalf("beta fields not stamped: %+v", first)
	}
	betaAt := first.BetaSignupAt

	// Second call: BetaSignupAt + CreatedAt must be unchanged; mutable fields refresh.
	time.Sleep(10 * time.Millisecond)
	second, err := c.UpsertUserOnFirstAuth(ctx, store.AuthClaims{
		UID: uid, Email: "x2@y", Provider: "google.com",
	})
	if err != nil {
		t.Fatalf("re-auth upsert: %v", err)
	}
	if !second.BetaSignupAt.Equal(betaAt) {
		t.Errorf("BetaSignupAt drifted: %v → %v", betaAt, second.BetaSignupAt)
	}
	if second.Email != "x2@y" || second.Provider != "google.com" {
		t.Errorf("mutable fields not refreshed: %+v", second)
	}
	if !second.LastSeen.After(first.LastSeen) {
		t.Errorf("LastSeen did not advance: %v vs %v", first.LastSeen, second.LastSeen)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	c := newTestClient(t)
	_, err := c.GetUser(context.Background(), "definitely-missing-"+time.Now().Format("150405.000"))
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPuzzleRoundTrip(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	date := "test-" + time.Now().UTC().Format("20060102150405.000")
	want := store.Puzzle{Date: date, Word: "TESTS", Hint: "noun", Difficulty: 2}
	if err := c.PutPuzzle(ctx, want); err != nil {
		t.Fatalf("PutPuzzle: %v", err)
	}
	got, err := c.GetPuzzle(ctx, date)
	if err != nil {
		t.Fatalf("GetPuzzle: %v", err)
	}
	if got.Word != want.Word || got.Difficulty != want.Difficulty {
		t.Errorf("roundtrip mismatch: %+v vs %+v", got, want)
	}
}

func TestGetPuzzle_NotFound(t *testing.T) {
	c := newTestClient(t)
	_, err := c.GetPuzzle(context.Background(), "definitely-missing-"+time.Now().Format("150405.000"))
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpsertAttempt_Replace(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	uid := "uid-" + time.Now().UTC().Format("150405.000")
	date := "2026-05-07"

	a := store.Attempt{UID: uid, PuzzleDate: date, Guesses: []string{"HELLO"}, Score: 100}
	if err := c.UpsertAttempt(ctx, a); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	a.Guesses = append(a.Guesses, "WORLD")
	a.Score = 200
	a.Won = true
	if err := c.UpsertAttempt(ctx, a); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := c.GetAttempt(ctx, uid, date)
	if err != nil {
		t.Fatalf("GetAttempt: %v", err)
	}
	if got.Score != 200 || !got.Won || len(got.Guesses) != 2 {
		t.Errorf("replace failed: %+v", got)
	}
}

func TestListUserMatches_OrderAndLimit(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	uid := "uid-" + time.Now().UTC().Format("150405.000")

	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		m := store.Match{
			ID:        uid + "-m" + time.Now().Format("20060102150405.000") + "-" + string(rune('A'+i)),
			Players:   []string{uid, "other"},
			Mode:      "async",
			State:     "ended",
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}
		if err := c.UpsertMatch(ctx, m); err != nil {
			t.Fatalf("UpsertMatch %d: %v", i, err)
		}
	}
	got, err := c.ListUserMatches(ctx, uid, 3)
	if err != nil {
		t.Fatalf("ListUserMatches: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(got))
	}
	// CreatedAt should descend.
	for i := 1; i < len(got); i++ {
		if !got[i-1].CreatedAt.After(got[i].CreatedAt) {
			t.Errorf("matches not in desc order at i=%d: %v then %v", i, got[i-1].CreatedAt, got[i].CreatedAt)
		}
	}
}

func TestSubmitScore_GTSemantics(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	board := "lb-test-" + time.Now().UTC().Format("150405.000")
	uid := "uid-A"

	if err := c.SubmitScore(ctx, board, uid, 10); err != nil {
		t.Fatalf("submit 10: %v", err)
	}
	// Lower score must NOT overwrite.
	if err := c.SubmitScore(ctx, board, uid, 5); err != nil {
		t.Fatalf("submit 5: %v", err)
	}
	rows, err := c.TopN(ctx, board, 1)
	if err != nil || len(rows) != 1 || rows[0].Score != 10 {
		t.Fatalf("expected score=10, got %+v err=%v", rows, err)
	}
	// Higher score updates.
	if err := c.SubmitScore(ctx, board, uid, 20); err != nil {
		t.Fatalf("submit 20: %v", err)
	}
	rows, _ = c.TopN(ctx, board, 1)
	if len(rows) != 1 || rows[0].Score != 20 {
		t.Errorf("expected score=20 after raise, got %+v", rows)
	}
}

func TestTopN_OrderAndLimit(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	board := "lb-top-" + time.Now().UTC().Format("150405.000")
	scores := map[string]int64{"a": 30, "b": 50, "c": 10, "d": 90, "e": 20}
	for uid, s := range scores {
		if err := c.SubmitScore(ctx, board, uid, s); err != nil {
			t.Fatalf("submit %s=%d: %v", uid, s, err)
		}
	}
	rows, err := c.TopN(ctx, board, 3)
	if err != nil {
		t.Fatalf("TopN: %v", err)
	}
	if len(rows) != 3 || rows[0].Score != 90 || rows[1].Score != 50 || rows[2].Score != 30 {
		t.Errorf("expected [90,50,30], got %+v", rows)
	}
}

func TestSubmitScore_ConcurrentSameUID(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	board := "lb-conc-" + time.Now().UTC().Format("150405.000")
	uid := "race"

	var wg sync.WaitGroup
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(score int64) {
			defer wg.Done()
			_ = c.SubmitScore(ctx, board, uid, score)
		}(int64(i))
	}
	wg.Wait()
	rows, err := c.TopN(ctx, board, 1)
	if err != nil || len(rows) != 1 || rows[0].Score != 10 {
		t.Errorf("expected final score=10, got %+v err=%v", rows, err)
	}
}

func TestIsOnline_AccurateBeforeAndAfterTTL(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	uid := "presence-" + time.Now().UTC().Format("150405.000")

	if err := c.MarkOnline(ctx, uid, 30*time.Second); err != nil {
		t.Fatalf("MarkOnline: %v", err)
	}
	online, err := c.IsOnline(ctx, uid)
	if err != nil || !online {
		t.Errorf("expected online=true, got %v err=%v", online, err)
	}

	// Re-mark with a tight TTL; query-side filter should report offline
	// before the background TTL purger runs.
	if err := c.MarkOnline(ctx, uid, 1*time.Second); err != nil {
		t.Fatalf("MarkOnline (short): %v", err)
	}
	time.Sleep(2 * time.Second)
	online, err = c.IsOnline(ctx, uid)
	if err != nil || online {
		t.Errorf("expected online=false after TTL, got %v err=%v", online, err)
	}
}

func TestMarkOnline_ConcurrentSameUID(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	uid := "p-race-" + time.Now().UTC().Format("150405.000")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ttl := time.Duration(10+i) * time.Second
			_ = c.MarkOnline(ctx, uid, ttl)
		}(i)
	}
	wg.Wait()
	online, err := c.IsOnline(ctx, uid)
	if err != nil || !online {
		t.Errorf("expected online=true after concurrent MarkOnline, got %v err=%v", online, err)
	}
}

func TestCacheRoundTrip_TTL(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "cache-" + time.Now().UTC().Format("150405.000")

	if err := c.CacheSet(ctx, key, []byte("hello"), 1*time.Second); err != nil {
		t.Fatalf("CacheSet: %v", err)
	}
	val, hit, err := c.CacheGet(ctx, key)
	if err != nil || !hit || string(val) != "hello" {
		t.Fatalf("expected hit, got hit=%v val=%q err=%v", hit, val, err)
	}
	time.Sleep(2 * time.Second)
	_, hit, err = c.CacheGet(ctx, key)
	if err != nil || hit {
		t.Errorf("expected miss after TTL, got hit=%v err=%v", hit, err)
	}
}

func TestCacheSet_ZeroTTL_NoExpiry(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	key := "cache-noexp-" + time.Now().UTC().Format("150405.000")

	if err := c.CacheSet(ctx, key, []byte("forever"), 0); err != nil {
		t.Fatalf("CacheSet ttl=0: %v", err)
	}
	time.Sleep(2 * time.Second)
	val, hit, err := c.CacheGet(ctx, key)
	if err != nil || !hit || string(val) != "forever" {
		t.Errorf("expected hit on no-TTL doc, got hit=%v val=%q err=%v", hit, val, err)
	}

	// Switching back to a TTL must $unset the prior expireAt-less state.
	if err := c.CacheSet(ctx, key, []byte("transient"), 1*time.Second); err != nil {
		t.Fatalf("CacheSet ttl=1s: %v", err)
	}
	time.Sleep(2 * time.Second)
	_, hit, err = c.CacheGet(ctx, key)
	if err != nil || hit {
		t.Errorf("expected miss after switching to TTL, got hit=%v err=%v", hit, err)
	}
}
