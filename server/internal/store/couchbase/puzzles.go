package couchbase

import (
	"context"
	"errors"
	"fmt"

	"github.com/couchbase/gocb/v2"

	"github.com/tiennm99/dleague/server/internal/store"
)

func (c *Client) GetPuzzle(ctx context.Context, date string) (store.Puzzle, error) {
	if c == nil || c.puzzles == nil {
		return store.Puzzle{}, store.ErrClosed
	}
	res, err := c.puzzles.Get(date, &gocb.GetOptions{Context: ctx, Timeout: defaultOpTimeout})
	if err != nil {
		if errors.Is(err, gocb.ErrDocumentNotFound) {
			return store.Puzzle{}, store.ErrNotFound
		}
		return store.Puzzle{}, fmt.Errorf("couchbase: puzzle get: %w", err)
	}
	var p store.Puzzle
	if err := res.Content(&p); err != nil {
		return store.Puzzle{}, fmt.Errorf("couchbase: puzzle decode: %w", err)
	}
	return p, nil
}

func (c *Client) PutPuzzle(ctx context.Context, p store.Puzzle) error {
	if c == nil || c.puzzles == nil {
		return store.ErrClosed
	}
	if _, err := c.puzzles.Upsert(p.Date, p, &gocb.UpsertOptions{
		Context: ctx,
		Timeout: defaultOpTimeout,
	}); err != nil {
		return fmt.Errorf("couchbase: puzzle upsert: %w", err)
	}
	return nil
}
