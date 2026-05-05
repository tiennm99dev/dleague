package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/tiennm99/dleague/server/internal/store"
)

func presenceKey(uid string) string { return "presence:" + uid }

// MarkOnline records uid as online with TTL. Repeated calls refresh TTL.
func (c *Client) MarkOnline(ctx context.Context, uid string, ttl time.Duration) error {
	if c == nil || c.c == nil {
		return store.ErrClosed
	}
	if err := c.c.Set(ctx, presenceKey(uid), 1, ttl).Err(); err != nil {
		return fmt.Errorf("redis: presence set: %w", err)
	}
	return nil
}

// IsOnline returns whether the presence key has not yet expired.
func (c *Client) IsOnline(ctx context.Context, uid string) (bool, error) {
	if c == nil || c.c == nil {
		return false, store.ErrClosed
	}
	n, err := c.c.Exists(ctx, presenceKey(uid)).Result()
	if err != nil {
		return false, fmt.Errorf("redis: presence exists: %w", err)
	}
	return n > 0, nil
}
