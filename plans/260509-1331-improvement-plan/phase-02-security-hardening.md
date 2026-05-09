# Phase 02 — Security & abuse hardening

## Context Links
- [Server review](reports/code-reviewer-server-260509-1331.md) — H10, M6, M7, M8, L7, L8
- [Web review](reports/code-reviewer-web-260509-1331.md) — H4, H5, M5
- [Architecture review](reports/code-reviewer-architecture-260509-1331.md) — H5

## Overview
- **Priority:** P1
- **Status:** completed
- **Description:** Stop logging credentials/UIDs at INFO; tighten WS origin matching; add per-UID rate limit; bound `AttemptSubmit.guesses`; force-refresh Firebase tokens on auth fail; surface auth errors instead of silently connecting with empty token.

## Key Insights
- `server/internal/ws/match_handler.go:115` logs the **share token** (bearer credential for challenge link) → log-replay attack vector (arch H5).
- `server/internal/ws/conn.go:129` and ~6 other `match_handler.go`/`match_room.go` lines log raw Firebase UIDs at INFO every connection (server H10).
- `server/internal/ws/conn.go:81` `OriginPatterns` accepts wildcards; doc claims strict equality (server M6).
- `server/internal/ws/rate_limiter.go` is per-conn only — 100 conns × bucket = 1000 msg/s aggregate (server M7).
- `web/src/routes/play/+page.svelte:216-221` (and leaderboard, m/[token]) silently `connect('')` if `idToken()` throws — server rejects, user sees red badge with no reason (web H4).
- `web/src/lib/auth-store.ts:26` calls `getIdToken()` without `forceRefresh` — no path to recover from 401 (web M5).
- `server/internal/ws/sync_match_handler.go:247-255` `displayName` falls back to raw `userID` → leaks UID into broadcast (`OpponentDisplayName`) (server L8).

## Requirements
- No bearer credentials, share tokens, or raw Firebase UIDs in logs at INFO/above.
- WS origin check rejects unintended wildcards in production.
- Per-UID rate limiting layer above per-conn (defence in depth).
- `AttemptSubmit.guesses` bounded at server (≤6 entries).
- Web surfaces auth failures rather than degrading to anonymous silently.
- Token-refresh path can force-refresh on demand (e.g., 401 handler hook).

## Related Code Files
**Modify**
- `server/internal/ws/match_handler.go` (truncate share token in log; bound guesses)
- `server/internal/ws/conn.go` (helper for hashed UID logging)
- `server/internal/ws/match_room.go`, `auth_refresh.go`, etc. (replace direct UID-in-log)
- `server/internal/ws/rate_limiter.go` (add per-UID bucket map)
- `server/internal/config/config.go` (document `OriginPatterns` glob behaviour)
- `server/internal/ws/sync_match_handler.go` (displayName fallback → "Player ${last4}")
- `web/src/lib/auth-store.ts` (`idToken(force = false)`)
- `web/src/routes/+layout.svelte` (catch idToken throw; surface toast; do NOT connect with empty)
- `web/src/lib/ws.ts` (on server-rejected upgrade or close-after-open, expose error to UI)

**Create**
- `server/internal/ws/log_redact.go` — `func redactUID(uid string) string` (HMAC-SHA256 with per-process random salt; 8-hex output).
- `web/src/lib/components/auth-error-toast.svelte` — minimal alert affordance.

## Implementation Steps

### Log redaction
1. Create `server/internal/ws/log_redact.go` with package-level `var logSalt [16]byte` initialised from `crypto/rand` at init. Export `redactUID(uid string) string` returning `fmt.Sprintf("u_%x", hmac.Sum256(salt, uid))[:10]`.
2. `server/internal/ws/match_handler.go:115` — change `token=%q` → `token=%q…` truncated to first 8 chars: `truncateToken(token)`. Add helper.
3. Replace direct UID log emits with `redactUID(c.UserID())`:
   - `server/internal/ws/conn.go:129`
   - `server/internal/ws/match_handler.go:99,115,238,245`
   - `server/internal/ws/match_room.go:217,224`
   - any others surfaced by grep `log.*c\.userID|log.*UID`.
4. `server/internal/ws/sync_match_handler.go:247-255` — `displayName` fallback: `"Player " + uid[len(uid)-4:]` (or `"Anonymous"` if anon). Drop raw-UID broadcast.

