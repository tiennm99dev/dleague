package couchbase

import (
	"context"
	"errors"
	"fmt"

	"github.com/couchbase/gocb/v2"

	"github.com/tiennm99/dleague/server/internal/store"
)

func (c *Client) GetMatch(ctx context.Context, matchID string) (store.Match, error) {
	if c == nil || c.matches == nil {
		return store.Match{}, store.ErrClosed
	}
	res, err := c.matches.Get(matchID, &gocb.GetOptions{Context: ctx, Timeout: defaultOpTimeout})
	if err != nil {
		if errors.Is(err, gocb.ErrDocumentNotFound) {
			return store.Match{}, store.ErrNotFound
		}
		return store.Match{}, fmt.Errorf("couchbase: match get: %w", err)
	}
	var m store.Match
	if err := res.Content(&m); err != nil {
		return store.Match{}, fmt.Errorf("couchbase: match decode: %w", err)
	}
	return m, nil
}

func (c *Client) UpsertMatch(ctx context.Context, m store.Match) error {
	if c == nil || c.matches == nil {
		return store.ErrClosed
	}
	if _, err := c.matches.Upsert(m.ID, m, &gocb.UpsertOptions{
		Context: ctx,
		Timeout: defaultOpTimeout,
	}); err != nil {
		return fmt.Errorf("couchbase: match upsert: %w", err)
	}
	return nil
}

// ListUserMatches uses N1QL to find matches where the player array contains
// uid, ordered newest first. Requires a primary index on the matches
// collection (Phase 1 creates it). Add a secondary index on the players
// array if measured latency is unacceptable.
func (c *Client) ListUserMatches(ctx context.Context, uid string, n int) ([]store.Match, error) {
	if c == nil || c.cluster == nil {
		return nil, store.ErrClosed
	}
	stmt := fmt.Sprintf(
		"SELECT m.* FROM `%s`.`_default`.`matches` m "+
			"WHERE ANY p IN m.players SATISFIES p = $uid END "+
			"ORDER BY m.createdAt DESC LIMIT $n",
		c.bucketID,
	)
	rows, err := c.cluster.Query(stmt, &gocb.QueryOptions{
		Context:              ctx,
		NamedParameters:      map[string]any{"uid": uid, "n": n},
		Adhoc:                true,
		Timeout:              defaultOpTimeout,
		ScanConsistency:      gocb.QueryScanConsistencyRequestPlus,
	})
	if err != nil {
		return nil, fmt.Errorf("couchbase: list user matches: %w", err)
	}
	defer rows.Close()

	var out []store.Match
	for rows.Next() {
		var m store.Match
		if err := rows.Row(&m); err != nil {
			return nil, fmt.Errorf("couchbase: list user matches row: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, gocb.ErrTimeout) {
		return nil, fmt.Errorf("couchbase: list user matches finalize: %w", err)
	}
	return out, nil
}
