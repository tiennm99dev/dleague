---
title: "Code review: MongoDB Atlas migration implementation"
type: code-review
date: 2026-05-07
plan: plans/260507-1648-mongodb-atlas-only-migration/
red_team_input: plans/reports/code-reviewer-260507-1648-mongodb-plan-redteam.md
reviewer_role: staff-engineer
---

# Code review — MongoDB Atlas migration (impl pass)

**Verdict:** Implementation faithfully matches the patched plan. All BLOCKING / HIGH items from the original red-team are reflected in code. Two MEDIUM findings, one LOW. No BLOCKING issues. `go build` + `go vet` clean.

## Top findings

### MEDIUM-1 — `UpsertUserOnFirstAuth` skips `withTimeout`, can hang on Atlas stalls
**File:** `server/internal/store/mongodb/users.go:22-55`

Every other persistent-store method in the package wraps its caller `ctx` with `withTimeout(ctx)` (defaults to 5s if no deadline). `UpsertUserOnFirstAuth` does not. First-auth is on the WS upgrade hot path (`auth.NewGate(verifier, st)` in `cmd/api/main.go:53`) — a stalled Atlas mid-`FindOneAndUpdate` blocks the WS handshake for whatever the upstream caller's deadline is. If a caller passes `context.Background()` (e.g. a future REST endpoint), the handshake hangs until TCP timeout.

**Change:** add `ctx, cancel := withTimeout(ctx); defer cancel()` to the top of the function, mirroring `GetUser` / `TouchLastSeen`.

### MEDIUM-2 — Test isolation: claimed DB drop is not implemented
**File:** `server/internal/store/mongodb/mongodb_test.go:18-19, 42`

Doc comment claims: *"isolates itself by writing to MONGODB_TEST_DB ... and dropping that database at the end of each test."* Reality: `t.Cleanup` only calls `_ = c.Close()`. No `db.Drop(ctx)` anywhere. Consequences:
- Tests that don't randomize keys (e.g. `TestSubmitScore_GTSemantics` uses `lb-test-{HHMMSS.SSS}` — racey at sub-ms) will collide on rerun.
- `dleague_test` accumulates documents indefinitely → cluster bloat on the M0 quota. Plan red-team Q2 already flagged this.
- The unique compound index on `attempts.{uid, puzzleDate}` will start raising `E11000` on test reruns that happen to land on the same generated UID.

**Change:** in `newTestClient`, add a `t.Cleanup` that drops the test database before `Close`:
```go
t.Cleanup(func() {
    dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = c.Database().Drop(dropCtx) // expose c.Database() or use a helper
    _ = c.Close()
})
```
`*Client` doesn't currently expose `*mongo.Database`. Two options: (a) add `func (c *Client) dropForTests(ctx) error` gated under `_test.go` build, or (b) restructure to a `TestMain` that drops once before+after the suite. Option (b) is what the red-team Q2 actually recommended.

### LOW-1 — `ListUserMatches` index ESR ordering is fine but worth a code comment
**File:** `server/internal/store/mongodb/indexes.go:55-61`

Index is `{players:1, createdAt:-1}`. Query is `find({players: uid}).sort({createdAt:-1}).limit(n)`. `players` is multikey (array), used as leading equality — Mongo will use the index for both filter and sort. Correct, but multikey + sort is a footgun pattern; future maintainers may try to flip the order.

**Change:** add one-line comment in `indexes.go` near the matches index: "leading equality on multikey `players` so the index can serve both the filter and the `createdAt` sort without an in-memory sort."

## Behavioral checklist (all green except above)

| Item | Result |
|---|---|
| Concurrency: `$max` + per-doc atomicity for `SubmitScore` | OK; `TestSubmitScore_ConcurrentSameUID` proves it |
| `MarkOnline` per-doc serialization | OK; `TestMarkOnline_ConcurrentSameUID` proves it |
| `UpdateOne` over `ReplaceOne` for presence (red-team MEDIUM-2) | OK, `presence.go:27-31` |
| TTL query-side filter on every read | OK, `presence.go:54`, `cache.go:35-37` |
| `CacheSet(ttl<=0)` → `$unset expireAt` (red-team HIGH-2) | OK, `cache.go:63-67`; `TestCacheSet_ZeroTTL_NoExpiry` proves the round-trip |
| `CacheGet` filter accepts docs missing `expireAt` | OK, `$or` with `$exists:false` at `cache.go:33-38` |
| `SubmitScore` bare `$max`, no `updatedAt`, no `$setOnInsert` (red-team HIGH-3 + MED-1) | OK, `leaderboards.go:27` |
| `Match.ID` / `Puzzle.Date` → `bson:"_id"` | OK, `store.go:78,97` |
| Compile-time `var _ store.Store = (*Client)(nil)` | OK, `client.go:90` |
| `SetServerSelectionTimeout(5s)` (red-team MED-4) | OK, `client.go:45` |
| `defaultOpTimeout = 5s` per-op cap | OK, `client.go:20` (applied via `withTimeout` everywhere except the user method flagged above) |
| `defer cur.Close(ctx)` on every cursor | OK; `matches.go:74`, `leaderboards.go:64`, `export.go:64-79` (manual close on every branch) |
| `cur.Err()` checked after iteration | OK, all three list/range methods |
| `bson:"_id"` driving primary-key reads | OK, `puzzles.go:26`, `matches.go:27`, `presence.go:28,53`, `cache.go:33,78` |
| TTL field is `time.Time` (BSON Date) per red-team HIGH-1 | OK; `presence.go:29` and `cache.go:73` use `time.Now().UTC().Add(ttl)`. Code comments at `indexes.go:31-34` and `presence.go:18` document the requirement. |
| Driver isolation: no `gocb` / `go-redis` anywhere in `server/` | Clean (grep confirms; only one residual mention is the comment in `redteam.md`, irrelevant) |
| `go.mongodb.org/mongo-driver/v2` only in `internal/store/mongodb/` | Clean; `make grep-isolation` target is wired (Makefile:72-79) |
| Build / vet | `go build ./server/...` clean, `go vet ./server/...` clean |
| Config: `MONGODB_URI` required, `MONGODB_DB` defaults `dleague`, Couchbase / Redis fields removed | OK, `config.go:33-38, 77-79`; `config_test.go` covers all three required-env cases |
| `cmd/atlas-smoke` wires `mongodb.New` + `Ping` only | OK, `atlas-smoke/main.go:36-44` |
| Single-backend wiring in `cmd/api/main.go` | OK, no `composed` / `redis` / `couchbase` references |

