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

// GetPuzzle returns ErrNotFound when the date has no puzzle stored.
// Puzzle.Date is mapped to bson `_id` so the lookup is by primary key.
func (c *Client) GetPuzzle(ctx context.Context, date string) (store.Puzzle, error) {
	if c == nil || c.db == nil {
		return store.Puzzle{}, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var p store.Puzzle
	err := c.db.Collection(collPuzzles).
		FindOne(ctx, bson.M{"_id": date}).
		Decode(&p)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Puzzle{}, store.ErrNotFound
	}
	if err != nil {
		return store.Puzzle{}, fmt.Errorf("mongodb: puzzle get: %w", err)
	}
	return p, nil
}

// PutPuzzle replaces (upsert) the puzzle for p.Date. The Puzzle struct's
// bson:"_id" tag on Date drives the document key.
func (c *Client) PutPuzzle(ctx context.Context, p store.Puzzle) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := c.db.Collection(collPuzzles).
		ReplaceOne(ctx, bson.M{"_id": p.Date}, p, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongodb: puzzle upsert: %w", err)
	}
	return nil
}
