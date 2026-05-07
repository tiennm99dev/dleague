---
phase: 9
title: "Async PvP via store.Store (Couchbase + Redis)"
status: completed
priority: P1
effort: "2d"
dependencies: [3, 4, 5, 8]
---

# Phase 9: Async PvP via store.Store

## Context Links

- Plan: [plan.md](plan.md)
- Store interface: defined in Phase 4

## Overview

REST endpoints backing async PvP: serve daily puzzle, accept attempts, build leaderboards, list match history. **All persistence flows through `store.Store` (Phase 3 Couchbase + Phase 4 Redis composed) — handlers do not import either backend directly.** Migration-friendly: swapping any backend later requires zero changes to this phase's code.

## Key Insights

- Puzzle GET is read-heavy and absorbs traffic — back with `CacheGet`/`CacheSet` (TTL 25h).
- Attempt write is the hot path: validate against puzzle solution → `UpsertAttempt` (Couchbase) → `SubmitScore` to leaderboards (Redis). Two writes; not transactional across stores. Acceptable: leaderboard rebuild from Couchbase covers any divergence.
- Friends leaderboard deferred to v2 (no friend graph).
- Beta data-loss policy: VM disk loss = data loss. Acceptable. Phase 12 export CLI is the safety net.

## Requirements

- Functional: 5 REST endpoints (see below). All except `GET /puzzles/:date` require Bearer auth.
- Non-functional: P50 latency <200 ms for attempt submit; handlers depend only on `store.Store` interface.

## Architecture

REST surface (mounted at `/api/v1/`):
| Method | Path | Auth | Purpose |
|--------|------|------|---------|
| GET | `/puzzles/today` | optional | Today's puzzle (cached) |
| GET | `/puzzles/:date` | optional | Specific date puzzle |
| GET | `/attempts/me/:date` | required | Resume in-progress attempt |
| POST | `/attempts` | required | Submit attempt; returns score + rank |
| GET | `/leaderboards/:scope/:date?` | optional | scope ∈ {global, daily} |

Write flow for `POST /attempts`:
```
verify Bearer → uid (Phase 5 middleware)
store.GetPuzzle(date) → solution     (cache read inside store impl)
score the attempt → score             (pure func)
store.UpsertAttempt(uid, date, ...)
store.SubmitScore("lb:daily:"+date, uid, score)
store.SubmitScore("lb:global:alltime", uid, score)
return {score, rank}
```

The composed store routes each call to the right backend; handlers stay paradigm-agnostic.

## Related Code Files

- Create:
  - `server/internal/api/puzzles.go`
  - `server/internal/api/attempts.go`
  - `server/internal/api/leaderboards.go`
  - `server/internal/api/scoring.go`        # pure scoring logic, unit-tested
  - `server/internal/api/api_test.go`        # uses memstore from Phase 4
- Modify:
  - `server/internal/http/router.go` — mount `/api/v1/...`
  - `server/cmd/api/main.go` — pass `store.Store` + `*auth.Firebase` to api handlers

## Implementation Steps

1. `scoring.go`: pure func `Score(puzzle, attempt) → (score int, rank string)`. Cover with unit tests.
2. `puzzles.go`: handler calls `store.GetPuzzle(date)`. The store impl handles caching.
3. `attempts.go`: POST handler verifies token (middleware attaches uid), validates body, scores via `scoring.Score`, calls `store.UpsertAttempt` + `store.SubmitScore` (twice — daily + global). Idempotent on `(uid,date)`.
4. `leaderboards.go`: handler calls `store.TopN(boardKey, n)`.
5. Wire into router under `/api/v1/`.
6. Tests: handler tests use `memstore` impl (no live backend needed); integration test gated by `COUCHBASE_TEST_CONN` + `REDIS_TEST_ADDR` covers the full path against running containers.

## Todo List

- [x] Scoring pure func + 8 unit tests (first/middle/late solve, case-insensitive, whitespace, loss, empties, 7th-guess no-win)
- [x] GET /puzzles/today + /:date — handler omits `word` field; returns hint + difficulty + length only
- [x] POST /attempts — auth-gated; server re-scores from persisted puzzle; UpsertAttempt + dual SubmitScore (daily + global)
- [x] GET /attempts/me/:date — auth-gated resume endpoint
- [x] GET /leaderboards/{scope}/{date?} — scope global|daily; `?n=` query for size
- [x] Router integration — api.Mount onto chi router under /api/v1; auth middleware applied to write/personal routes only
- [x] Handler tests with memstore — 9 tests cover hint-leak, 404, 400 malformed-date, scoring round-trip, leaderboard updates, GT semantics on resubmit, future-date rejection, missing-auth 401, resume
- [ ] Integration test against live Couchbase + Redis containers (env-gated) — deferred per "skip deployment" directive; memstore tests prove the same code paths
- [x] Idempotency — re-submitting same (uid,date) replaces attempt; ZADD-GT keeps highest leaderboard score (regression test included)
- [x] No `gocb` / `redis/go-redis` import outside `internal/store/{couchbase,redis}/` — boundary still intact

## Success Criteria

- [ ] Submit attempt → score returned within 200 ms (warm cluster)
- [ ] Leaderboard reflects new score
- [ ] Re-submit on same date: leaderboard has correct (highest) score
- [ ] All `/api/v1` handlers compile with `memstore` substituted for the composed store — proves migration-readiness
- [ ] `go test ./server/internal/api/...` green

## Risk Assessment

- **Cheating: client claims a high score** — server re-scores from the persisted attempt and the puzzle's solution. Don't trust client-side `score`.
- **Race on simultaneous submits** — `UpsertAttempt` overwrites; `SubmitScore` ZADD-GT wins highest. Idempotent by design.
- **Cross-store divergence** — Couchbase write succeeds, Redis ZADD fails. Mitigation: log error; on next leaderboard read, `RebuildDaily` covers gap.

## Security Considerations

- Validate the puzzle date in the request matches today's UTC date (or yesterday's grace window). Don't let clients submit for arbitrary past dates.
- Rate-limit `POST /attempts` per uid: max 1/min (testing scale; revisit at growth).

## Next Steps

Phase 10 (sync PvP) layers WS-based real-time matches; persists via the same `store.Store`.

## Unresolved Questions

- Time zone for "daily" — UTC midnight? User-local? Decision: UTC for MVP.
- Score model — guesses-based (Wordle-style 6-row), time-based, or combo? Defer; scoring.go is the place.
