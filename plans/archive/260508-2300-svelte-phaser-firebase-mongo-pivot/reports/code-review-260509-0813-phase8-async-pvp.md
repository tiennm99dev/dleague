# Code Review — Phase 08 Async PvP

Date: 260509-0813 (UTC)
Scope: vs `a4b3762` (Phase 07). 16 modified + ~13 new files.
Verdict: **DONE_WITH_CONCERNS** — 2 critical (anti-correctness vs spec), several med/low.

## Build / Test
- `go build ./...` clean
- `go vet ./...` clean
- `go test ./...` 7/7 packages pass (Mongo-gated tests skip cleanly without `MONGO_TEST_URI`)
- `npx svelte-check` 0 errors / 0 warnings (390 files)

---

## Critical

### C1. Anonymous filter is non-functional — `is_anonymous` never persisted on User docs
- `server/internal/ws/conn.go:121` sets `conn.isAnonymous` from the Firebase token, but
- `server/internal/ws/conn.go:99-104` calls `hub.userRepo.UpsertByUID(ctx, uid, profile)` where `profile := tokenToProfile(...)` (line 151-161); `UserProfile` has no `IsAnonymous` field and `users.go:39-72` never writes `is_anonymous` on insert/update.
- `server/internal/store/leaderboards.go:144` then filters `if !ok || u.IsAnonymous { continue }` — but `u.IsAnonymous` is always the Go zero-value `false` because the field doesn't exist in any user doc. **Anonymous users WILL appear on the leaderboard.**
- Same defect in `users.go:86`: `IncrementStats` filter `{_id: uid, is_anonymous: {$ne: true}}` — `$ne: true` matches docs where the field is missing, so anonymous users' stats ARE incremented.
- Spec line 27, 142, 169, 184: anonymous filter is a hard requirement.
- **Fix sketch**: extend `UserProfile` with `IsAnonymous bool`; pass `connAnonymous` from `conn.go` into `tokenToProfile`; have `UpsertByUID` write `is_anonymous` on both `$set` (and `$setOnInsert` for safety).
- Severity: **Critical** (spec requirement violated; security/UX intent broken).

### C2. WithTransaction retry can return wrong "pending" status to client
- `server/internal/ws/match_handler.go:194-232`. mongo-driver v2 `WithTransaction` retries the callback on `TransientTransactionError` / `UnknownTransactionCommitResult`.
- Race: A and B submit concurrently. Both transactions Insert their attempt. Both reach `Complete`. One commits; the other gets a write conflict and the callback re-runs. On retry, `AttemptRepo.Insert` (line 56) sees existing attempt → returns `ErrAttemptExists`. Callback line 196-198 returns `(nil, nil)` without setting `winnerUID`/`completed`.
- Closure variables `winnerUID, completed` (lines 185-186) keep zero values — outer code at lines 239-245 marshals `{winner_uid:"", status:"pending"}`. But the DB state is actually `complete` (set by the OTHER transaction).
- Client receives a misleading "pending" ack on a fully-completed match. They will not know the result without a second poll/refresh.
- **Fix sketch**: when handling `ErrAttemptExists` inside the tx, re-fetch match via `MatchRepo.GetByID(sc, matchID)`; if `state=="complete"`, copy `match.WinnerUID`/`completed=true` into closure vars before returning nil.
- Severity: **Critical** (correctness under contention — likely to bite within first weeks of real PvP usage).

---

## High

### H1. `share_token` unique+sparse index will block Phase 09 sync matches
- `server/internal/store/indexes.go:53-55`: `share_token` has `SetUnique(true).SetSparse(true)`.
- `models.go:64`: `ShareToken string` is plain (no `omitempty`) → always serialized as `""` for any future sync-mode match.
- Sparse only skips docs where the field is **missing**, not empty. Two sync-mode matches (no share_token assigned) will both write `share_token: ""` → unique violation on the second.
- Async PvP today is unaffected (every async match has a UUID), so this is a forward-compat landmine for Phase 09.
- **Fix**: add `,omitempty` to `ShareToken` and only set it for async matches; or use `SetPartialFilterExpression({mode: "async"})` instead of `SetSparse`.
- Severity: **High** (guaranteed regression when sync mode lands).

### H2. Stale leaderboard index on non-existent `period_end` field
- `server/internal/store/indexes.go:97`: index `{game_id: 1, period_end: -1}`.
- `models.go:117-125`: `Leaderboard` has no `period_end` field; the document key for queries is `_id` (e.g. `"wordle_daily_2026-05-09"`), `game_id`, `period`, `date`.
- The index covers nothing real. `LeaderboardRepo.Get` (`leaderboards.go:218`) queries by `_id`, which uses the implicit `_id` index — fine. But the misnamed index wastes RAM and is misleading.
- **Fix**: drop the index, or replace with `{game_id: 1, date: -1}` if "find latest leaderboard for game" is a future query.
- Severity: **High** (factually wrong — index will never accelerate any code path; comment on indexes.go:13 still says "8 indexes across 5 collections" but EnsureIndexes now creates 10).