### Origin + rate limit + payload bounds
5. `server/internal/config/config.go:90` — extend doc comment for `AllowedOrigins`: "Glob patterns matching coder/websocket `OriginPatterns`. Use exact `host:port` strings unless wildcard intended; production should not contain `*`."
6. Add a boot-time check: `if cfg.IsProduction() && containsWildcard(cfg.AllowedOrigins) { log.Println("WARN: production origin contains wildcard") }`.
7. `server/internal/ws/rate_limiter.go` — add `type UIDLimiter struct { mu sync.Mutex; buckets map[string]*tokenBucket }` with `Allow(uid string)` using same bucket params. Wire into `dispatch` after auth: `if !hub.UIDLimiter.Allow(c.UserID()) { return errorEnvelope(429), nil }`.
8. `server/internal/ws/match_handler.go` (AttemptSubmit handler) — before persist, validate `len(msg.GetGuesses()) <= 6`; reject with 422.

### Web auth UX
9. `web/src/lib/auth-store.ts:23-26` — change to `export async function idToken(force = false): Promise<string> { const u = get(authUser); if (!u) throw ...; return u.getIdToken(force); }`.
10. `web/src/lib/ws.ts` — on `onclose` with code 1008/4401 or similar auth-reject (verify server close codes), trigger a one-shot `idToken(true)` then reconnect; if still fails, dispatch an `auth-error` event/store.
11. `web/src/routes/+layout.svelte` (hoisted from Phase 01) — on `idToken()` throw, do NOT call `connect('')`. Instead set `authError` store; render the new toast component.
12. Create `web/src/lib/components/auth-error-toast.svelte` — fixed-position alert with "Sign in to continue" + retry button; mounted in layout.

## Todo List
- [x] UID redaction helper + replace log emits (steps 1, 3)
- [x] Truncate share token in JoinAsChallengee log (step 2)
- [x] displayName fallback strips UID (step 4)
- [x] OriginPatterns doc + production wildcard warn (steps 5-6)
- [x] Per-UID rate limiter (step 7)
- [x] Bound AttemptSubmit.guesses (step 8)
- [x] `idToken(force)` overload (step 9)
- [x] WS auth-error path → force-refresh + surface (steps 10-12)

## Success Criteria
- `grep -nE 'log\..*\b(c\.userID|token)\b' server/internal/ws/*.go` returns 0 unredacted hits.
- Stress with `wrk` style: 200 conns from same UID → per-UID limiter caps message rate.
- AttemptSubmit with 100-element guesses array → 422, not stored.
- Web: sign-out then visit /play → toast appears, no silent empty-token connect.
- `cfg.AllowedOrigins=["*"]` in production env → boot logs WARN.

## Risk Assessment
- **Per-UID limiter map unbounded:** anonymous churn could grow `buckets`. Mitigation: TTL-evict idle entries (e.g., last-seen > 1h) in a goroutine, or LRU cap.
- **Force-refresh storm:** if server flaps 401, client could loop `idToken(true)` → Firebase rate limit. Mitigation: cap to 1 force-refresh per minute per session.
- **Origin warn-only is weak:** consider hard-fail on wildcard in prod; deferred to operator decision.

## Security Considerations
- Phase 01 already covers auth-field race; this phase reduces what leaks if logs are captured.
- Hashing salt is per-process — rotation on restart is fine (analytics on hashed UIDs across restarts is not a goal).
- Bearer token truncation must be irreversible (never log full token, including in error envelope payloads).

## Next Steps
- Phase 03 wires the auth-error toast into the broader UX affordance work.
- Phase 06 adds tests for rate-limit per-UID and bounded guesses.

## Completion Notes

**Completed:** 2026-05-09

**Summary of Changes:**
- `log_redact.go` created: UID redaction via HMAC-SHA256 with per-process salt.
- `match_handler.go:115` share token truncation → `truncateToken()` helper.
- `match_room.go:78` raw UID in log redacted via `RedactUID()`.
- `sync_match_handler.go:247-255` displayName fallback → `"Player ${last4}"` strips UID.
- `rate_limiter.go` added per-UID `UIDLimiter` with TTL eviction; wired into dispatch.
- `match_handler.go` AttemptSubmit handler bounds guesses to ≤6; rejects with 422.
- `auth-store.ts:idToken(force=false)` parameter added.
- `ws.ts` WS auth-error path forces token refresh (1/min cap) + signals auth error store.
- `AuthErrorToast` component created; mounted in `+layout.svelte` on `idToken()` throw.
- Review fix-ups: `match_room.go:78` raw UID redacted; `maxGuesses` → `wordle.MaxAttempts`; `auth-error-toast.svelte` `aria-live` removed (consistency with `role="alert"`).

**Test Results:**
- Server: `go test -race` 10/10 packages green (97 test runs).
- Web: `svelte-check` 0 errors.

**Reports:**
- [Code review](reports/code-reviewer-phase-02-diff-260509-1502.md) (APPROVE_WITH_FIXES — fixes applied)
- [Tester](reports/tester-phase-02-260509-1502.md) (green)
