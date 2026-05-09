package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/tiennm99/dleague/server/internal/store"
)

func TestAttemptInsert_ListByMatch(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewAttemptRepo(client.Database())

	matchOID := bson.NewObjectID()
	a := store.Attempt{
		MatchID:   matchOID,
		PlayerUID: "uid-player-1",
		Guesses:   []string{"SLATE", "CRANE", "ROUTE"},
		TimeMs:    12000,
		Won:       true,
		Mode:      "async",
	}

	if err := repo.Insert(ctx, a); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Duplicate insert must return ErrAttemptExists.
	dupErr := repo.Insert(ctx, a)
	if !errors.Is(dupErr, store.ErrAttemptExists) {
		t.Errorf("second Insert: want ErrAttemptExists, got %v", dupErr)
	}

	list, err := repo.ListByMatch(ctx, matchOID.Hex())
	if err != nil {
		t.Fatalf("ListByMatch: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("len(list)=%d, want 1", len(list))
	}
	if list[0].PlayerUID != a.PlayerUID {
		t.Errorf("player_uid=%q, want %q", list[0].PlayerUID, a.PlayerUID)
	}
	if len(list[0].Guesses) != 3 {
		t.Errorf("guesses len=%d, want 3", len(list[0].Guesses))
	}
}

func TestGetByMatchAndPlayer_NotFound(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	repo := store.NewAttemptRepo(client.Database())

	// Non-existent (match_id, player_uid) pair must return nil, nil.
	got, err := repo.GetByMatchAndPlayer(ctx, bson.NewObjectID().Hex(), "ghost-uid")
	if err != nil {
		t.Fatalf("GetByMatchAndPlayer: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing attempt, got non-nil")
	}
}
