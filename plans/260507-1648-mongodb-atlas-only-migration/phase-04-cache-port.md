---
phase: 4
title: "Cache half port (leaderboards, presence, cache)"
status: completed
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 4: Cache half port (leaderboards, presence, cache)

## Context links

- Research: [`plans/reports/researcher-260507-1650-mongodb-replaces-redis.md`](../reports/researcher-260507-1650-mongodb-replaces-redis.md) — see §1, §2, §3 for the patterns.
- `Store` interface methods 36–45: [`server/internal/store/store.go`](../../server/internal/store/store.go).
- Redis reference impls: `server/internal/store/redis/{leaderboards,presence,cache}.go`.

## Overview

Port the cache/leaderboard/presence half of `store.Store` (6 methods) to MongoDB. The non-trivial parts are: (a) using `$max` to replicate Redis ZADD-GT atomicity; (b) every TTL read includes `expireAt: {$gt: now()}` to mask the up-to-60s background-purge lag.

## Requirements

**Functional — implement against `*mongodb.Client`:**

| Store method | Mongo collection | Mongo op |
|---|---|---|
| `SubmitScore(board, uid, score) error` | `leaderboards` | `updateOne({board, uid}, {$max:{score}}, upsert)` |
| `TopN(board, n) ([]Rank, error)` | `leaderboards` | `find({board}).sort({score:-1}).limit(n)`, project `{uid, score}` |
| `MarkOnline(uid, ttl) error` | `presence` | `updateOne({_id:uid}, {$set:{expireAt: now+ttl}}, upsert)` |
| `IsOnline(uid) (bool, error)` | `presence` | `countDocuments({_id:uid, expireAt:{$gt:now}})` |
| `CacheGet(key) ([]byte, bool, error)` | `cache` | `findOne({_id:key, $or:[{expireAt:{$gt:now}}, {expireAt:{$exists:false}}]})` |
| `CacheSet(key, val, ttl) error` | `cache` | `updateOne({_id:key}, {$set:{val, expireAt: now+ttl}}, upsert)` — **special case `ttl <= 0`**: omit `expireAt` and `$unset` any existing one (parity with Redis/memstore "no expiry"). |

**Non-functional:**
- All TTL reads include `expireAt: {$gt: now()}` filter — *non-negotiable*. Document this in code comments + `migration-readiness.md` rewrite (Phase 7).
- `expireAt` field MUST be a `time.Time` (BSON Date). String / epoch-int values are silently retained forever — TTL only fires on Date-typed fields.
- `CacheSet(ttl=0)` parity: writes `val` with no `expireAt` (and `$unset` any existing one). `CacheGet` filter explicitly tolerates docs missing `expireAt`.
- `SubmitScore` does **not** write `updatedAt` (or any other timestamp). Avoids unbounded write/index churn from no-op scores. If we ever need a "last write" probe, add a separate `firstSeenAt` via `$setOnInsert` only.
- No `$setOnInsert: {board, uid}` in `SubmitScore` — Mongo auto-seeds equality fields from the filter on upsert insert.
- `topCap` (Redis trim-to-1000) is **dropped**. Storage is cheap; index makes top-N reads fast regardless of doc count.
- `SubmitScore` is single-shot atomic (no read-modify-write loop needed).

## Architecture

```
internal/store/mongodb/
├── ... (Phase 2 + 3 files)
├── leaderboards.go        ← Phase 4
├── presence.go            ← Phase 4
├── cache.go               ← Phase 4
└── mongodb_test.go        (extended in Phase 4)
```

## Related code files

- **Create:** `server/internal/store/mongodb/leaderboards.go`, `presence.go`, `cache.go`.
- **Modify:** `server/internal/store/mongodb/mongodb_test.go` — add 6 method tests + the TTL workaround test.
- **No changes to:** redis impl, composed, memstore (yet).

## Implementation steps

