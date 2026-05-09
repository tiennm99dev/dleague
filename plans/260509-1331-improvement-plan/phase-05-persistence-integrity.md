# Phase 05 — Persistence & data integrity

## Context Links
- [Server review](reports/code-reviewer-server-260509-1331.md) — H4, H8, H9, M1, M2, M3, L1
- Depends on: Phase 01 (the unique-index + state-filter fixes)

## Overview
- **Priority:** P1
- **Status:** pending
- **Description:** Solidify persistence boundary: enforce DB-level uniqueness, add defensive state filters, document Atlas tier expectations, decide `Attempt.Hints` future, audit idempotency, harden leaderboard refresh path. Phase 01 lands the must-fix index; this phase is the surrounding cleanup.

## Key Insights
- `server/internal/store/matches.go:96-100` `JoinAsChallengee` filter relies on `$exists:false` + app-level enforcement — no DB unique on `(_id, challengee_uid)`; under heavy concurrency on sharded clusters, race window opens (server H4).
- `server/internal/store/leaderboards.go:48-192` `Refresh` decodes all attempts of all matches every 5 min via `cur.All` — O(N) memory spike (server M2). 10K matches × 2 attempts = 20K docs.
- `server/internal/store/mongo.go:76-82` `parseDBName` ignores URI parse errors silently → defaults to "dleague" on malformed URI (server M3).
- `server/internal/store/models.go:83` `Attempt.Hints` field exists but is never written by any code path (server M1).
- `server/internal/store/matches.go:136` `Complete` (async) doesn't filter on `state:"pending"` — covered by Phase 01 step 8 but L1 is the broader audit (server L1).
- README.md:52 claims "Atlas M0" — `mongo.go:23-24` pool config (`100 max`, `10 min`) may be too aggressive for M0's 500-conn cap (server unresolved Q1).

## Requirements
- All conditional updates that mutate state (match → complete, attempt insert) carry both filter (`state:"pending"`) and unique-index defence.
- Boot fails on malformed `MONGO_URI` rather than silent fallback.
- `Attempt.Hints` either populated correctly or removed from the model.
- Leaderboard refresh has bounded memory regardless of match volume — solution may be a docs-only "MVP threshold acknowledged" note, OR an aggregation pipeline rewrite (decide based on near-term scale; default: doc + threshold guard).
- Atlas tier expectations documented next to the Mongo pool config.

## Related Code Files
**Modify**
- `server/internal/store/matches.go` (filter audit; CompleteSync mirror)
- `server/internal/store/attempts.go` (state filter on insert if applicable)
- `server/internal/store/leaderboards.go` (threshold guard or aggregation)
- `server/internal/store/mongo.go` (fail-fast on URI parse)
- `server/internal/store/models.go` (Hints decision; comment if kept)
- `server/internal/store/indexes.go` (any other unique-index gaps)
- `server/internal/ws/match_handler.go` (Hints population if kept)
- `docs/system-architecture.md` (Atlas tier + pool config note)
- `docs/codebase-summary.md` (Atlas + Hints field note)

**Delete** — possibly `Attempt.Hints` field if unused.

## Implementation Steps

### State filters + unique audits
1. Audit every `*.UpdateOne`/`FindOneAndUpdate` in `server/internal/store/`. For each that transitions state, verify filter includes the source state. Specifically check:
   - `matches.go:CompleteSync` (already filters on `state:"active"` — confirm).
   - `matches.go:Complete` (Phase 01 step 8 adds `state:"pending"` — confirm).
   - `matches.go:JoinAsChallengee` (filter has `$exists:false` — keep but **add `state:"pending"`** for defence; server H4).
   - `users.go:IncrementStats` (no state machine; OK).
2. Add a `// MUST: filter on source state to prevent double-resolve` comment above each.
3. `server/internal/store/indexes.go` — beyond Phase 01's compound-unique on attempts, audit:
   - `matches` `share_token` partial-filter unique (already exists per server review; confirm).
   - `users` `uid` unique (verify).
   - `daily_seeds` `(date, game_id)` unique (verify).
   - Add any missing.

