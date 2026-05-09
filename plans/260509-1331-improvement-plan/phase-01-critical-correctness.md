# Phase 01 — Critical correctness bugs

## Context Links
- [Server review](reports/code-reviewer-server-260509-1331.md) — H1, H2, H5, H8, H9
- [Web review](reports/code-reviewer-web-260509-1331.md) — H1, H3, H5, H8, H9, M19

## Overview
- **Priority:** P1
- **Status:** completed
- **Description:** Must-fix correctness issues. Cover races (queue stale conn, auth-field reads, GAME_STATE double-dispatch), DoS amplifier (`os.Exit`), duplicate writes (attempts, stats), and silent-failure UX paths (no-connect on routes, abandoned promises, broken sync Enter). All blockers for trust in the system.

## Key Insights
- `server/internal/ws/conn.go:161-172` disconnect defer never removes conn from queue → stale `*Conn` → ghost match (review server H1).
- `server/internal/ws/auth_refresh.go:48-51` writes `userID/isAnonymous/isAdmin/tokenExpiresAt` outside `c.mu`; cross-conn readers via `HandleMove`/grace timer race-read torn strings (server H2).
- `server/internal/ws/sync_match_handler.go:267-279` `cryptoSeed` calls `os.Exit(1)` on `crypto/rand` failure — kernel hiccup nukes whole server (server H5).
- `server/internal/store/indexes.go:78` compound `(match_id, player_uid)` index is **not unique** → concurrent retries dup-insert attempts (server H8).
- `web/src/routes/quick-match/+page.svelte:11-12` and `web/src/routes/sync/+page.svelte` mount but never call `connect()` — page hangs (web H1).
- `web/src/lib/ws.ts:117-125` `onclose` doesn't reject pending promises until max-attempts; `submitGuess`/`submitAttempt` silently drop on disconnect (web H3).
- `web/src/lib/components/sync-game-scene.svelte:122-123` checks `'ENTER'/'BACKSPACE'` but `keyboard.svelte:17` emits `'Enter'/'Backspace'` → on-screen Enter broken (web M19, re-classified H).

## Requirements
**Functional**
- All routes that need WS get a connected socket before sending frames.
- No stale `*Conn` in queue after tab-close.
- Cross-goroutine reads of conn auth fields are safe under `-race`.
- Server fails one request rather than crashing on `crypto/rand` error.
- No duplicate `attempt` doc; no double-incremented user stats on tx retry.
- Pending WS requests rejected promptly when socket closes.
- Sync mode on-screen keyboard Enter submits a guess.
- Rejoin payload (`MATCH_REJOIN_ACK`) rehydrates board + opponent rows.

**Non-functional**
- `go test -race` clean.
- No regression in async/sync match happy path.
- No new lint warnings.

## Related Code Files
**Modify**
- `server/internal/ws/conn.go` (disconnect defer, mu scope)
- `server/internal/ws/auth_refresh.go` (lock writes)
- `server/internal/ws/hub.go` (read accessors for userID)
- `server/internal/ws/match_room.go` (use accessor)
- `server/internal/ws/sync_match_handler.go` (cryptoSeed return error; displayName)
- `server/internal/ws/queue.go` (no-op; called from defer)
- `server/internal/ws/match_handler.go` (state filter; idempotent stats)
- `server/internal/store/matches.go` (`Complete` filter `state:"pending"`)
- `server/internal/store/attempts.go` (handle 11000 → ErrAttemptExists)
- `server/internal/store/indexes.go` (make compound unique)
- `web/src/routes/+layout.svelte` (hoist connect/disconnect)
- `web/src/routes/quick-match/+page.svelte` (drop manual connect; keep handler reg)
- `web/src/routes/sync/+page.svelte` (drop disconnect)
- `web/src/routes/play/+page.svelte` (drop connect; dedupe GAME_STATE)
- `web/src/routes/leaderboard/+page.svelte` (drop disconnect)
- `web/src/routes/m/[token]/+page.svelte` (drop disconnect)
- `web/src/lib/ws.ts` (rejectAllPending in onclose; fresh token in scheduleReconnect)
- `web/src/lib/components/sync-game-scene.svelte` (normalize Enter/Backspace; accept rejoin props)
- `web/src/lib/components/keyboard.svelte` (single casing convention)

