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

// GetMatch returns the match by ID or ErrNotFound. Match.ID is mapped to
// bson `_id`, so the lookup is by primary key.
func (c *Client) GetMatch(ctx context.Context, matchID string) (store.Match, error) {
	if c == nil || c.db == nil {
		return store.Match{}, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var m store.Match
	err := c.db.Collection(collMatches).
		FindOne(ctx, bson.M{"_id": matchID}).
		Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.Match{}, store.ErrNotFound
	}
	if err != nil {
		return store.Match{}, fmt.Errorf("mongodb: match get: %w", err)
	}
	return m, nil
}

// UpsertMatch replaces (upsert) the match doc keyed by m.ID.
func (c *Client) UpsertMatch(ctx context.Context, m store.Match) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := c.db.Collection(collMatches).
		ReplaceOne(ctx, bson.M{"_id": m.ID}, m, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongodb: match upsert: %w", err)
	}
	return nil
}

// ListUserMatches finds matches whose `players` array contains uid, ordered
// newest first. Backed by the (players, createdAt:-1) compound index.
func (c *Client) ListUserMatches(ctx context.Context, uid string, n int) ([]store.Match, error) {
	if c == nil || c.db == nil {
		return nil, store.ErrClosed
	}
	if n <= 0 {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(n))

	cur, err := c.db.Collection(collMatches).
		Find(ctx, bson.M{"players": uid}, opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: list user matches: %w", err)
	}
	defer cur.Close(ctx)

	out := make([]store.Match, 0, n)
	for cur.Next(ctx) {
		var m store.Match
		if err := cur.Decode(&m); err != nil {
			return nil, fmt.Errorf("mongodb: list user matches decode: %w", err)
		}
		out = append(out, m)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("mongodb: list user matches cursor: %w", err)
	}
	return out, nil
}
