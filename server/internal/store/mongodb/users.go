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

// UpsertUserOnFirstAuth stamps beta-tester provenance only on first write.
//
// Strategy: a single findOneAndUpdate with upsert. `$setOnInsert` carries the
// fields we want frozen forever (IsBetaTester, BetaSignupAt, CreatedAt). `$set`
// carries the fields we refresh on every auth (Email, DisplayName, Provider,
// LastSeen). ReturnDocument(After) gives us the post-update doc atomically.
func (c *Client) UpsertUserOnFirstAuth(ctx context.Context, claims store.AuthClaims) (store.User, error) {
	if c == nil || c.db == nil {
		return store.User{}, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	now := time.Now().UTC()

	filter := bson.M{"uid": claims.UID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"uid":          claims.UID,
			"isBetaTester": true,
			"betaSignupAt": now,
			"createdAt":    now,
		},
		"$set": bson.M{
			"email":       claims.Email,
			"displayName": claims.DisplayName,
			"provider":    claims.Provider,
			"lastSeen":    now,
		},
	}
	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var u store.User
	err := c.db.Collection(collUsers).
		FindOneAndUpdate(ctx, filter, update, opts).
		Decode(&u)
	if err != nil {
		return store.User{}, fmt.Errorf("mongodb: user upsert: %w", err)
	}
	return u, nil
}

// GetUser returns ErrNotFound when no doc exists.
func (c *Client) GetUser(ctx context.Context, uid string) (store.User, error) {
	if c == nil || c.db == nil {
		return store.User{}, store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	var u store.User
	err := c.db.Collection(collUsers).
		FindOne(ctx, bson.M{"uid": uid}).
		Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("mongodb: user get: %w", err)
	}
	return u, nil
}

// TouchLastSeen updates only the lastSeen field. Returns ErrNotFound if no
// user exists for uid (caller is expected to UpsertUserOnFirstAuth first).
func (c *Client) TouchLastSeen(ctx context.Context, uid string, at time.Time) error {
	if c == nil || c.db == nil {
		return store.ErrClosed
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	res, err := c.db.Collection(collUsers).
		UpdateOne(ctx,
			bson.M{"uid": uid},
			bson.M{"$set": bson.M{"lastSeen": at.UTC()}},
		)
	if err != nil {
		return fmt.Errorf("mongodb: touch last seen: %w", err)
	}
	if res.MatchedCount == 0 {
		return store.ErrNotFound
	}
	return nil
}

// withTimeout adds the package default timeout if ctx has none.
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultOpTimeout)
}
