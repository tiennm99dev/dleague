# Phase 02 Diff Review — Security & Abuse Hardening

**Date:** 2026-05-09 15:02 UTC
**Scope:** Post-Phase-02 diff (server+web). Verified: build/vet clean, `go test -race ./...` 9/9 pkgs pass, `svelte-check` 0 errors / 0 warnings.

## Verdict

**APPROVE_WITH_FIXES** — Phase intent met (logs sanitized, per-UID limiter, guesses bound, web auth-error path, force-refresh). One genuine PII leak remains (match_room.go:78 raw UID via wrapped error), and a few minor concerns. None are blockers in current shape but the leak undermines step 3 of the plan.

## Spec compliance

| # | Step | Status | Notes |
|---|------|--------|-------|
| 1 | log_redact.go w/ HMAC-SHA256 + 8-hex output | Done | `log_redact.go:23-31`. Per-process salt, fallback on rand failure. |
| 2 | TruncateToken share-token in JoinAsChallengee log | Done | `match_handler.go:117` `TruncateToken(token)`. |
| 3 | Replace direct UID logs w/ RedactUID | **Partial** | All `log.Printf` direct UIDs redacted, BUT `match_room.go:78` returns `fmt.Errorf("...conn %q...", c.UserID(), ...)` whose `%v` later prints raw UID via `sync_match_handler.go:99`. |
| 4 | displayName fallback strips UID | Done | `sync_match_handler.go:255-267`: anon→Anonymous; auth→"Player <last4>"; len<4 fallback "Player"; uid==""→Anonymous. |
| 5 | OriginPatterns doc | Done | `config.go:24-28` clarifies glob semantics + prod-warn. |
| 6 | Boot-time wildcard warn in prod | Done | `cmd/api/main.go:136-142`. Fires before HTTP listen (`:226`). |
| 7 | Per-UID rate limiter | Done | `rate_limiter.go:79-140` UIDLimiter w/ TTL eviction; wired in `conn.go:298-303` and `cmd/api/main.go:150-189`. Nil-safe. |
| 8 | Bound AttemptSubmit.guesses ≤6 | Done | `match_handler.go:151-154` returns 422 before any DB write. |
| 9 | idToken(force) overload | Done | `auth-store.ts:23`. |
| 10 | WS auth-reject force-refresh + 1/min cap | Done w/ adaptation | `ws.ts:130-142`; uses 1006 (correct — server returns HTTP 401 pre-upgrade); kept 1008/4401 for forward-compat. Reasonable adaptation. |
| 11 | Layout: don't `connect('')`, surface authError | Done | `+layout.svelte:30-37`. Sets `authError` on idToken throw. |
| 12 | auth-error-toast component | Done | `auth-error-toast.svelte` (62 lines), role=alert + retry calling `idToken(true)`+connect. |

## Issues

### Major

**[MAJOR] Raw UID still leaked via wrapped error in match_room.HandleMove**
`server/internal/ws/match_room.go:78`
```go
return fmt.Errorf("match_room: conn %q not a player in match %q", c.UserID(), r.MatchID)
```
This error propagates to `sync_match_handler.go:99`:
```go
log.Printf("ws match_move: HandleMove matchID=%q uid=%s: %v", msg.GetMatchId(), RedactUID(c.UserID()), err)
```
The `%v` formats the wrapped error which contains raw UID — the prefix `RedactUID(c.UserID())` is the conn's own UID (redacted), but `err` carries the *room's* matched UID via fmt.Errorf in raw form. Defeats step 3.
**Fix:** use `RedactUID(c.UserID())` inside the fmt.Errorf, or change to a sentinel error and have the caller log the redacted UID via `c.UserID()`.

### Minor

**[MINOR] `maxGuesses = 6` is a magic number; `wordle.MaxAttempts` already exists**
`server/internal/ws/match_handler.go:151`
```go
const maxGuesses = 6
```
`server/internal/game/wordle/wordle.go:13` exports `MaxAttempts = 6`. Drift risk if Wordle ever changes. Use `wordle.MaxAttempts`.

**[MINOR] No tests for UIDLimiter**
`server/internal/ws/rate_limiter_test.go` has 4 tests for `RateLimiter`, none for `UIDLimiter`. Race-clean from existing tests is incidental — eviction loop / concurrent Allow not exercised. Plan defers to Phase 06; flag here for visibility.

**[MINOR] TruncateToken returns "<short>" sentinel for tokens ≤8 chars**
`server/internal/ws/log_redact.go:34-39`. Acceptable but Firebase share tokens are >8 chars by design (UUID-derived in `store/matches.go`); branch is unreachable in practice. The 8-char prefix has 16^8 ≈ 4B combinations — fine for distinguishing in logs at this volume.

**[MINOR] Ellipsis is U+2026 (`…`)**
`log_redact.go:38` uses `"…"` not `"..."`. Most modern log parsers handle UTF-8 fine; some legacy line-based regex tools may misread byte length. Switch to `"..."` if downstream tooling is uncertain. Low priority.