### Boot hygiene
4. `server/internal/store/mongo.go:76-82` `parseDBName` — change to return `(string, error)`. On parse failure, return error; caller in `Connect` propagates to `main.go` boot. Document: "Boot fails fast on malformed MONGO_URI rather than silent default."

### Leaderboard refresh
5. `server/internal/store/leaderboards.go:48-192` — add a hard threshold check at top of `Refresh`: query `db.matches.countDocuments({state:"complete", date})`; if > 5000, log WARN and return `ErrLeaderboardTooLarge` (sentinel). Scheduler logs and continues. Documents the scaling boundary explicitly.
6. Add comment block above `Refresh` explaining: "Current implementation: full re-decode every 5 min. Acceptable up to ~5000 matches/day. Scale-out path: aggregation pipeline `$lookup` + `$sort` + `$limit`. See server review M2."
7. (Optional, defer if low priority) Replace `cur.All` with a streaming reduce-into-heap (top-100). Defer unless near-term scale demands.

### Hints field decision
8. **Decision: drop the field.** Rationale: zero callers, replay-from-guesses is computable on-demand if anti-cheat ever needs it (re-run `wordle.Score` over `attempt.guesses[]`).
9. `server/internal/store/models.go:83` — delete `Hints []*WordleHint` field. Update any references (none expected per review).
10. `make compile-server` (or `go build ./...`) — green.

### Atlas docs
11. `docs/system-architecture.md` — add a "Persistence — Atlas" subsection documenting:
    - Free tier M0: 500 cluster-wide conns, 100 per Atlas user.
    - Current pool (`mongo.go:23-24`): max 100, min 10. Sufficient for M0 if single instance.
    - On scale-up, raise to 200/20.
12. `docs/codebase-summary.md` — one-line note that Atlas M0 limits are documented in system-architecture.md.

## Todo List
- [ ] State-filter audit + comments across `store/` (steps 1-2)
- [ ] Unique-index audit (step 3)
- [ ] `parseDBName` fail-fast (step 4)
- [ ] Leaderboard threshold guard + comment (steps 5-6)
- [ ] Drop `Attempt.Hints` field (steps 8-10)
- [ ] Atlas tier docs (steps 11-12)

## Success Criteria
- `grep -rn "UpdateOne\|FindOneAndUpdate" server/internal/store/*.go` — every state-mutating call has a state filter or a "// not state-machine" comment.
- Boot with `MONGO_URI=garbage` fails before listening.
- `db.attempts.findOne({hints: {$exists: true}})` returns 0 docs after redeploy (or migration is no-op since field was never written).
- Leaderboard refresh on a synthetic 6000-match day → WARN logged, no OOM, no panic.
- `docs/system-architecture.md` has explicit Atlas tier subsection.

## Risk Assessment
- **Dropping Hints field:** if any prod migration tooling expects the field, it'll fail. Mitigation: field is `omitempty` and never set, so dropping the Go struct field is purely a code change; existing docs without the field decode fine. No data migration needed.
- **JoinAsChallengee state filter:** if existing documents have `state` other than `"pending"` due to older code, joins fail. Mitigation: confirm via grep that all `Create` paths set `state:"pending"`; existing prod data is empty (no deploy yet).
- **Threshold guard breaks leaderboard at scale:** intentional — surfaces a known limit early rather than silent O(N) memory growth.

## Security Considerations
- Unique indexes prevent duplicate-attempt stat inflation (covered by Phase 01).
- Fail-fast on URI prevents accidental connections to wrong DB in misconfigured envs.

## Next Steps
- Phase 06 adds tests for state-filter behaviour (concurrent join attempts, double-complete attempts).
- v2 backlog: leaderboard aggregation pipeline if scale crosses threshold.
