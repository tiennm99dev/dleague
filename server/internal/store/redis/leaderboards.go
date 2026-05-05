package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/tiennm99/dleague/server/internal/store"
)

// Cap top entries per board to bound memory. Redis ZSETs are tiny so this
// can easily be bumped if leaderboards grow.
const topCap = 1000

// SubmitScore records uid's score on `board` only if it's strictly higher
// than the existing score (ZADD GT). Caller-provided board name is treated
// as a Redis key — typical: "lb:daily:2026-05-05" or "lb:global:alltime".
func (c *Client) SubmitScore(ctx context.Context, board, uid string, score int64) error {
	if c == nil || c.c == nil {
		return store.ErrClosed
	}
	if err := c.c.ZAddArgs(ctx, board, goredis.ZAddArgs{
		GT:      true,
		Members: []goredis.Z{{Score: float64(score), Member: uid}},
	}).Err(); err != nil {
		return fmt.Errorf("redis: zadd: %w", err)
	}
	// Trim to top `topCap` to bound memory; leaves indexes 0..topCap-1
	// (highest scores) and removes the rest.
	if err := c.c.ZRemRangeByRank(ctx, board, 0, -int64(topCap)-1).Err(); err != nil {
		return fmt.Errorf("redis: zremrangebyrank: %w", err)
	}
	return nil
}

// TopN returns up to n highest-scoring members of board, descending.
func (c *Client) TopN(ctx context.Context, board string, n int) ([]store.Rank, error) {
	if c == nil || c.c == nil {
		return nil, store.ErrClosed
	}
	if n <= 0 {
		return nil, nil
	}
	rows, err := c.c.ZRevRangeWithScores(ctx, board, 0, int64(n-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: zrevrange: %w", err)
	}
	out := make([]store.Rank, 0, len(rows))
	for _, r := range rows {
		uid, _ := r.Member.(string)
		out = append(out, store.Rank{UID: uid, Score: int64(r.Score)})
	}
	return out, nil
}