**Create**
- `web/src/lib/match-rejoin-store.ts` — small writable for `ack.ownState`/`ack.opponentHints`.

**Delete** — none.

## Implementation Steps

### Server-side races + DoS
1. `server/internal/ws/conn.go:161-172` — in disconnect defer, before `deleteSession`, add nil-checked `if hub.GameDeps != nil && hub.GameDeps.Queue != nil { hub.GameDeps.Queue.Remove(conn) }`.
2. `server/internal/ws/conn.go:34-37` — extend `Conn.mu` doc to cover `userID/isAnonymous/isAdmin/tokenExpiresAt`. Add `func (c *Conn) UserID() string`, `IsAnonymous() bool`, `IsAdmin() bool` that take RLock.
3. `server/internal/ws/auth_refresh.go:48-51` — wrap the four-field write in `c.mu.Lock()/Unlock()`.
4. Replace cross-goroutine direct field reads with accessors: `hub.go:103`, `match_room.go:51,140,217,224`, `disconnect.go:56-57`, leaderboard handler if any.
5. `server/internal/ws/sync_match_handler.go:267-279` — change `func cryptoSeed() int64` to `(int64, error)`; on `rand.Read` err return `(0, err)`. Caller in `startSyncMatch` returns 500 envelope.

### Server-side dup writes
6. `server/internal/store/indexes.go:78` — change the `(match_id, player_uid)` compound index to `Unique: true`. Drop+recreate at boot is OK (dev empty).
7. `server/internal/store/attempts.go:53-74` — keep the in-tx Find fast-path; on Insert, type-switch `mongo.WriteException` and return `ErrAttemptExists` for code 11000.
8. `server/internal/store/matches.go:136` — `Complete` UpdateOne filter add `state: "pending"`. Mirror `CompleteSync`.
9. `server/internal/ws/match_handler.go:236-240` — gate `IncrementStats` on `result.ModifiedCount == 1`. If 0, treat as "another tx already completed" → no increment.

### Web lifecycle
10. `web/src/routes/+layout.svelte` — in `onMount`, `authUser.subscribe(async (u) => { if (u && !connected) { try { connect(await idToken()); } catch { /* show toast */ } } else if (!u && connected) disconnect(); })`. Track `connected` locally.
11. Remove `connect()`/`disconnect()` calls from per-route files: `play/+page.svelte:216-221`, `leaderboard:55-60`, `m/[token]:80-85`, plus `onDestroy` disconnects in same files.
12. Per-route `onMount`: register handlers BEFORE any `sendRequest` calls. `play/+page.svelte:211-228` reorder: handlers first, then `gameStartMs`, then no connect.
13. `web/src/lib/ws.ts:117-125` — in `onclose`, before reconnect schedule, call `rejectAllPending(new Error('connection lost'))` regardless of `closed` value (caller can retry).
14. `web/src/lib/ws.ts:128-137` — change `scheduleReconnect` to fetch fresh token before each `openSocket`: `setTimeout(async () => { try { openSocket(await idToken()); } catch { /* surface */ } }, delay)`. Drop the `token` param.

### Web rejoin + dispatch dedupe
15. Create `web/src/lib/match-rejoin-store.ts` exporting `writable<{ ownState?: WordleState; opponentHints?: WordleHint[] } | null>(null)`.
16. `+layout.svelte:34` — on `MATCH_REJOIN_ACK`, set the store before `goto('/sync?...')`.
17. `web/src/routes/sync/+page.svelte:20` — pass `initialState`/`initialOpponentHints` props from store; clear store after consumption.
18. `web/src/lib/components/sync-game-scene.svelte:25-28` — wire props into Phaser scene init.