## Tests

13 tests in `mongodb_test.go`, all gated by `MONGODB_TEST_URI`:
- `TestSubmitScore_GTSemantics` — proves `$max` ignores lower scores. Solid.
- `TestSubmitScore_ConcurrentSameUID` — 10 goroutines, asserts max wins. Proves doc-level write atomicity.
- `TestIsOnline_AccurateBeforeAndAfterTTL` — re-marks with 1s TTL, sleeps 2s, asserts query-side filter masks pre-purge state. Exactly the red-team's "load-bearing pattern" assertion.
- `TestCacheSet_ZeroTTL_NoExpiry` — proves `$unset` flip back to TTL works. Exactly what red-team HIGH-2 required.
- `TestUpsertUserOnFirstAuth_Idempotent` — proves `$setOnInsert` immutability + `$set` mutation contract.
- `TestUpsertAttempt_Replace`, `TestPuzzleRoundTrip`, `TestListUserMatches_OrderAndLimit`, `TestTopN_OrderAndLimit`, `TestMarkOnline_ConcurrentSameUID`, `TestCacheRoundTrip_TTL`, `TestPing`, `TestGetUser_NotFound`, `TestGetPuzzle_NotFound` — all meaningful.

**Test-quality note:** there's no `TestExport`. `Export` covers 4 collections in series, has manual cursor cleanup on every branch (`export.go:67, 71, 76, 79`), but no behavioral test verifies the JSONL wire shape stays compatible with whatever pre-existing importer reads it. Plan Phase 6 was skipped ("no data deployed yet"), so this is acceptable for now, but flag for a follow-up if/when migration data lands.

## Plan-vs-impl deviations

None of substance. Two cosmetic deltas:
- Plan pseudocode uses `bson.M{"score": -1}` for sort; impl uses `bson.D{{Key:"score", Value:-1}}` (`leaderboards.go:51-52, 66`). `bson.D` is the correct choice for sort/index specs (preserves order); `bson.M` is unordered Go-map and only happens to work for single-field. Impl is **stricter than plan** — good.
- Plan uses `time.Now().Add(ttl)`; impl uses `time.Now().UTC().Add(ttl)` everywhere. Stricter, correct.

## What the plan-level review couldn't catch (and didn't)

- `UpsertUserOnFirstAuth` missing `withTimeout` (above) — only visible at code level.
- Test cleanup gap (above) — comment vs reality.
- All other items checked — clean.

## Unresolved questions

1. Do we want to expose `func (c *Client) Database() *mongo.Database` (or a test-only `dropForTests`) so test cleanup can actually drop the test DB? Pure refactor; gated on red-team Q2.
2. Should `UpsertUserOnFirstAuth` return `ErrNotFound` semantics anywhere, or is the upsert-always contract intentional? Today there's no path that returns `ErrNotFound` from this method, which is correct for upsert — just confirming the contract is "always returns a populated `User` or wraps a driver error."
3. `Export` has no test. Is the wire-shape compatibility with the legacy Couchbase JSONL importer still load-bearing now that Phase 6 is skipped? If migration is permanently deferred, simplify or delete `Export`.
4. `M0 connection pool` (red-team meta-note): driver default pool is 100; Atlas M0 cap is 500. Single Coolify replica is fine. If we ever scale to 2+ replicas for the API, we should add `options.Client().SetMaxPoolSize(...)` to stay under the cluster cap. Not a current bug.
5. `ListUserMatches` index serves the query, but with a multikey leading field. Worth confirming via `db.matches.find({...}).explain()` against a populated test cluster that the planner picks `IXSCAN` not `COLLSCAN` once data lands.
