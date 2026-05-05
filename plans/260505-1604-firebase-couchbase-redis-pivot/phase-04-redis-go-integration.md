---
phase: 4
title: "Redis 8.4 Go integration (cache + leaderboards)"
status: in_progress
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 4: Redis 8.4 Go integration

## Context Links

- Plan: [plan.md](plan.md)
- go-redis: https://github.com/redis/go-redis

## Overview

`internal/store/redis` package wrapping `*redis.Client`. Implements the **cache + leaderboard half** of `store.Store` (Phase 3 owns the persistent half). `internal/store/composed/` wires both impls together so `main.go` sees one `store.Store`.

## Key Insights

- Self-hosted Redis 8.4 inside docker-compose: addr `redis:6379`, password from `REDIS_PASSWORD`. No TLS (internal network).
- AOF persistence enabled in Phase 1's compose definition — leaderboards survive container restart. **Still treat Redis as cache** (the persistent source of truth lives in Couchbase).
- go-redis v9 idioms:
  - `client.ZAdd(ctx, key, redis.Z{Score, Member})`, GT option for monotonic high-score updates
  - `client.ZRevRangeWithScores(ctx, key, 0, n-1)` for top-N
  - Pub/sub via `client.Subscribe` (kept stub-only for future multi-instance)

## Requirements

- Functional: leaderboard Submit/TopN; presence MarkOnline/IsOnline; generic cache Get/Set; rebuild-from-Couchbase helper for cold-start safety.
- Non-functional: PoolSize 10 (well within Redis defaults), 3s op timeout, no `redis` import outside this package.

## Architecture

```
internal/store/
├── redis/
│   ├── client.go         # Open/Ping/Close, conn pool, keepalive
│   ├── leaderboards.go   # SubmitScore, TopN, RebuildDaily(cb)
│   ├── presence.go       # MarkOnline, IsOnline
│   ├── cache.go          # Get/Set with TTL
│   └── redis_test.go     # gated by REDIS_TEST_ADDR
└── composed/
    └── composed.go       # store.Store impl: routes persistent ops → couchbase, ephemeral → redis
```

Key namespaces (Redis side):
- `lb:global:alltime` — global leaderboard ZSET (no TTL)
- `lb:daily:<YYYY-MM-DD>` — daily ZSET (TTL 35d)
- `lb:friends:<uid>` — per-user friends ZSET (TTL 1d, rebuild on read)
- `presence:<uid>` — STRING with EXPIRE (60s)
- `cache:puzzle:<YYYY-MM-DD>` — puzzle JSON, TTL 25h

## Related Code Files

- Create:
  - `server/internal/store/redis/{client,leaderboards,presence,cache,redis_test}.go`
  - `server/internal/store/composed/composed.go` — accepts `*couchbase.Client`, `*redis.Client`, returns `store.Store`
- Modify:
  - `server/cmd/api/main.go` — wire `composed.New(cb, rdb)` and pass result to router
  - `server/internal/http/health.go` — Redis Ping path

## Implementation Steps

1. `cd server && go get github.com/redis/go-redis/v9@latest`
2. `client.go`: `redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, PoolSize: 10})`. Ping on init. Idle keepalive ticker.
3. `leaderboards.go`:
   - `SubmitScore(ctx, board, uid, score)` → `ZADD key GT score uid`; post-write `ZREMRANGEBYRANK key 0 -1001` to cap top-1000.
   - `TopN(ctx, board, n)` → `ZRevRangeWithScores`.
   - `RebuildDaily(ctx, cb, date)` — query attempts via Couchbase, ZADD batch.
4. `presence.go`: `MarkOnline(uid, ttl)` → `SET presence:<uid> 1 EX ttl`. `IsOnline(uid)` → `EXISTS`.
5. `cache.go`: `Get(key) ([]byte, bool, error)` → `GET`; `Set(key, val, ttl)` → `SETEX`.
6. `composed.New(cb, rdb)` → returns a struct implementing `store.Store`:
   - User/Puzzle/Attempt/Match/Export → delegate to `cb`
   - SubmitScore/TopN/MarkOnline/IsOnline/CacheGet/CacheSet → delegate to `rdb`
   - `Ping(ctx)` → both
   - `Close()` → both
7. Tests: gated integration on `REDIS_TEST_ADDR`; cover all leaderboard semantics.

## Todo List

- [x] go-redis v9 added (v9.19.0); miniredis v2 added for tests
- [x] `redis.Client` New/Ping/Close (PoolSize 10 default, configurable)
- [x] Leaderboard Submit/TopN — ZADD GT semantics + ZRemRangeByRank cap to top-1000
- [ ] `RebuildDaily` from Couchbase — deferred to Phase 9 where the attempts→leaderboard mapping is consumed
- [x] Presence MarkOnline/IsOnline (SET key 1 EX ttl + EXISTS)
- [x] Generic Cache Get/Set (CacheGet returns (val, hit, err); miss → no error)
- [x] `composed.Store` glues couchbase + redis behind `store.Store`; both halves passthrough; Ping/Close fan out
- [ ] Health endpoint Pings Redis (deferred with Couchbase ping; both wire at main.go integration)
- [x] Tests green via miniredis — GT semantics, TopN ordering, presence TTL fast-forward, cache TTL fast-forward + miss

## Success Criteria

- [ ] Higher-score submission updates ZSET; lower does not (GT)
- [ ] `TopN` returns sorted descending
- [ ] Cold restart: `RebuildDaily` reconstructs leaderboard from Couchbase in <2s for 1K attempts
- [ ] `redis` import only inside `internal/store/redis/` (grep verify)
- [ ] `composed.Store` passes the same test suite as `memstore`

## Risk Assessment

- **AOF disk usage growth** — `appendonly yes` writes every command. Mitigation: `auto-aof-rewrite-percentage 100` in Redis config to compact periodically.
- **Container restart drops connections** — go-redis pool reconnects automatically.
- **Friends leaderboard memory blowup** — per-user ZSETs accumulate. Mitigation: TTL 1d on `lb:friends:<uid>`; rebuild on next read.

## Security Considerations

- Redis password mandatory (`requirepass` in compose); even on internal network — defense in depth.
- All Redis writes server-side; clients call REST/WS endpoints, never Redis directly.
- No Lua scripts (migration-friendly).

## Next Steps

Phase 9 consumes `SubmitScore`/`TopN`. Phase 10 consumes presence helpers. Phase 12 ships export CLI which delegates to `cb.Export` (Redis side has no persistent state worth exporting beyond derivable leaderboards).

## Unresolved Questions

- Pub/sub stubs needed v1? No — Go WS hub fans out fine on single instance. Add stubs only if Phase 10 actually needs them.
- Leaderboard cap policy on `lb:friends:<uid>` — top-100 or full friend list? Defer; depends on friend-graph design (post-beta).