### Web sync Enter + duplicate dispatch
19. `web/src/lib/components/keyboard.svelte:17` — emit canonical `'Enter'`/`'Backspace'` (already does); confirm.
20. `web/src/lib/components/sync-game-scene.svelte:74-87,122-123` — change comparisons to `'Enter'`/`'Backspace'` (drop uppercase).
21. `web/src/routes/play/+page.svelte:79-98,184,223-227` — pick ONE delivery for GAME_STATE. Remove the push handler at `:223-227`; keep only the request/response path. Add `lastAppliedSeq` if protocol grows seq.
22. Verify `attemptSubmitting` guard around `submitAttempt` — set synchronously before any await.

## Todo List

### Server
- [x] `conn.go` queue-remove on disconnect (step 1)
- [x] `Conn` accessors + `mu` extension; auth_refresh write under lock (steps 2-4)
- [x] `cryptoSeed` returns error; handler responds 500 (step 5)
- [x] Unique compound index + WriteException handling (steps 6-7)
- [x] `Complete` state filter + idempotent stats (steps 8-9)

### Web
- [x] Hoist WS lifecycle to layout; remove per-route connect/disconnect (steps 10-11)
- [x] Register handlers before send (step 12)
- [x] `rejectAllPending` on close + fresh token on reconnect (steps 13-14)
- [x] Rejoin store + sync-game props wiring (steps 15-18)
- [x] Normalize Enter/Backspace casing (steps 19-20)
- [x] Dedupe GAME_STATE delivery + tighten attemptSubmitting guard (steps 21-22)

## Success Criteria
- `go test ./... -race` passes.
- Manual: open `/quick-match` directly in fresh tab → matches and starts (no spinner-forever).
- Manual: kill server mid-`submitGuess` → client shows "connection lost" within 1s, not 5s.
- Manual: sync match on-screen keyboard Enter submits.
- Mongo: `db.attempts.getIndexes()` shows `unique: true` on `(match_id, player_uid)`.
- Manual: force tx retry via fault injection (or 2 concurrent submits) — only one attempt doc; user `wins` increments by 1, not 2.
- Manual: refresh page mid-sync match → board + opponent rows rehydrate from rejoin payload.

## Risk Assessment
- **Unique-index migration on existing data:** if any prod-shaped Mongo has dup attempt docs, index creation fails. Mitigation: add a one-shot dedup query in `EnsureIndexes` that warns + aborts if dups found; document operator action.
- **Layout-hoisted WS may break SSR:** `+layout.svelte` already runs client-only (`+layout.ts` ssr=false confirmed in web review strengths). Low risk.
- **`scheduleReconnect` losing token param:** if `idToken()` throws during reconnect, no fallback. Mitigation: surface a toast via connection-status component; user retries via affordance in Phase 03.
- **Removing GAME_STATE push path:** if server starts pushing without request_id, client misses updates. Mitigation: confirm with server side that GAME_STATE is request-bound (verified at server `match_handler.go` paths). Add log in dev-mode if push arrives.

## Security Considerations
- Auth-refresh under lock closes a torn-string read window. Net: tightens session integrity.
- Index uniqueness prevents double-counting that could be abused for stat inflation.

## Next Steps
- Phase 02 builds on the same conn-accessor work to redact UIDs in logs.
- Phase 06 writes regression tests over each fix.

## Completion notes

**Date:** 2026-05-09

**Status:** All 22 implementation steps landed + review fixes applied.

**Test results:**
- Server: `go build` ✅, `go vet` ✅, `go test -race` 83 tests pass ✅
- Web: `svelte-check` 0 errors / 0 warnings ✅

**Review findings:** Code review phase reported 2 regressions (M1: ws.ts retry cap, M2: quick-match connection gate) — both fixes applied in same session before test run.

**Reports:**
- [code-reviewer-phase-01-diff-260509-1502.md](reports/code-reviewer-phase-01-diff-260509-1502.md) — APPROVE_WITH_FIXES (fixes applied)
- [tester-phase-01-260509-1502.md](reports/tester-phase-01-260509-1502.md) — all green
