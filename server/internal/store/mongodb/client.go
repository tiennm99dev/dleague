// Package mongodb implements store.Store on top of MongoDB Atlas (M0+).
//
// Imports of `go.mongodb.org/mongo-driver/v2/...` are confined to this
// package; the migration seam (server/internal/store/store.go) is what every
// other package depends on. CI grep guards the boundary.
package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tiennm99/dleague/server/internal/store"
)

// defaultOpTimeout caps any single op so a stale cluster cannot block handlers.
const defaultOpTimeout = 5 * time.Second

// Config bundles the connect-time inputs.
type Config struct {
	URI      string // mongodb+srv://... including credentials
	Database string // typically "dleague"
}

// Client owns the *mongo.Client and the *mongo.Database handles. The Mongo Go
// driver pools connections internally; the *mongo.Client is safe to share.
type Client struct {
	c  *mongo.Client
	db *mongo.Database
}

// New connects to the cluster, configures a 5s server-selection timeout
// (default is 30s — too long to leave the WS hub hanging on a dead Atlas),
// and ensures every collection's indexes exist.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.URI == "" || cfg.Database == "" {
		return nil, fmt.Errorf("mongodb: URI and Database are required")
	}

	opts := options.Client().
		ApplyURI(cfg.URI).
		SetServerSelectionTimeout(5 * time.Second)

	cl, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := cl.Ping(pingCtx, nil); err != nil {
		_ = cl.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}

	db := cl.Database(cfg.Database)
	if err := ensureIndexes(ctx, db); err != nil {
		_ = cl.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ensure indexes: %w", err)
	}

	return &Client{c: cl, db: db}, nil
}

// Ping verifies the cluster is reachable.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.c == nil {
		return store.ErrClosed
	}
	return c.c.Ping(ctx, nil)
}

// Close disconnects from the cluster. Idempotent.
func (c *Client) Close() error {
	if c == nil || c.c == nil {
		return nil
	}
	cl := c.c
	c.c = nil
	c.db = nil
	return cl.Disconnect(context.Background())
}

// Compile-time assertion that *Client satisfies store.Store. If a method is
// missing from any of users.go / puzzles.go / attempts.go / matches.go /
// export.go / leaderboards.go / presence.go / cache.go, this fails to build.
var _ store.Store = (*Client)(nil)