**[MINOR] auth-error-toast a11y: role=alert + aria-live=polite is inconsistent**
`web/src/lib/components/auth-error-toast.svelte:20`
WAI-ARIA spec: `role="alert"` implies `aria-live="assertive"`. Setting `aria-live="polite"` here downgrades urgency. For an auth failure (user must act), assertive is appropriate. Either:
- Drop `aria-live` (let role=alert use its implicit assertive), OR
- If polite is intentional (don't interrupt screen reader), use `role="status"` with `aria-live="polite"` instead.

**[MINOR] Toast does not move focus to Retry button**
Same file. Screen-reader users will hear the alert but Tab order is unchanged. For a blocking auth failure, focusing the Retry button on mount would aid keyboard users. Acceptable as-is for a non-modal alert; flag as polish.

**[MINOR] `pendingForceRefresh` is module-scoped state, not promise-based**
`web/src/lib/ws.ts:70`. Works because module = per-tab/per-session in browser. Slight smell: a flag flipped in `onclose` and read in `scheduleReconnect`'s deferred `setTimeout`. Promise-based would be cleaner but YAGNI applies — current shape correct.

**[MINOR] `RedactUID("")` returns "u_anon"**
`log_redact.go:24-26`. No callers depend on a different return; verified by grep. But the literal "u_anon" could collide with a hashed UID prefix `u_<hex>` — extremely unlikely (`anon` is not hex), but consider `u_(empty)` for unambiguous distinction. Optional polish.

**[MINOR] `pendingForceRefresh` flag flipped only when 1/min cap NOT hit; if hit, code returns early without scheduling reconnect**
`ws.ts:131-142`. Reading carefully: when `now - lastForceRefreshAt > FORCE_REFRESH_COOLDOWN_MS` is false (within cooldown), branch sets `authError` then `return` — bails entirely, NO reconnect attempted. Correct (don't loop force-refresh). When cap NOT hit, sets `pendingForceRefresh=true` and falls through to `scheduleReconnect`. Verified the early-return guards against force-refresh storm. Good.

### Informational / verified-clean

- `cfg.IsProduction()` check fires at boot before HTTP listen (`main.go:136` precedes `srv.ListenAndServe` at `:228`). Spec compliance confirmed.
- 1006-only on first attempt guard (`reconnectAttempt === 0`) correctly distinguishes server reject from mid-session network drops. Lid-close-then-reopen scenario: first reconnect at attempt count >0 (because `closed=false` and reconnectAttempt was incremented during prior attempts since the initial connect), so the force-refresh path is skipped. Correct.
- All `idToken(` callsites reviewed: layout & ws.ts:359 (refresh timer) keep `force=false` (right — that's a periodic refresh, not auth-fail recovery); ws.ts:167 uses `pendingForceRefresh` flag; toast retry uses `force=true`. Coverage correct.
- UIDLimiter: `Allow` updates `lastSeen` (`rate_limiter.go:111`); `EvictIdle` snapshot under lock (`:117-126`); `RunEvictor` exits on ctx done (`:132-139`). Race-clean — `go test -race` passes.
- 20 msg/sec / 40 burst for sync match: a normal player submits ~5-10 ATTEMPT_SUBMITs per match plus PING/PONG; far below cap. Anti-abuse threshold is reasonable. No handler emits multiple downstream WS calls per inbound (verified via grep — each handler returns one envelope).
- Phase 01 invariants intact: lifecycle hoist still in `+layout.svelte:23`; `rejectAllPending` in ws.ts:125; quick-match gate confirmed via prior tester report.
- AttemptSubmit guesses-bound rejection path: returns 422 before idempotency check, before `Insert`, before `WithTransaction`. No DB write on reject. Verified.
- `displayName`: anon→"Anonymous", uid==""→"Anonymous", uid len<4→"Player", uid len≥4→"Player <last4>". All branches present (`sync_match_handler.go:255-267`).

## Strengths

- Clean separation: redaction helpers in dedicated file (`log_redact.go`), wired surgically — no scattered string concat.
- UIDLimiter design: TTL eviction prevents unbounded growth; nil-safe wiring for tests.
- Adaptation comment on close-code 1006 with `reconnectAttempt === 0` guard is thoughtful — implementer caught a spec drift (server uses 401 pre-upgrade, not 1008/4401) and chose forward-compat without breaking present behavior.
- 1/min force-refresh cap prevents the documented Firebase rate-limit storm.
- Tests pass under `-race` (10/10 runs clean per implementer; verified locally 1× clean).

## Open follow-ups

1. Fix match_room.go:78 raw UID in error wrap (block step 3 truly complete).
2. Replace `const maxGuesses = 6` with `wordle.MaxAttempts`.
3. Add UIDLimiter unit tests (deferred to Phase 06 per plan).
4. Decide on a11y: `role=alert` + assertive vs `role=status` + polite for auth-error toast.
5. Optional: focus Retry button on mount for keyboard a11y.

## Unresolved questions

- Is `match_room.go:78` raw-UID leak considered in-scope for Phase 02 hot-fix or deferred to Phase 03 polish? The plan explicitly says "0 unredacted hits" for grep success-criterion — it slips the literal grep (which scans `log.*`) but violates the spirit (PII in logs).
- Should `RedactUID` collision-resistance be ratcheted up (e.g., 12 hex chars)? At 8 hex chars and per-process salt rotation, intra-restart collision risk is ~birthday √(2^32) ≈ 65k UIDs before 50% collision. For an MVP serving a few thousand DAU acceptable; flag for product-scale review.

```
**Status:** DONE_WITH_CONCERNS
**Summary:** APPROVE_WITH_FIXES — Phase 02 spec largely met; one genuine PII leak in match_room.go:78 (raw UID embedded in fmt.Errorf, surfaces via %v in caller's log) should be addressed before close. Other items are minor polish.
```
