package mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Export streams every persistent doc as JSONL to w. One row per doc:
// `{"collection":"<name>","doc":<json>}`. Matches the wire shape produced
// by the previous Couchbase-backed export, so any importer that reads
// the predecessor JSONL also reads this output.
func (c *Client) Export(ctx context.Context, w io.Writer) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	enc := json.NewEncoder(w)

	pairs := []struct {
		collection string
		decode     func(raw bson.Raw) (any, error)
	}{
		{collUsers, func(raw bson.Raw) (any, error) {
			var u store.User
			if err := bson.Unmarshal(raw, &u); err != nil {
				return nil, err
			}
			return u, nil
		}},
		{collPuzzles, func(raw bson.Raw) (any, error) {
			var p store.Puzzle
			if err := bson.Unmarshal(raw, &p); err != nil {
				return nil, err
			}
			return p, nil
		}},
		{collAttempts, func(raw bson.Raw) (any, error) {
			var a store.Attempt
			if err := bson.Unmarshal(raw, &a); err != nil {
				return nil, err
			}
			return a, nil
		}},
		{collMatches, func(raw bson.Raw) (any, error) {
			var m store.Match
			if err := bson.Unmarshal(raw, &m); err != nil {
				return nil, err
			}
			return m, nil
		}},
	}

	for _, pair := range pairs {
		coll := c.db.Collection(pair.collection)
		cur, err := coll.Find(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("mongodb: export find %s: %w", pair.collection, err)
		}
		for cur.Next(ctx) {
			doc, err := pair.decode(cur.Current)
			if err != nil {
				_ = cur.Close(ctx)
				return fmt.Errorf("mongodb: export decode %s: %w", pair.collection, err)
			}
			if err := enc.Encode(map[string]any{"collection": pair.collection, "doc": doc}); err != nil {
				_ = cur.Close(ctx)
				return fmt.Errorf("mongodb: export encode %s: %w", pair.collection, err)
			}
		}
		if err := cur.Err(); err != nil {
			_ = cur.Close(ctx)
			return fmt.Errorf("mongodb: export cursor %s: %w", pair.collection, err)
		}
		if err := cur.Close(ctx); err != nil {
			return fmt.Errorf("mongodb: export close %s: %w", pair.collection, err)
		}
	}
	return nil
}
