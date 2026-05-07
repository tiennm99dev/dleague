package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/dleague/server/internal/store"
)

// SubmitScore replicates Redis ZADD GT semantics: only update when the new
// score is strictly greater than the existing one. `$max` is atomic at the
// single-doc level. Filter equality fields (board, uid) auto-seed on upsert
// insert — no `$setOnInsert` needed. We deliberately do NOT write a
// timestamp field here: that would force an index-touching write on every
// no-op submit, eating into Atlas M0's ops/sec budget for no payoff.
func (c *Client) SubmitScore(ctx context.Context, board, uid string, score int64) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	filter := bson.M{"board": board, "uid": uid}
	update := bson.M{"$max": bson.M{"score": score}}

	_, err := c.db.Collection(collLeaderboards).
		UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongodb: submit score: %w", err)
	}
	return nil
}

// TopN returns up to n highest-scoring members of board, descending. Backed
// by the (board, score:-1) compound index. No trim: storage is cheap and
// the index keeps top-N reads fast regardless of doc count.
func (c *Client) TopN(ctx context.Context, board string, n int) ([]store.Rank, error) {
	if c == nil || c.db == nil {
		return nil, store.ErrClosed
	}
	if n <= 0 {
		return nil, nil
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	opts := options.Find().
		SetSort(bson.D{{Key: "score", Value: -1}}).
		SetLimit(int64(n)).
		SetProjection(bson.D{
			{Key: "uid", Value: 1},
			{Key: "score", Value: 1},
			{Key: "_id", Value: 0},
		})

	cur, err := c.db.Collection(collLeaderboards).
		Find(ctx, bson.M{"board": board}, opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: top N find: %w", err)
	}
	defer cur.Close(ctx)

	out := make([]store.Rank, 0, n)
	for cur.Next(ctx) {
		var r store.Rank
		if err := cur.Decode(&r); err != nil {
			return nil, fmt.Errorf("mongodb: top N decode: %w", err)
		}
		out = append(out, r)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("mongodb: top N cursor: %w", err)
	}
	return out, nil
}
