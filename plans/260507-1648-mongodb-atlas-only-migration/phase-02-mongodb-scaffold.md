---
phase: 2
title: "internal/store/mongodb/ scaffold"
status: completed
priority: P1
effort: "1d"
dependencies: [1]
---

# Phase 2: `internal/store/mongodb/` scaffold

## Context links

- Existing seam: [`server/internal/store/store.go`](../../server/internal/store/store.go)
- Current Couchbase impl shape (mirror it): [`server/internal/store/couchbase/client.go`](../../server/internal/store/couchbase/client.go)
- Driver docs: https://www.mongodb.com/docs/drivers/go/current/

## Overview

Create the new `internal/store/mongodb/` package with a `Client` type that will eventually implement `store.Store` (full impl ships across Phases 3+4). This phase delivers: package layout, connection lifecycle (`New`, `Ping`, `Close`), index initialization on startup, and a single integration test gated by `MONGODB_TEST_URI` (matches the gating pattern of `couchbase_test.go`).

## Requirements

**Functional:**
- `mongodb.New(ctx, cfg)` returns a `*Client` connected to a single Mongo cluster, with all collections + indexes ensured on startup.
- `Ping(ctx)` round-trips a `{ping:1}` command.
- `Close()` disconnects; idempotent.
- Integration test `mongodb_test.go` runs only when `MONGODB_TEST_URI` env var is set. Skips otherwise (matching existing pattern).

**Non-functional:**
- `gocb`-equivalent grep-isolation: only files inside `internal/store/mongodb/` may import `go.mongodb.org/mongo-driver/v2/...`. Verified in Phase 7 cleanup tests.
- Package must compile against Go 1.25.5.

## Architecture

```
server/internal/store/mongodb/
├── client.go         # Client struct, New/Ping/Close, ensureIndexes
├── indexes.go        # Index definitions (compound for leaderboards, TTL for presence/cache)
├── mongodb_test.go   # Integration test (gated by MONGODB_TEST_URI)
└── (Phase 3 + 4 add: users.go, puzzles.go, attempts.go, matches.go, export.go,
                       leaderboards.go, presence.go, cache.go)
```

## Related code files

- **Create:** `server/internal/store/mongodb/client.go` — connection lifecycle, holds `*mongo.Client` + `*mongo.Database`.
- **Create:** `server/internal/store/mongodb/indexes.go` — `ensureIndexes(ctx, db)` creates compound + TTL indexes idempotently via `coll.Indexes().CreateMany()`.
- **Create:** `server/internal/store/mongodb/mongodb_test.go` — `TestPing`, `TestEnsureIndexes`. Gated.
- **Modify:** `server/internal/config/config.go` — add `MongoURI` field; remove `CouchbaseConn`/`RedisAddr` env reads in Phase 5 (not yet).
- **Modify:** `server/go.mod` — add `go.mongodb.org/mongo-driver/v2` v2.6.x.

## Implementation steps

1. **Add the driver dependency:**
   ```sh
   cd server
   go get go.mongodb.org/mongo-driver/v2/mongo
   go mod tidy
   ```
2. **Write `client.go`:**
   ```go
   package mongodb
   import (
       "context"
       "fmt"
       "time"
       "go.mongodb.org/mongo-driver/v2/mongo"
       "go.mongodb.org/mongo-driver/v2/mongo/options"
   )
   type Config struct{ URI, Database string }
   type Client struct{ c *mongo.Client; db *mongo.Database }
   func New(ctx context.Context, cfg Config) (*Client, error) {
       opts := options.Client().
           ApplyURI(cfg.URI).
           SetServerSelectionTimeout(5 * time.Second) // fail fast when Atlas is unreachable
       cl, err := mongo.Connect(opts)
       if err != nil { return nil, fmt.Errorf("mongodb: connect: %w", err) }
       db := cl.Database(cfg.Database)
       if err := ensureIndexes(ctx, db); err != nil {
           _ = cl.Disconnect(ctx)
           return nil, fmt.Errorf("mongodb: ensure indexes: %w", err)
       }
       return &Client{c: cl, db: db}, nil
   }
   func (c *Client) Ping(ctx context.Context) error { return c.c.Ping(ctx, nil) }
   func (c *Client) Close() error { return c.c.Disconnect(context.Background()) }
   ```
