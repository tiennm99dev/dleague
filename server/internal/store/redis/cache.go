package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/tiennm99/dleague/server/internal/store"
)

// CacheGet returns (value, true, nil) on hit, (nil, false, nil) on miss.
// Errors only propagate when Redis itself is unhappy.
func (c *Client) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	if c == nil || c.c == nil {
		return nil, false, store.ErrClosed
	}
	v, err := c.c.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("redis: get: %w", err)
	}
	return v, true, nil
}

// CacheSet stores val with TTL (zero TTL = no expiry).
func (c *Client) CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	if c == nil || c.c == nil {
		return store.ErrClosed
	}
	if err := c.c.Set(ctx, key, val, ttl).Err(); err != nil {
		return fmt.Errorf("redis: set: %w", err)
	}
	return nil
}
