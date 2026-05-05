package memstore_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/memstore"
)

func mustStore(t *testing.T) *memstore.Store {
	t.Helper()
	s := memstore.New()
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertUserOnFirstAuthStampsBetaFieldsOnceOnly(t *testing.T) {
	s := mustStore(t)

	t1 := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 9, 30, 0, 0, time.UTC)
	clock := &fakeClock{now: t1}
	s.SetClock(clock.Now)

	ctx := context.Background()

	u, err := s.UpsertUserOnFirstAuth(ctx, store.AuthClaims{
		UID: "u1", Email: "a@b", DisplayName: "A", Provider: "password",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !u.IsBetaTester {
		t.Errorf("IsBetaTester should be true on first auth")
	}
	if !u.BetaSignupAt.Equal(t1) {
		t.Errorf("BetaSignupAt = %v, want %v", u.BetaSignupAt, t1)
	}

	clock.now = t2
	u2, err := s.UpsertUserOnFirstAuth(ctx, store.AuthClaims{
		UID: "u1", Email: "a2@b", DisplayName: "A2", Provider: "google.com",
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if !u2.BetaSignupAt.Equal(t1) {
		t.Errorf("BetaSignupAt overwritten on re-auth: got %v, want preserved %v", u2.BetaSignupAt, t1)
	}
	if !u2.LastSeen.Equal(t2) {
		t.Errorf("LastSeen not refreshed: got %v, want %v", u2.LastSeen, t2)
	}
	if u2.Email != "a2@b" || u2.Provider != "google.com" {
		t.Errorf("mutable fields not refreshed: %+v", u2)
	}
}

func TestGetUserNotFound(t *testing.T) {
	s := mustStore(t)
	_, err := s.GetUser(context.Background(), "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPuzzleAttemptMatchRoundTrip(t *testing.T) {
	s := mustStore(t)
	ctx := context.Background()

	p := store.Puzzle{Date: "2026-05-05", Word: "league", Difficulty: 3}
	if err := s.PutPuzzle(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPuzzle(ctx, "2026-05-05")
	if err != nil {
		t.Fatal(err)
	}
	if got.Word != "league" {
		t.Errorf("Word = %q", got.Word)
	}

	a := store.Attempt{UID: "u1", PuzzleDate: "2026-05-05", Won: true, Score: 800}
	if err := s.UpsertAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	a2, err := s.GetAttempt(ctx, "u1", "2026-05-05")
	if err != nil {
		t.Fatal(err)
	}
	if a2.Score != 800 || !a2.Won {
		t.Errorf("Attempt round-trip: %+v", a2)
	}

	m := store.Match{ID: "m1", Players: []string{"u1", "u2"}, Mode: "async", CreatedAt: time.Now()}
	if err := s.UpsertMatch(ctx, m); err != nil {
		t.Fatal(err)
	}
	m2, err := s.GetMatch(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(m2.Players) != 2 || m2.Mode != "async" {
		t.Errorf("Match round-trip: %+v", m2)
	}
}

func TestListUserMatchesOrderedByCreatedAt(t *testing.T) {
	s := mustStore(t)
	ctx := context.Background()
	t0 := time.Now()
	for i, ts := range []time.Time{t0, t0.Add(time.Hour), t0.Add(2 * time.Hour), t0.Add(3 * time.Hour)} {
		m := store.Match{
			ID:        string(rune('a' + i)),
			Players:   []string{"u1", "u2"},
			Mode:      "async",
			CreatedAt: ts,
		}
		if err := s.UpsertMatch(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	// Match for a different user — must not appear.
	if err := s.UpsertMatch(ctx, store.Match{ID: "z", Players: []string{"u9"}, CreatedAt: t0.Add(10 * time.Hour)}); err != nil {
		t.Fatal(err)
	}

	got, err := s.ListUserMatches(ctx, "u1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Newest first → "d" then "c".
	if got[0].ID != "d" || got[1].ID != "c" {
		t.Errorf("order = %s,%s, want d,c", got[0].ID, got[1].ID)
	}
}

func TestSubmitScoreGTSemantics(t *testing.T) {
	s := mustStore(t)
	ctx := context.Background()
	mustSubmit := func(uid string, sc int64) {
		t.Helper()
		if err := s.SubmitScore(ctx, "lb:daily:2026-05-05", uid, sc); err != nil {
			t.Fatal(err)
		}
	}
	mustSubmit("u1", 100)
	mustSubmit("u1", 50) // lower — should NOT replace
	mustSubmit("u2", 200)
	mustSubmit("u3", 150)

	top, err := s.TopN(ctx, "lb:daily:2026-05-05", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 3 {
		t.Fatalf("len = %d, want 3", len(top))
	}
	if top[0].UID != "u2" || top[0].Score != 200 {
		t.Errorf("top[0] = %+v", top[0])
	}
	if top[1].UID != "u3" || top[1].Score != 150 {
		t.Errorf("top[1] = %+v", top[1])
	}
	if top[2].UID != "u1" || top[2].Score != 100 {
		t.Errorf("top[2] = %+v — GT semantics broken", top[2])
	}
}

func TestPresenceTTL(t *testing.T) {
	s := mustStore(t)
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	s.SetClock(clock.Now)

	ctx := context.Background()
	if err := s.MarkOnline(ctx, "u1", 60*time.Second); err != nil {
		t.Fatal(err)
	}
	on, err := s.IsOnline(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("expected online immediately after MarkOnline")
	}

	clock.now = clock.now.Add(61 * time.Second)
	on, err = s.IsOnline(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("expected offline after TTL expiry")
	}
}

func TestCacheTTL(t *testing.T) {
	s := mustStore(t)
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	s.SetClock(clock.Now)
	ctx := context.Background()

	if err := s.CacheSet(ctx, "k", []byte("v"), 10*time.Second); err != nil {
		t.Fatal(err)
	}
	v, ok, err := s.CacheGet(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || string(v) != "v" {
		t.Fatalf("CacheGet = (%q, %v)", v, ok)
	}
	clock.now = clock.now.Add(11 * time.Second)
	_, ok, err = s.CacheGet(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected expiry")
	}
}

func TestExportEmitsJSONLPerCollection(t *testing.T) {
	s := mustStore(t)
	ctx := context.Background()

	if _, err := s.UpsertUserOnFirstAuth(ctx, store.AuthClaims{UID: "u1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.PutPuzzle(ctx, store.Puzzle{Date: "2026-05-05", Word: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertAttempt(ctx, store.Attempt{UID: "u1", PuzzleDate: "2026-05-05"}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMatch(ctx, store.Match{ID: "m1", Players: []string{"u1"}}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := s.Export(ctx, &buf); err != nil {
		t.Fatal(err)
	}

	dec := json.NewDecoder(&buf)
	collections := map[string]int{}
	for dec.More() {
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			t.Fatal(err)
		}
		c, _ := row["collection"].(string)
		collections[c]++
	}
	for _, want := range []string{"users", "puzzles", "attempts", "matches"} {
		if collections[want] != 1 {
			t.Errorf("collection %s count = %d, want 1; full counts = %v", want, collections[want], collections)
		}
	}
}

func TestClosePreventsAllOps(t *testing.T) {
	s := memstore.New()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetUser(context.Background(), "u1")
	if !errors.Is(err, store.ErrClosed) {
		t.Errorf("GetUser err = %v, want ErrClosed", err)
	}
	if err := s.Ping(context.Background()); !errors.Is(err, store.ErrClosed) {
		t.Errorf("Ping err = %v, want ErrClosed", err)
	}
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }
