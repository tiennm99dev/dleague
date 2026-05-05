// Package couchbase implements the persistent half of store.Store on top of
// Couchbase Server 8.0 via gocb v2. The package is the ONLY place gocb is
// allowed to be imported (see plan.md "migration-friendly design"); a CI grep
// guards the boundary.
//
// Concrete behavior the cache half (store/redis) does NOT cover lives here:
// users, puzzles, attempts, matches, and Export. Cache + leaderboard methods
// on this struct return ErrUnsupported — the composed store routes those to
// the Redis impl.
package couchbase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/couchbase/gocb/v2"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Op timeout — keeps a stale cluster from blocking handlers.
const defaultOpTimeout = 5 * time.Second

// Config bundles the connect-time inputs.
type Config struct {
	ConnString string
	Username   string
	Password   string
	Bucket     string
}

// Client owns the cluster + collection handles.
type Client struct {
	cluster  *gocb.Cluster
	bucket   *gocb.Bucket
	users    *gocb.Collection
	puzzles  *gocb.Collection
	attempts *gocb.Collection
	matches  *gocb.Collection
	bucketID string
}

// ErrUnsupported is returned by cache/leaderboard methods on the Couchbase
// client — those are routed to Redis via the composed store.
var ErrUnsupported = errors.New("couchbase: operation not supported on this client; use composed store")

// New connects to the cluster, waits for KV + Query services, and resolves
// the four collection handles under `<bucket>._default.{users,…}`.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ConnString == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("couchbase: ConnString and Bucket required")
	}

	cluster, err := gocb.Connect(cfg.ConnString, gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: cfg.Username,
			Password: cfg.Password,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("couchbase: connect: %w", err)
	}

	bucket := cluster.Bucket(cfg.Bucket)
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := bucket.WaitUntilReady(timeoutFor(waitCtx, 30*time.Second), &gocb.WaitUntilReadyOptions{
		ServiceTypes: []gocb.ServiceType{gocb.ServiceTypeKeyValue, gocb.ServiceTypeQuery},
	}); err != nil {
		_ = cluster.Close(nil)
		return nil, fmt.Errorf("couchbase: wait ready: %w", err)
	}

	scope := bucket.DefaultScope()
	c := &Client{
		cluster:  cluster,
		bucket:   bucket,
		users:    scope.Collection("users"),
		puzzles:  scope.Collection("puzzles"),
		attempts: scope.Collection("attempts"),
		matches:  scope.Collection("matches"),
		bucketID: cfg.Bucket,
	}
	return c, nil
}

// Ping verifies cluster reachability.
func (c *Client) Ping(_ context.Context) error {
	if c == nil || c.cluster == nil {
		return store.ErrClosed
	}
	_, err := c.cluster.Ping(&gocb.PingOptions{
		ServiceTypes: []gocb.ServiceType{gocb.ServiceTypeKeyValue},
		Timeout:      defaultOpTimeout,
	})
	return err
}

// Close releases the cluster handle.
func (c *Client) Close() error {
	if c == nil || c.cluster == nil {
		return nil
	}
	cl := c.cluster
	c.cluster = nil
	return cl.Close(nil)
}

// timeoutFor returns the deadline implied by ctx, falling back to fallback.
// gocb's WaitUntilReady wants a duration, not a context.
func timeoutFor(ctx context.Context, fallback time.Duration) time.Duration {
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 {
			return d
		}
	}
	return fallback
}

// Cache + leaderboard ops are not implemented on this client.

func (c *Client) SubmitScore(context.Context, string, string, int64) error {
	return ErrUnsupported
}
func (c *Client) TopN(context.Context, string, int) ([]store.Rank, error) {
	return nil, ErrUnsupported
}
func (c *Client) MarkOnline(context.Context, string, time.Duration) error { return ErrUnsupported }
func (c *Client) IsOnline(context.Context, string) (bool, error)          { return false, ErrUnsupported }
func (c *Client) CacheGet(context.Context, string) ([]byte, bool, error) {
	return nil, false, ErrUnsupported
}
func (c *Client) CacheSet(context.Context, string, []byte, time.Duration) error {
	return ErrUnsupported
}
