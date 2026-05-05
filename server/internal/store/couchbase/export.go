package couchbase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/couchbase/gocb/v2"
)

// Export streams every persistent doc as JSONL to w. One row per doc:
// `{"collection":"<name>","doc":<json>}`. Reuses the row-prefix shape from
// the memstore impl so the future importer is collection-agnostic.
func (c *Client) Export(ctx context.Context, w io.Writer) error {
	if c == nil || c.cluster == nil {
		return fmt.Errorf("couchbase: client closed")
	}
	enc := json.NewEncoder(w)
	for _, collection := range []string{"users", "puzzles", "attempts", "matches"} {
		stmt := fmt.Sprintf(
			"SELECT t.* FROM `%s`.`_default`.`%s` t",
			c.bucketID, collection,
		)
		rows, err := c.cluster.Query(stmt, &gocb.QueryOptions{
			Context: ctx,
			Adhoc:   true,
			Timeout: 30 * defaultOpTimeout,
		})
		if err != nil {
			return fmt.Errorf("couchbase: export query %s: %w", collection, err)
		}
		for rows.Next() {
			var doc map[string]any
			if err := rows.Row(&doc); err != nil {
				_ = rows.Close()
				return fmt.Errorf("couchbase: export row %s: %w", collection, err)
			}
			if err := enc.Encode(map[string]any{"collection": collection, "doc": doc}); err != nil {
				_ = rows.Close()
				return fmt.Errorf("couchbase: export encode %s: %w", collection, err)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return fmt.Errorf("couchbase: export finalize %s: %w", collection, err)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("couchbase: export close %s: %w", collection, err)
		}
	}
	return nil
}
