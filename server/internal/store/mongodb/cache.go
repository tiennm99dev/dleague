package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/dleague/server/internal/store"
)

// CacheGet returns (val, true, nil) on hit, (nil, false, nil) on miss.
//
// The filter accepts docs missing `expireAt` (parity with Redis SET / memstore
// "no expiry" semantics — see CacheSet with ttl<=0). Same query-side TTL
// guard as IsOnline: never return a doc whose expireAt is past.
func (c *Client) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	if c == nil || c.db == nil {
		return nil, false, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var doc struct {
		Val      []byte     `bson:"val"`
		ExpireAt *time.Time `bson:"expireAt,omitempty"`
	}
	filter := bson.M{
		"_id": key,
		"$or": bson.A{
			bson.M{"expireAt": bson.M{"$gt": time.Now().UTC()}},
			bson.M{"expireAt": bson.M{"$exists": false}},
		},
	}
	err := c.db.Collection(collCache).
		FindOne(ctx, filter).
		Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("mongodb: cache get: %w", err)
	}
	return doc.Val, true, nil
}

// CacheSet stores val for key with the given TTL. ttl<=0 is treated as
// "store forever" (parity with Redis SET without EX, and with memstore).
// In that case we $unset any prior expireAt so a key reset to "no expiry"
// is not silently inheriting a stale TTL.
func (c *Client) CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var update bson.M
	if ttl <= 0 {
		update = bson.M{
			"$set":   bson.M{"val": val},
			"$unset": bson.M{"expireAt": ""},
		}
	} else {
		update = bson.M{
			"$set": bson.M{
				"val":      val,
				"expireAt": time.Now().UTC().Add(ttl),
			},
		}
	}

	_, err := c.db.Collection(collCache).
		UpdateOne(ctx, bson.M{"_id": key}, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("mongodb: cache set: %w", err)
	}
	return nil
}
