package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/dleague/server/internal/store"
)

// MarkOnline records uid as online with TTL. Repeated calls refresh TTL.
//
// updateOne+$set (rather than replaceOne) so future fields on the doc are
// not silently dropped on every heartbeat. The expireAt field MUST be a
// time.Time — driver encodes that as BSON Date and TTL only fires on Date.
func (c *Client) MarkOnline(ctx context.Context, uid string, ttl time.Duration) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, err := c.db.Collection(collPresence).
		UpdateOne(ctx,
			bson.M{"_id": uid},
			bson.M{"$set": bson.M{"expireAt": time.Now().UTC().Add(ttl)}},
			options.UpdateOne().SetUpsert(true),
		)
	if err != nil {
		return fmt.Errorf("mongodb: mark online: %w", err)
	}
	return nil
}

// IsOnline returns true iff a non-expired presence doc exists for uid.
//
// The `expireAt: {$gt: now()}` filter is load-bearing — Mongo's TTL purge
// runs on a ~60s background scan, so a doc may physically exist for up to
// ~90s past its logical expiry. Filtering at query time guarantees an
// accurate liveness answer regardless of physical purge timing.
func (c *Client) IsOnline(ctx context.Context, uid string) (bool, error) {
	if c == nil || c.db == nil {
		return false, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	n, err := c.db.Collection(collPresence).
		CountDocuments(ctx, bson.M{
			"_id":      uid,
			"expireAt": bson.M{"$gt": time.Now().UTC()},
		})
	if err != nil {
		return false, fmt.Errorf("mongodb: is online: %w", err)
	}
	return n > 0, nil
}