3. **Write `indexes.go`:**
   - `users`: index `{uid: 1}` unique.
   - `puzzles`: `_id` is the date string, no extra indexes needed.
   - `attempts`: index `{uid: 1, puzzleDate: 1}` unique.
   - `matches`: index `{players: 1, createdAt: -1}` for `ListUserMatches`.
   - `leaderboards`: indexes `{board: 1, score: -1}` and `{board: 1, uid: 1}` (unique).
   - `presence`: TTL index `{expireAt: 1}` with `expireAfterSeconds: 0`. **The `expireAt` field MUST be inserted as a Go `time.Time` — the Mongo Go driver encodes it as BSON Date and TTL fires only on Date-typed fields. Strings or epoch ints are silently retained forever.**
   - `cache`: TTL index `{expireAt: 1}` with `expireAfterSeconds: 0`. Same Date-type rule.
   - All created via a single helper that loops a slice of `{collection, model}` and calls `Indexes().CreateMany`. Idempotent — Mongo no-ops on duplicate index spec. **Partial-failure note:** if one spec is malformed, prior indexes in the slice are still created and the bad one returns an error. Re-running `CreateMany` is safe because existing indexes are no-ops.
4. **Write `mongodb_test.go`:**
   ```go
   //go:build integration
   package mongodb
   func TestPing(t *testing.T) {
       uri := os.Getenv("MONGODB_TEST_URI")
       if uri == "" { t.Skip("MONGODB_TEST_URI not set; skipping") }
       cl, err := New(context.Background(), Config{URI: uri, Database: "dleague_test"})
       require.NoError(t, err); defer cl.Close()
       require.NoError(t, cl.Ping(context.Background()))
   }
   ```
5. **Compile + run integration test against the Atlas cluster from Phase 1:**
   ```sh
   MONGODB_TEST_URI="$(cat ~/.dleague/atlas.uri)" go test -tags=integration ./server/internal/store/mongodb/...
   ```
6. **Run the existing test suite** to confirm no other package broke (`go test ./...`).

## Todo list

- [ ] `go.mongodb.org/mongo-driver/v2` added to `server/go.mod`
- [ ] `client.go` with `New`/`Ping`/`Close`
- [ ] `indexes.go` with all 7 collections' indexes
- [ ] `mongodb_test.go` with build tag + env gate
- [ ] Integration test passes against Atlas M0
- [ ] `go test ./...` green
- [ ] `go vet ./...` clean

## Success criteria

- Running `go test -tags=integration -run TestPing ./server/internal/store/mongodb/...` with `MONGODB_TEST_URI` set returns PASS.
- Running the same without `MONGODB_TEST_URI` returns SKIP.
- `golangci-lint run ./server/internal/store/mongodb/...` clean.

## Risk assessment

- **Driver v2 API drift from blog examples.** Driver v2 (Jan 2025+) renamed/restructured packages vs v1. Verify imports against current docs page (https://www.mongodb.com/docs/drivers/go/current/quick-start/).
- **`Indexes().CreateMany` partial failures.** If one index spec is wrong, Mongo creates the rest and returns the error for the bad one. `ensureIndexes` should return early on first error to keep startup deterministic.
- **TTL index `expireAfterSeconds: 0` semantics.** Means "expire when `expireAt <= now()`", not "expire after 0 seconds from insertion". Double-check by writing a doc with `expireAt: now() - 1h` and verifying it gets purged within 60–90s.

## Security considerations

- Test DB is `dleague_test`, not `dleague` — keeps integration tests off the prod DB.
- `MONGODB_TEST_URI` is per-developer; not committed.

## Next steps

Unblocks Phases 3 + 4 (parallel). The package shape + connection + indexes are in place; the per-method ports drop into separate files.