1. **`leaderboards.go`:**
   ```go
   func (c *Client) SubmitScore(ctx context.Context, board, uid string, score int64) error {
       coll := c.db.Collection("leaderboards")
       filter := bson.M{"board": board, "uid": uid}
       // $max replicates Redis ZADD GT: only updates if new > existing.
       // Filter equality fields (board, uid) auto-seed on upsert insert; no $setOnInsert needed.
       // No timestamp written — avoids index churn on no-op scores.
       update := bson.M{"$max": bson.M{"score": score}}
       _, err := coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
       if err != nil { return fmt.Errorf("mongodb: submitScore: %w", err) }
       return nil
   }
   func (c *Client) TopN(ctx context.Context, board string, n int) ([]store.Rank, error) {
       if n <= 0 { return nil, nil }
       coll := c.db.Collection("leaderboards")
       opts := options.Find().SetSort(bson.M{"score": -1}).SetLimit(int64(n)).
           SetProjection(bson.M{"uid": 1, "score": 1, "_id": 0})
       cur, err := coll.Find(ctx, bson.M{"board": board}, opts)
       if err != nil { return nil, fmt.Errorf("mongodb: topN: %w", err) }
       defer cur.Close(ctx)
       var rows []store.Rank
       if err := cur.All(ctx, &rows); err != nil {
           return nil, fmt.Errorf("mongodb: topN decode: %w", err)
       }
       return rows, nil
   }
   ```
2. **`presence.go`:**
   ```go
   func (c *Client) MarkOnline(ctx context.Context, uid string, ttl time.Duration) error {
       coll := c.db.Collection("presence")
       // updateOne+$set (not replaceOne) so future fields on the doc are not silently dropped.
       _, err := coll.UpdateOne(ctx, bson.M{"_id": uid},
           bson.M{"$set": bson.M{"expireAt": time.Now().Add(ttl)}},
           options.UpdateOne().SetUpsert(true))
       if err != nil { return fmt.Errorf("mongodb: markOnline: %w", err) }
       return nil
   }
   // IsOnline: query-side filter masks TTL purge lag (~60s background scan).
   func (c *Client) IsOnline(ctx context.Context, uid string) (bool, error) {
       coll := c.db.Collection("presence")
       n, err := coll.CountDocuments(ctx,
           bson.M{"_id": uid, "expireAt": bson.M{"$gt": time.Now()}})
       if err != nil { return false, fmt.Errorf("mongodb: isOnline: %w", err) }
       return n > 0, nil
   }
   ```
3. **`cache.go`:**
   ```go
   func (c *Client) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
       coll := c.db.Collection("cache")
       var doc struct {
           Val      []byte     `bson:"val"`
           ExpireAt *time.Time `bson:"expireAt,omitempty"`
       }
       // Tolerate docs missing expireAt (set via CacheSet(ttl<=0) — parity with Redis "no expiry").
       filter := bson.M{
           "_id": key,
           "$or": bson.A{
               bson.M{"expireAt": bson.M{"$gt": time.Now()}},
               bson.M{"expireAt": bson.M{"$exists": false}},
           },
       }
       err := coll.FindOne(ctx, filter).Decode(&doc)
       if errors.Is(err, mongo.ErrNoDocuments) { return nil, false, nil }
       if err != nil { return nil, false, fmt.Errorf("mongodb: cacheGet: %w", err) }
       return doc.Val, true, nil
   }
   func (c *Client) CacheSet(ctx context.Context, key string, val []byte, ttl time.Duration) error {
       coll := c.db.Collection("cache")
       var update bson.M
       if ttl <= 0 {
           // Parity with Redis SET (no EX) and memstore: store forever.
           update = bson.M{
               "$set":   bson.M{"val": val},
               "$unset": bson.M{"expireAt": ""},
           }
       } else {
           update = bson.M{"$set": bson.M{"val": val, "expireAt": time.Now().Add(ttl)}}
       }
       _, err := coll.UpdateOne(ctx, bson.M{"_id": key}, update, options.UpdateOne().SetUpsert(true))
       if err != nil { return fmt.Errorf("mongodb: cacheSet: %w", err) }
       return nil
   }
   ```
