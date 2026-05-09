package store_test

// Mongo-gated: set MONGO_TEST_URI to run against a real MongoDB.
// Without it all tests skip cleanly via mongoTestURI (defined in mongo_test.go).

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

func TestMatchCreate_GetByShareToken(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewMatchRepo(client.Database())

	matchID, token, err := repo.Create(ctx, store.Match{
		GameID:        "wordle",
		ChallengerUID: "uid-challenger",
		Seed:          42,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if matchID == "" || token == "" {
		t.Fatal("expected non-empty matchID and token")
	}

	m, err := repo.GetByShareToken(ctx, token)
	if err != nil {
		t.Fatalf("GetByShareToken: %v", err)
	}
	if m == nil {
		t.Fatal("expected match, got nil")
	}
	if m.State != "pending" {
		t.Errorf("state=%q, want pending", m.State)
	}
	if m.Mode != "async" {
		t.Errorf("mode=%q, want async", m.Mode)
	}
	if m.ShareToken != token {
		t.Errorf("share_token mismatch: got %q, want %q", m.ShareToken, token)
	}

	// Non-existent token must return nil, nil.
	missing, err := repo.GetByShareToken(ctx, "no-such-token")
	if err != nil {
		t.Fatalf("GetByShareToken (missing): %v", err)
	}
	if missing != nil {
		t.Error("expected nil for missing token, got non-nil")
	}
}

func TestJoinAsChallengee_ConcurrentRace(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewMatchRepo(client.Database())

	_, token, err := repo.Create(ctx, store.Match{
		GameID:        "wordle",
		ChallengerUID: "uid-A",
		Seed:          99,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Two goroutines race to join; exactly one must succeed.
	type result struct{ err error }
	ch := make(chan result, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		uid := "uid-joiner-" + string(rune('1'+i))
		go func(u string) {
			defer wg.Done()
			_, joinErr := repo.JoinAsChallengee(ctx, token, u)
			ch <- result{joinErr}
		}(uid)
	}
	wg.Wait()
	close(ch)

	wins := 0
	for r := range ch {
		if r.err == nil {
			wins++
		} else if !errors.Is(r.err, store.ErrAlreadyJoined) {
			t.Errorf("unexpected error: %v", r.err)
		}
	}
	if wins != 1 {
		t.Errorf("expected exactly 1 successful join, got %d", wins)
	}
}

func TestSweepExpired(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewMatchRepo(client.Database())

	// SweepExpired on a clean DB must not error.
	if err := repo.SweepExpired(ctx); err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
}
