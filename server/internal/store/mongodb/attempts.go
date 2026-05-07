package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/dleague/server/internal/store"
)

// GetAttempt returns the user's run on the given date or ErrNotFound.
// Lookup uses the (uid, puzzleDate) compound unique index.
func (c *Client) GetAttempt(ctx context.Context, uid, date string) (store.Attempt, error) {
	if c == nil || c.db == nil {
		return store.Attempt{}, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var a store.Attempt
	err := c.db.Collection(collAttempts).
		FindOne(ctx, bson.M{"uid": uid, "puzzleDate": date}).
		Decode(&a)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Attempt{}, store.ErrNotFound
	}
	if err != nil {
		return store.Attempt{}, fmt.Errorf("mongodb: attempt get: %w", err)
	}
	return a, nil
}

// UpsertAttempt replaces the attempt doc for (uid, puzzleDate). The unique
// compound index from indexes.go enforces the natural key.
func (c *Client) UpsertAttempt(ctx context.Context, a store.Attempt) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	filter := bson.M{"uid": a.UID, "puzzleDate": a.PuzzleDate}
	_, err := c.db.Collection(collAttempts).
		ReplaceOne(ctx, filter, a, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongodb: attempt upsert: %w", err)
	}
	return nil
}
