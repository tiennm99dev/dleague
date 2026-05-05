// Package redis implements the cache + leaderboard half of store.Store on
// go-redis v9. The package is the ONLY place go-redis is allowed to be
// imported (mirrors the gocb boundary in the couchbase package).
//
// Persistent ops (users / puzzles / attempts / matches / Export) return
// ErrUnsupported here; the composed store routes those to Couchbase.
package redis

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/tiennm99/dleague/server/internal/store"
)

const defaultOpTimeout = 3 * time.Second

// Config bundles the connect-time inputs.
type Config struct {
	Addr     string
	Password string
	PoolSize int
}

// Client wraps a go-redis client. Use New to construct.
type Client struct {
	c *goredis.Client
}

// ErrUnsupported is returned for persistent ops on this client. The composed
// store routes those calls to the Couchbase impl.
var ErrUnsupported = errors.New("redis: operation not supported on this client; use composed store")

// New connects + pings.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("redis: Addr required")
	}
	pool := cfg.PoolSize
	if pool <= 0 {
		pool = 10
	}
	c := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		PoolSize: pool,
	})
	pingCtx, cancel := context.WithTimeout(ctx, defaultOpTimeout)
	defer cancel()
	if err := c.Ping(pingCtx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return &Client{c: c}, nil
}

// NewFromClient lets tests inject a pre-built go-redis client (e.g. miniredis).
func NewFromClient(c *goredis.Client) *Client { return &Client{c: c} }

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.c == nil {
		return store.ErrClosed
	}
	return c.c.Ping(ctx).Err()
}

func (c *Client) Close() error {
	if c == nil || c.c == nil {
		return nil
	}
	cl := c.c
	c.c = nil
	return cl.Close()
}

// ─── unsupported (persistent) ops — composed store will route past this ───

func (c *Client) UpsertUserOnFirstAuth(context.Context, store.AuthClaims) (store.User, error) {
	return store.User{}, ErrUnsupported
}
func (c *Client) GetUser(context.Context, string) (store.User, error) {
	return store.User{}, ErrUnsupported
}
func (c *Client) TouchLastSeen(context.Context, string, time.Time) error    { return ErrUnsupported }
func (c *Client) GetPuzzle(context.Context, string) (store.Puzzle, error)   { return store.Puzzle{}, ErrUnsupported }
func (c *Client) PutPuzzle(context.Context, store.Puzzle) error             { return ErrUnsupported }
func (c *Client) GetAttempt(context.Context, string, string) (store.Attempt, error) {
	return store.Attempt{}, ErrUnsupported
}
func (c *Client) UpsertAttempt(context.Context, store.Attempt) error { return ErrUnsupported }
func (c *Client) GetMatch(context.Context, string) (store.Match, error) {
	return store.Match{}, ErrUnsupported
}
func (c *Client) UpsertMatch(context.Context, store.Match) error { return ErrUnsupported }
func (c *Client) ListUserMatches(context.Context, string, int) ([]store.Match, error) {
	return nil, ErrUnsupported
}
func (c *Client) Export(context.Context, io.Writer) error { return ErrUnsupported }
