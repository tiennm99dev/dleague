package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

// mongoTestURI returns the test URI or skips the test when not set.
// Gate: MONGO_TEST_URI must be set (e.g. mongodb://localhost:27017/dleague_test).
func mongoTestURI(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI not set; skipping MongoDB integration test")
	}
	return uri
}

func TestConnect_Ping(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			t.Errorf("Disconnect: %v", err)
		}
	}()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestEnsureIndexes(t *testing.T) {
	uri := mongoTestURI(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := store.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	db := client.Database()

	// EnsureIndexes must be idempotent — run twice, no error on second call.
	if err := store.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes (first): %v", err)
	}
	if err := store.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("EnsureIndexes (second/idempotent): %v", err)
	}

	// Count explicit indexes across the 5 collections.
	// Each collection has at least the _id index plus what we create.
	// We expect 8 explicit indexes created across users/matches/attempts/daily_puzzles/leaderboards.
	collections := []string{"users", "matches", "attempts", "daily_puzzles", "leaderboards"}
	total := 0
	for _, name := range collections {
		cursor, err := db.Collection(name).Indexes().List(ctx)
		if err != nil {
			t.Fatalf("Indexes().List(%s): %v", name, err)
		}
		var docs []interface{}
		if err := cursor.All(ctx, &docs); err != nil {
			t.Fatalf("cursor.All(%s): %v", name, err)
		}
		// subtract 1 for the implicit _id index
		explicit := len(docs) - 1
		if explicit < 0 {
			explicit = 0
		}
		total += explicit
	}

	const wantExplicit = 8
	if total != wantExplicit {
		t.Errorf("explicit indexes = %d, want %d", total, wantExplicit)
	}
}