### H3. No WS-handler tests; spec §14 E2E not implemented
- Spec §14: `End-to-end: two test users, A creates challenge, B joins, both submit, server returns correct winner.` — not present.
- `server/internal/ws/` only has `conn_test.go` and `hub_test.go` with no Phase-08 cases (grep confirms zero `ChallengeCreate|ChallengeJoin|AttemptSubmit|LeaderboardQuery` in `_test.go`).
- The store-level race test (`matches_test.go:69`) covers the join-race invariant only.
- decideWinner (`match_handler.go:282-314`) is also untested.
- Severity: **High** (winning-decision logic is the most consequential code path in this phase).

---

## Medium

### M1. `match_handler.go` exceeds the spec-stated 200 LOC budget
- 332 LOC. Spec line 53: "Each handler file <200 LOC." `leaderboards.go` 259 LOC also crosses the project's general 200-line guideline.
- Ergonomic refactor: extract `decideWinner` + `buildAttemptAck` + `resolveDailySeed` into `match_logic.go`.
- Severity: Med (code-standards drift, not a bug).

### M2. Client `connect()` race in `/m/[token]/+page.svelte`
- `+page.svelte:73-87`: `onMount` calls `connect(fbToken)` then `await joinChallenge(token)`. Inside `joinChallenge` `waitForConnection` subscribes to `connectionState`. If WS opens BEFORE the subscriber attaches, the synchronous `subscribe` callback fires immediately with the current state — Svelte stores deliver current value on subscribe — so this is OK, but only if `connectionState` is a Svelte writable/readable. Worth confirming `$lib/ws.ts` exposes it as such (not verified in this review).
- Same pattern in `leaderboard/+page.svelte:63`.
- Severity: Med (works for Svelte stores; brittle if `connectionState` ever becomes a plain emitter).

### M3. `gameStartMs` reset misses challenge-mode start
- `web/src/routes/play/+page.svelte:214`: `gameStartMs = Date.now()` set at `onMount`, not at first guess.
- Challenge-mode flow: `/m/[token]` → goto `/play?match=...` → onMount runs → gameStartMs fixed BEFORE user makes any move. The `time_ms` therefore includes "time spent reading instructions / waiting" — typically 1-3 seconds extra, but can be unbounded if the user pauses.
- Anti-cheat lower bound (>500ms) still satisfied; leaderboard fairness skewed though.
- **Fix**: reset `gameStartMs` on first non-empty guess.
- Severity: Med (UX/fairness — affects time-tiebreak ordering).

### M4. `submitAttempt` swallows server error silently
- `play/+page.svelte:106-126`: on `sendRequest` failure (timeout, 422, 500), the promise rejects → `console.error` only. UI shows "Submitting your result…" spinner forever (line 56 in `results-screen.svelte`); state never transitions because `attemptSubmitting` is set false in `finally`, but `winnerUid`/`matchStatus` remain empty so `isPending` (line 39) stays true.
- Spec line 145 returns 422 for anti-cheat — client shows no message.
- **Fix**: surface error in `errorMsg` toast; offer Retry button.
- Severity: Med (poor UX on the very flow Phase 08 ships).

### M5. Comment drift: `EnsureIndexes` doc says 8 indexes
- `indexes.go:13`: "creates the 8 application-level indexes" — actually creates 10.
- Severity: Med (documentation correctness).

---

## Low / Nit

### L1. Solo daily wins do not appear on the "Daily Leaderboard"
- `leaderboards.go:54-62` filters `mode: "async"` only. Solo daily play (`game_handler.go`) records no Match/Attempt at all — only the in-memory `wordleSession`.
- Per spec line 80-87 this is by-design (leaderboard pre-computes from completed async matches), but the route is named `/leaderboard` and labelled "Daily Leaderboard". Users who only play solo will never see themselves.
- Either rename to "Daily Challenges Leaderboard" or persist solo attempts (out-of-scope here).
- Severity: Low (UX/copy).

### L2. Carryover: `sessions sync.Map` never evicted
- `game_handler.go:50, 113-115`. Solo Wordle session map grows unboundedly per UID. Acknowledged as deferred from Phase 07 M3; async PvP does NOT use this map (handlers operate via repos), so blast radius unchanged.
- Severity: Low (acknowledged).

### L3. `resolveDailySeed` dead-code branch
- `match_handler.go:266-272`: the `dailyGetter` type assertion is always true for the production `*store.DailyPuzzleRepo` (`DailyPuzzleStore` interface already includes `GetByDate`). The `seed 0` fallback for failed-assertion is unreachable. Harmless but misleading.
- Severity: Nit.

