package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

func TestLeaderboardRefreshAndGet(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewLeaderboardRepo(client.Database())
	date := time.Now().UTC().Format("2006-01-02")

	// Refresh on a DB with no matches should produce an empty rankings doc.
	if err := repo.Refresh(ctx, "wordle", "daily", date); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	lb, err := repo.Get(ctx, "wordle", "daily", date)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if lb == nil {
		t.Fatal("expected leaderboard doc, got nil")
	}
	if lb.GameID != "wordle" {
		t.Errorf("game_id=%q, want wordle", lb.GameID)
	}
	if lb.Period != "daily" {
		t.Errorf("period=%q, want daily", lb.Period)
	}
	// Rankings may be empty (no completed matches) but slice must be non-nil.
	if lb.Rankings == nil {
		t.Error("rankings must not be nil (may be empty slice)")
	}
}

func TestLeaderboardGet_Missing(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewLeaderboardRepo(client.Database())

	// A date far in the future should have no doc yet.
	lb, err := repo.Get(ctx, "wordle", "daily", "2099-12-31")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if lb != nil {
		t.Error("expected nil for non-existent leaderboard, got non-nil")
	}
}
