// Package store provides MongoDB-backed repositories for dleague.
//
// One *Client per process is created at boot and shared across all handlers.
// Atlas connections use mongodb+srv:// URIs with TLS handled transparently.
// Local dev uses mongodb://user:pass@localhost:27017/?authSource=admin.
package store

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// defaultDB is the MongoDB database name used by all repos.
	defaultDB = "dleague"

	// Pool defaults tuned for Atlas M0 (max 500 conns; we leave headroom).
	poolMaxSize = 100
	poolMinSize = 10

	// poolMaxIdleTime: connections unused longer than this are dropped.
	poolMaxIdleTime = 30 * time.Second

	// connectTimeout: maximum time for initial TCP + TLS handshake.
	connectTimeout = 10 * time.Second

	// serverSelectionTimeout: topology discovery; prevents hanging on unreachable
	// cluster during boot.
	serverSelectionTimeout = 5 * time.Second
)

// Client wraps *mongo.Client and exposes lifecycle helpers.
// Atlas TLS is implicit with mongodb+srv:// — no explicit ssl=true needed.
type Client struct {
	inner  *mongo.Client
	dbName string
}

// Connect opens a connection pool against uri and returns a Client.
// ctx is reserved for future use (driver v2 Connect is synchronous; Ping
// should be called separately to verify reachability).
// The caller must call Disconnect on a non-error return.
func Connect(_ context.Context, uri string) (*Client, error) {
	if uri == "" {
		return nil, fmt.Errorf("store: empty MONGO_URI")
	}

	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(poolMaxSize).
		SetMinPoolSize(poolMinSize).
		SetMaxConnIdleTime(poolMaxIdleTime).
		SetConnectTimeout(connectTimeout).
		SetServerSelectionTimeout(serverSelectionTimeout)

	inner, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}

	dbName := parseDBName(uri)
	if dbName == "" {
		dbName = defaultDB
	}
	return &Client{inner: inner, dbName: dbName}, nil
}

// parseDBName extracts the database name from a MongoDB URI's path segment.
// Returns empty string when no path is set, deferring to defaultDB.
func parseDBName(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || len(u.Path) <= 1 {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}

// Ping verifies that the server is reachable. Called during boot to fail fast.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("store: nil client")
	}
	return c.inner.Ping(ctx, nil)
}

// Disconnect drains the connection pool. Call via defer in main.
func (c *Client) Disconnect(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return nil
	}
	return c.inner.Disconnect(ctx)
}

// Database returns the *mongo.Database resolved from the URI path
// (or defaultDB when the URI omits one). All repos use this handle.
func (c *Client) Database() *mongo.Database {
	return c.inner.Database(c.dbName)
}

// Inner exposes the raw *mongo.Client for transaction helpers (e.g. StartSession).
func (c *Client) Inner() *mongo.Client {
	return c.inner
}