### L4. `IncrementStats` errors are logged but not surfaced
- `match_handler.go:227-229`: stats failures `log.Printf` and continue. Acceptable per "best-effort" comment but means a partial-update can leave winner with no `wins++` if Mongo flickers. No reconciliation path.
- Severity: Low (MVP-acceptable tradeoff).

### L5. `LeaderboardQuery` returns empty rankings silently when no snapshot
- `leaderboard_handler.go:44-46`: if `lb == nil`, returns empty `LeaderboardSnapshot{}` and `log.Printf` warns. Client cannot distinguish "no snapshot yet" from "no players today". Low priority — `results-screen.svelte` shows "No entries yet for today".
- Severity: Nit.

### L6. `JoinAsChallengee` filter relies on `omitempty` BSON encoding
- `matches.go:99` `challengee_uid: {$exists: false}`. Works because `Match.ChallengeeUID *string` + `bson:"challengee_uid,omitempty"` (`models.go:63`) means nil is omitted on Insert. Verified, but a future contributor changing the type to plain string would silently break the join-race semantics.
- Defensive alternative: `{$or: [{challengee_uid: {$exists: false}}, {challengee_uid: nil}]}`.
- Severity: Nit.

### L7. `Players []string` field still in Match for "Phase 09 sync mode reuse"
- `models.go:55-57`. YAGNI — adds noise. Drop until needed.
- Severity: Nit.

---

## Positive Observations
- `JoinAsChallengee` correctly atomic via single `FindOneAndUpdate` with sentinel `ErrAlreadyJoined` — clean, race-safe; `matches_test.go:69` proves it.
- Anti-cheat bounds (`match_handler.go:148-151`) properly return 422 ERROR envelope.
- Self-join blocked at `match_handler.go:91-93` BEFORE the transaction starts (good — no wasted session).
- Idempotency: pre-tx `GetByMatchAndPlayer` (line 159-169) cleanly returns existing ack.
- Scheduler lifecycle (`scheduler/tick.go`) tied to signal context; tests cover both fire and cancel paths.
- Auth gate allowlist (`auth_gate.go:13-18`) correctly excludes only protocol/auth messages — all 4 new types require auth via the default-true branch.
- `matchOID` parse + `bson.ObjectIDFromHex` validation guards against malformed match_id (line 171-174).
- Svelte route `/m/[token]` SSR off (`+page.ts`) is correct for client-only WS flow.
- TypeScript strict mode passes with no `any` leaks across new files.

---

## Block-the-commit Decision
- **C1 (anonymous-filter dead code)** is a spec-requirement violation. The phase status should NOT be marked `completed` until either:
  (a) `is_anonymous` is persisted on User and the filter actually fires, or
  (b) the phase plan is amended to descope anonymous-exclusion to Phase 09.
- **C2 (status-pending bug)** is a real correctness defect under realistic concurrency. Tolerable to land if the spec is updated to acknowledge it; better to fix in this commit.
- H1 and H2 are high-priority but not blocking — they affect future phases / are wasted-work indexes.
- H3 missing E2E test means C2 was never going to be caught by tests; recommend at least adding a unit test for `decideWinner`.

If the team accepts shipping with C1 and C2 documented as known gaps, the rest can land. Otherwise: address C1 (small change) and C2 (re-fetch on `ErrAttemptExists` inside tx) before merge.

---

## Unresolved Questions
1. Is anonymous-leaderboard exclusion mandatory for Phase 08, or can it slip to Phase 09 alongside rate-limiting? Spec says mandatory; landed code does not deliver it.
2. Should solo daily wins persist as Attempts (with a synthetic Match or `mode: "solo"`) so the "Daily Leaderboard" reflects all daily play, not just challenges? Current behaviour is challenge-only — confirm UX intent.
3. `share_token` index sparseness vs partialFilterExpression — please confirm Phase 09 will revisit before any sync match writes occur.
4. Is `time_ms` measured from `/play` mount or first-guess? Spec is silent; current impl uses mount which mildly favours fast-typers and penalises challengees who arrive via redirect (extra ~500-1500ms).

**Status:** DONE_WITH_CONCERNS
**Summary:** Phase 08 builds and tests pass; race-safe join + anti-cheat + scheduler are correctly implemented. Two critical defects: (C1) anonymous filter is dead code because `is_anonymous` is never persisted on User docs, and (C2) `WithTransaction` retry path can return a misleading `pending` ack on a completed match. Several Med/Low items around stale index, missing E2E tests, and 332-LOC handler.
**Concerns/Blockers:** C1 violates spec requirement (anonymous attempts must be excluded from leaderboard); recommend not marking phase complete until fixed or descoped. C2 is fixable with a re-fetch inside the `ErrAttemptExists` branch.