4. **Tests:**
   - `TestSubmitScore_GTSemantics` — submit score=10, then score=5; assert read still shows 10. Then submit score=20; assert reads 20.
   - `TestTopN_OrderAndLimit` — submit 5 scores, assert TopN(3) returns highest 3 in desc order.
   - `TestSubmitScore_ConcurrentSameUID` — spawn 10 goroutines each submitting an increasing score; assert final score is the max submitted.
   - `TestIsOnline_AccurateBeforeTTL` — `MarkOnline(uid, 30s)`; assert `IsOnline` true.
   - `TestIsOnline_FalseAfterTTL` — `MarkOnline(uid, 1s)`; sleep 2s; assert `IsOnline` false (relies on query-side filter, not physical purge — passes even if doc still physically exists).
   - `TestMarkOnline_ConcurrentSameUID` — 10 goroutines call `MarkOnline(uid, mixedTTLs)` simultaneously; assert no error and `IsOnline` returns true (proves doc-level write serialization is graceful, not a panic source).
   - `TestCacheRoundtrip_TTL` — `CacheSet(k, v, 1s)`; `CacheGet(k)` → hit; sleep 2s; `CacheGet(k)` → miss.
   - `TestCacheSet_ZeroTTL_NoExpiry` — `CacheSet(k, v, 0)`; sleep 2s; `CacheGet(k)` → still hit. Then `CacheSet(k, v, 1s)`; sleep 2s; `CacheGet(k)` → miss. (Proves `$unset` path on the no-expiry case + correct switching back to TTL.)
5. **Run integration tests against Atlas:**
   ```sh
   MONGODB_TEST_URI=... go test -tags=integration -count=1 ./server/internal/store/mongodb/... -run 'TestSubmitScore|TestTopN|TestIsOnline|TestCache'
   ```

## Todo list

- [ ] `leaderboards.go` (2 methods) + 3 tests
- [ ] `presence.go` (2 methods) + 2 tests
- [ ] `cache.go` (2 methods) + 1 test
- [ ] All TTL-reading methods include `expireAt: {$gt: now()}` filter
- [ ] Concurrent-submit test passes (proves $max atomicity)
- [ ] `go vet`, `golangci-lint`, `go test` all green

## Success criteria

- `TestSubmitScore_ConcurrentSameUID` passes with no flake across 10+ runs (proves single-doc write atomicity).
- `TestIsOnline_FalseAfterTTL` passes within 2s, **before** the TTL background scan would run — proving the query-side filter works (this is the load-bearing pattern for the whole consolidation).
- Existing `internal/api/*` tests, when reconfigured to use `mongodb` impl instead of `redis`, pass with no semantic change.

## Risk assessment

- **Forgot `expireAt: {$gt: now()}` on a read.** Highest-impact bug class — silent stale reads. Mitigation: code review, plus `TestIsOnline_FalseAfterTTL` and `TestCacheRoundtrip_TTL` deliberately probe at sub-purge intervals.
- **`$max` with non-numeric type.** Mongo `$max` works on `int64` numerics; if `score` is ever changed to a `decimal128`, audit. Currently always `int64`. ✓
- **`$setOnInsert` collision with index.** When `{board, uid}` doesn't yet exist, the upsert inserts a new doc with `board` + `uid` from `$setOnInsert`. The unique compound index from Phase 2 prevents accidental duplicate docs. ✓
- **`time.Now()` precision.** Mongo BSON Date is millisecond-precision. `time.Time.UnixMilli()` round-trip is lossless. Sub-millisecond TTLs are not supported but our minimum TTL is 30s — irrelevant.
- **Cursor allocation pressure on TopN.** TopN allocates a slice. Set initial capacity to `n` to avoid grow-by-2x churn. Already in pseudocode.

## Security considerations

- Cache values may contain user-scoped JSON; no encryption-at-rest beyond Atlas's default. Acceptable for beta.
- Presence collection contains UIDs — same exposure as Redis presence. No regression.

## Next steps

After Phase 3 + 4 both complete, Phase 5 swaps the wiring in `cmd/api/main.go`.
