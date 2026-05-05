package couchbase

import (
	"context"
	"errors"
	"fmt"

	"github.com/couchbase/gocb/v2"

	"github.com/tiennm99/dleague/server/internal/store"
)

func attemptKey(uid, date string) string { return uid + "::" + date }

func (c *Client) GetAttempt(ctx context.Context, uid, date string) (store.Attempt, error) {
	if c == nil || c.attempts == nil {
		return store.Attempt{}, store.ErrClosed
	}
	res, err := c.attempts.Get(attemptKey(uid, date), &gocb.GetOptions{Context: ctx, Timeout: defaultOpTimeout})
	if err != nil {
		if errors.Is(err, gocb.ErrDocumentNotFound) {
			return store.Attempt{}, store.ErrNotFound
		}
		return store.Attempt{}, fmt.Errorf("couchbase: attempt get: %w", err)
	}
	var a store.Attempt
	if err := res.Content(&a); err != nil {
		return store.Attempt{}, fmt.Errorf("couchbase: attempt decode: %w", err)
	}
	return a, nil
}

func (c *Client) UpsertAttempt(ctx context.Context, a store.Attempt) error {
	if c == nil || c.attempts == nil {
		return store.ErrClosed
	}
	if _, err := c.attempts.Upsert(attemptKey(a.UID, a.PuzzleDate), a, &gocb.UpsertOptions{
		Context: ctx,
		Timeout: defaultOpTimeout,
	}); err != nil {
		return fmt.Errorf("couchbase: attempt upsert: %w", err)
	}
	return nil
}
