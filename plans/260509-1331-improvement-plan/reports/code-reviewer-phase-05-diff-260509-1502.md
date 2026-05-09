# Phase 05 diff review — persistence & data integrity

## Verdict
**APPROVE_WITH_FIXES** — spec largely implemented; one spec deviation (daily_puzzles compound unique not added), one minor accuracy issue (parseDBName doc), one threshold-rationale gap. No blocking defects. `go build/vet/test -race` clean.

## Spec compliance

| Step | Item | Status | Notes |
|---|---|---|---|
| 1 | State-filter audit (`JoinAsChallengee`, `Complete`, `CompleteSync`, `IncrementStats`) | DONE | Filters present; CompleteSync was already correct. |
| 2 | `// MUST` comments above each | DONE | Added on JoinAsChallengee:97, Complete:134, CompleteSync:203; "not a state machine" on IncrementStats:103. |
| 3 | Index audit | PARTIAL | `users.display_name` unique kept; `matches.share_token` partial-unique kept; all 9 indexes named. **Missing:** `daily_puzzles` `(date, game_id)` unique was spec-asked. See Issue D-1. |
| 4 | `parseDBName` fail-fast | DONE | Returns `(string, error)`; Connect propagates. Caveat: `url.Parse` is permissive, so this is mostly cosmetic — see Issue M-1. |
| 5 | Leaderboard threshold guard | DONE | `leaderboardMaxMatches=5000` const; `CountDocuments` at top of `Refresh`; sentinel `ErrLeaderboardTooLarge` returned + WARN logged. |
| 6 | Refresh comment block | DONE | leaderboards.go:54-55 — concise, references review M2. |
| 7 | Streaming heap | DEFERRED | Per spec, optional. |
| 8-9 | Drop `Attempt.Hints` | DONE | Field removed from `models.go:Attempt`. No callers wrote it (verified by grep). |
| 10 | Compile green | DONE | `go build/vet ./...` pass. |
| 11 | Atlas tier doc | DONE | `system-architecture.md` adds "Atlas tier requirements" subsection; M0/M10+ split clear. |
| 12 | codebase-summary pointer | DONE | One-line note added. |

## Issues

### D-1 (Medium) — daily_puzzles unique key not compound
**File:** `server/internal/store/indexes.go:100-109`
**Problem:** Spec step 3 lists `daily_puzzles (date, game_id)` unique. Current schema uses `_id = "YYYY-MM-DD"` (date string only). For MVP single-game ("wordle") this is fine, but the moment a second game is added two daily puzzles for the same date will collide on `_id`. Implementer's note ("daily_puzzles._id covers it") is correct only under the single-game assumption.
**Fix:** Either (a) document explicitly that daily_puzzles is single-game-keyed and that adding a second game requires schema migration to compound `_id`, OR (b) change `_id` shape to `"<game>_<date>"` now. Pick (a) for YAGNI; add a `// SCHEMA NOTE: when adding a second game, migrate _id to "<game>_<date>"` comment in `daily_puzzles.go` near `Upsert`.

### M-1 (Medium) — `parseDBName` fail-fast is mostly cosmetic
**File:** `server/internal/store/mongo.go:80-89`
**Problem:** `mongo.Connect(opts)` already calls `ApplyURI(uri)` which validates connection string format using the canonical mongo URI grammar (returns error for bad scheme, malformed host, etc.). By the time `parseDBName` runs, the URI has already passed driver validation. `url.Parse` is permissive and rarely fails on inputs that already satisfied the driver. So the new error path catches almost nothing the driver wouldn't have caught first.
**Impact:** Low — fail-fast intent is preserved (driver fails before us), but the "fix M3" framing is misleading.
**Fix options:**
  1. Document the redundancy: change comment at `mongo.go:77` to "Returns error on parse failure (rare; mongo.Connect's ApplyURI catches most malformed URIs first)."
  2. Use canonical parser: `go.mongodb.org/mongo-driver/v2/x/mongo/driver/connstring` — `connstring.ParseAndValidate(uri)` exposes `.Database`. This is semantically the right tool but adds an `x/` (internal-ish) import.
**Recommend option 1** (cheapest, honest).

### L-1 (Low) — leaderboard threshold magnitude undocumented
**File:** `server/internal/store/leaderboards.go:14-16`
**Problem:** `leaderboardMaxMatches = 5000` const has comment "to avoid unbounded memory growth" but no rationale on why 5000 specifically. Phase spec mentioned the figure but reader of the code alone has no anchor.
**Fix:** Extend comment: `// 5000 matches/day × ~2 attempts/match × ~500 B/doc ≈ 5 MB peak. Above this, switch to aggregation pipeline (see review M2).`

### L-2 (Low) — leaderboard CountDocuments index coverage
**File:** `server/internal/store/leaderboards.go:62-73`
**Problem:** The CountDocuments filter is `{game_id, mode, state, completed_at}`. Existing index `matches_state_created_at` covers `(state, created_at)` — wrong sort key. `matches_state_expires_at` covers `(state, expires_at)`. There is no index on `(state, completed_at)`. CountDocuments will use the `state` prefix of one of the compounds and in-memory filter on the rest — fine at 5000 docs, but borderline at 50K.
**Impact:** Low at MVP. Not a blocker.
**Fix:** Note in `leaderboards.go` doc comment that an additional index `(state, completed_at)` may be needed when leaderboard date partitions grow. Or add it now (cheap; ~30 bytes/doc). Defer is acceptable.

### L-3 (Low) — `users.go:84-87` minor pre-existing nit (out of phase scope)
**File:** `server/internal/store/users.go:83-88`
**Problem:** `FindOneAndUpdate` with `SetUpsert(true)` — the `errors.Is(res.Err(), mongo.ErrNoDocuments)` guard is unreachable because upsert inserts a doc; either it returns the upserted doc or a real error. Not introduced by Phase 05.
**Fix:** Out of scope; leave for future cleanup.

## Strengths
- Comments are 1-line, on-point ("MUST: filter on source state to prevent double-resolve") — matches the spec's directive style.
- Index naming is descriptive and self-documenting (`attempts_match_player_unique`, `matches_share_token_unique`) — operators reading `db.collection.getIndexes()` will understand intent.
- Sentinel error pattern (`ErrLeaderboardTooLarge`) is idiomatic Go; scheduler's `errors.Is` discrimination preserves visibility while allowing graceful continue.
- Atlas tier doc subsection is structured: tier comparison + pool sizing + scale path. Better than the original "M0 with 100 max" oversimplification.
- `go test -race` clean across store + scheduler packages — no concurrency regression introduced by `CountDocuments` pre-flight.
- Threshold guard runs BEFORE the unbounded `cur.All` decode, so memory is genuinely bounded.
- Phase 01 invariants intact (verified): compound unique on attempts, `state:"pending"` filter on Complete, `ErrAttemptExists` flow unchanged.

## File-size check
- `matches.go` 234 LOC, `leaderboards.go` 278 LOC — both >200, both pre-existing. No regression from Phase 05. Out of scope to split.
- `indexes.go` 155, `mongo.go` 116, `users.go` 130, `models.go` 126, `tick.go` 91 — fine.

## Edge cases (scout-equivalent)
- **Create flow sets `state:"pending"`:** confirmed (`matches.go:42`). New JoinAsChallengee filter `state:"pending"` does not break happy path.
- **Existing prod data with non-"pending" state:** none — no prod deploy yet (per spec risk-assessment).
- **Concurrent join:** filter combines `share_token` (unique partial-index) + `state:"pending"` + `challengee_uid:$exists:false` — three-way guard. Strictly stronger than the previous 2-way guard, so race window narrows, never widens.
- **Tx retry idempotency:** Complete & CompleteSync filters on source state make retries no-ops (returns 0 ModifiedCount), preserving Phase 01 step 8 contract.
- **CountDocuments before Find:** small TOCTOU window between count and find, but threshold has 0 functional safety impact (just a soft cap), so race is benign.

## Open follow-ups
1. Decide daily_puzzles compound-key migration trigger (D-1) — comment now or refactor when 2nd game added.
2. Consider canonical `connstring.ParseAndValidate` if M-1 cosmetic concern matters; otherwise update comment.
3. Add `(state, completed_at)` index on `matches` before scaling past ~10K matches/day (L-2).
4. Document threshold magnitude rationale (L-3) — 5-min cleanup.
5. Phase 06 should add a concurrent-join test exercising the new `state:"pending"` filter.

**Status:** DONE_WITH_CONCERNS
**Summary:** Phase 05 ships the core hardening (state filters, index naming, threshold guard, Hints drop, Atlas docs). One spec deviation on daily_puzzles compound key and one cosmetic fail-fast claim warrant minor doc fixes; no blocking defects.
**Concerns/Blockers:** D-1 daily_puzzles compound key gap (spec deviation, MVP-acceptable); M-1 parseDBName fail-fast is mostly redundant (driver validates first).
