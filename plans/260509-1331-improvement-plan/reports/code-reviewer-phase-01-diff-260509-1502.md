# Phase 01 Diff Review — Critical Correctness

## Verdict
**APPROVE_WITH_FIXES** — All 22 spec steps landed; race-detector + svelte-check clean. One major regression (quick-match has no connection wait), one major bug (scheduleReconnect lacks retry cap on token-fetch failure), several minor cleanups.

## Spec compliance

| # | Step | Status | Notes |
|---|------|--------|-------|
| 1 | conn.go disconnect defer queue.Remove | Done | conn.go:189-194; nil-guarded; runs before grace+session+unregister |
| 2 | Conn.mu doc + UserID/IsAnonymous/IsAdmin accessors | Done | conn.go:43-48 doc, 66-88 accessors; all RLock |
| 3 | auth_refresh four-field write under Lock | Done+ | auth_refresh.go:53-58; also writes isAdmin (was missing pre-fix) — see Issues §3 |
| 4 | Replace cross-goroutine c.userID/p.userID reads w/ accessors | Done | hub.go:103, disconnect.go:37,56-57, match_room.go:50,135,141,187,190,196-200,224,264,267, leaderboard_handler.go:17, queue.go:52, game_handler.go:64,99, sync_match_handler.go:26,73,99,109,132,135,215,258. One direct read remains: conn.go:203 — safe (same-goroutine) but inconsistent |
| 5 | cryptoSeed (int64,error); caller 500 envelope | Done | sync_match_handler.go:275-285 + caller 181-187; `os` import gone |
| 6 | Unique compound (match_id,player_uid) | Done | indexes.go:82-90 SetUnique(true) |
| 7 | attempts.go: keep Find fast-path + WriteException 11000 | Done | attempts.go:53-65 (Find) + 73-81 (typed switch) |
| 8 | Complete filter `state:"pending"` + ModifiedCount | Done | matches.go:126-145; returns (int64,error). Sole caller match_handler.go:234 updated. CompleteSync left intact (already had `state:"active"` filter from M5) |
| 9 | IncrementStats gated on ModifiedCount==1 | Done | match_handler.go:241-249; debug log on skip |
| 10 | +layout.svelte hoist connect/disconnect | Done | +layout.svelte:21-39; tracks `connected` local; subscribe to authUser |
| 11 | Remove per-route connect/disconnect | Done | play, leaderboard, m/[token], sync clean. quick-match never had them (pre-existing) |
| 12 | Register handlers BEFORE sendRequest | Done | sync-game-scene onMount registers handlers first; play/+page no longer push-handles GAME_STATE |
| 13 | rejectAllPending in onclose unconditional | Done | ws.ts:121 — runs before reconnect schedule |
| 14 | scheduleReconnect fetches fresh token; drop param | Partial | ws.ts:128-144 fetches fresh token, but failure-recovery loop has no retry cap — see Issues §1 |
| 15 | match-rejoin-store.ts | Done | exports writable<{ownState?,opponentHints?}\|null>(null) |
| 16 | +layout.svelte set store before goto | Done | +layout.svelte:51-54 |
| 17 | sync/+page reads store, clears after consume | Done | sync/+page.svelte:19-26; passes initialState/initialOpponentHints props |
| 18 | sync-game-scene wires init props | Done | sync-game-scene.svelte:25-50; uses untrack() for $state initialisers |
| 19 | keyboard.svelte canonical 'Enter'/'Backspace' | Done | keyboard.svelte:17,45 — already correct, verified |
| 20 | sync-game-scene Enter/Backspace title-case | Done | sync-game-scene.svelte:77,82 |
| 21 | Dedupe GAME_STATE delivery | Done | play/+page.svelte no longer registers GAME_STATE handler; sync-game-scene registers in onMount |
| 22 | attemptSubmitting guard before await | Done | play/+page.svelte:167-168 — `if (submitting) return; submitting = true;` synchronous before line 173 await |

## Issues found

### Major (blocking)

**M1 — `scheduleReconnect` recursive loop has no retry cap on token failure**
File: `web/src/lib/ws.ts:128-144`
The MAX_RECONNECT_ATTEMPTS check lives only in `onclose` (line 122), not in `scheduleReconnect`. When `idToken()` rejects (e.g. user signed out, network failure, Firebase down), the catch on line 141-143 calls `scheduleReconnect(0)` recursively. `reconnectAttempt++` runs unconditionally each pass. Delay caps at MAX_RECONNECT_DELAY_MS (30s), but the timer is rescheduled forever.
Repro: user signs out mid-reconnect → infinite 30s-spaced retries until tab closes.
Fix: add `if (reconnectAttempt >= MAX_RECONNECT_ATTEMPTS) { connectionState.set('disconnected'); return; }` at the top of scheduleReconnect, OR check `closed` flag in the catch and bail.

**M2 — quick-match sends QUEUE_JOIN before WS may be connected**
File: `web/src/routes/quick-match/+page.svelte:11-12`
`onMount` calls `sendQueueJoin('wordle')` immediately. `sendQueueJoin` (ws.ts:231) silently no-ops if `socket.readyState !== OPEN`. After hoisting WS lifecycle to layout, fresh-tab nav to `/quick-match` races: layout's `connect()` is async; queue join fires synchronously and is dropped.
This is the exact failure phase H1/step 11 was meant to fix. The hoist closed it for routes that wait on `connectionState` (m/[token], leaderboard, sync) but NOT quick-match. Phase success criterion "open `/quick-match` directly in fresh tab → matches and starts (no spinner-forever)" is not satisfied.
Fix: in quick-match onMount, gate `sendQueueJoin` on `connectionState.subscribe(s => s === 'connected')`, mirroring leaderboard/+page.svelte:55-60 pattern.

### Minor

**N1 — orphaned imports**
- `web/src/routes/+layout.svelte:2` — `onDestroy` imported but unused (cleanup is now via `onMount` return)
- `web/src/routes/play/+page.svelte:30` — `removeHandler` imported but unused (no GAME_STATE handler registered here anymore)

**N2 — useless ternary**
File: `web/src/routes/play/+page.svelte:156`
```ts
handleKey(e.key === 'Backspace' ? 'Backspace' : e.key.length === 1 ? e.key : e.key);
```
Both branches of the inner ternary return `e.key`. Equivalent to `handleKey(e.key)`. Cleanup miss.

**N3 — empty onDestroy**
File: `web/src/routes/m/[token]/+page.svelte:82-84`
Empty body with comment. Can be removed entirely.

**N4 — direct field read style inconsistency**
File: `server/internal/ws/conn.go:203`
`deleteSession(conn.userID)` is read-safe (same goroutine as readLoop where auth_refresh runs synchronously) but inconsistent with the accessor convention introduced everywhere else. Prefer `conn.UserID()` for grep-discoverability.

**N5 — `await connect(...)` is redundant**
File: `web/src/routes/+layout.svelte:30`
`connect()` returns `void`, not Promise. The `await` resolves immediately. Harmless but misleading.

### Informational

**I1 — `displayName(c)` still returns UID as fallback**
File: `server/internal/ws/sync_match_handler.go:254-262`
Pre-existing; not Phase 01 scope. Phase 02 should redact this in logs and replace with cached profile name.

**I2 — `CompleteSync` returns no count**
Phase said "Mirror CompleteSync" — CompleteSync's transactional update has `state:"active"` filter (M5 fix) so it's already idempotent, but it doesn't surface ModifiedCount. Sync-mode IncrementStats in `match_room.go:226` is therefore not gated on whether the update actually modified a row. Risk: low (transactions + resolved-flag short-circuit prevent re-entry), but not symmetric with async path. Phase 06 test target.

**I3 — admin-claim revocation now flows through AUTH_REFRESH**
auth_refresh.go:57 now writes `isAdmin` under lock. Pre-fix it was set once in UpgradeHandler and never updated, so a server-side admin revocation persisted on the conn until disconnect. Net positive: closes a privilege-retention bug. No callers depend on isAdmin being immutable post-connect (verified: only `IsAdmin()` reads exist; no cached-value assumptions).

**I4 — grace timer captures Conn pointer**
disconnect.go:32-62 schedule closure pins `c *Conn` for 30s. If user reconnects with a new conn before the timer fires, the OLD conn's `c.UserID()` is still readable (RLock-safe). Cancel() is invoked from handleMatchRejoin via Cancel(matchID, cUID). Pre-existing pattern — verified safe.

## Strengths

- Race-detector clean (`go test -race ./...` 10/10 pass).
- svelte-check 0/0/0.
- RWMutex use is correct everywhere — no Lock-where-RLock-suffices and no RLock-where-Lock-needed.
- `untrack()` wrapper on $state initialisers in sync-game-scene.svelte:41-50 correctly avoids reactive-dep on prop initial values.
- `attempts.go` Insert keeps both layers (Find pre-check + WriteException 11000): cheap path for single-tx normal flow + safety net for concurrent retries.
- `checkAttemptDups` runs BEFORE CreateMany (indexes.go:19-21) — correct order; aborts cleanly if migration would be unsafe.
- `startSyncMatch` rollback (`a.setActiveMatchID("")` on Mongo error, sync_match_handler.go:218-219) preserves H2 fix while the new `cryptoSeed` error path bails before any state mutation.
- `+layout.svelte` `connected` local correctly handles the four transitions (null→user, user→null, user→user, null→null) — login-logout-login flips cleanly.

## Open follow-ups

- **Fix M1 + M2 before merging Phase 01.** Both are direct regressions of the H1 spec.
- **Phase 02 prep**: conn.go:203 direct field read — switch to accessor when redaction work touches the disconnect path.
- **Phase 06 (tests)**:
  - Regression test for fresh-tab `/quick-match` happy path (mock connectionState transitions).
  - Regression test for AUTH_REFRESH demoting `isAdmin` (was untestable pre-fix).
  - Race test for concurrent ATTEMPT_SUBMIT under unique-index — currently relies on transaction retry behaviour.
  - Symmetry test: CompleteSync ModifiedCount-gating (parallel to async Complete).
- **Phase 03 (UX)**: surface "connection lost — reconnecting" toast when scheduleReconnect bails or when token-fetch fails (currently only `console.warn`).
- **Plan TODO update**: mark steps 1-22 done in phase-01.md once M1/M2 are addressed.

## Unresolved questions

- Was `quick-match` deliberately omitted from the implementer's diff list, or did they assume the layout-hoist was sufficient? The phase explicitly listed it under Modify (line 50 of phase spec).
- Should `scheduleReconnect` reset `reconnectAttempt` to 0 on a successful subsequent `openSocket` (which fires onopen → line 104)? Currently yes, but the recursive token-fail branch never reaches openSocket, so the counter never resets in that loop.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** 22/22 steps landed and tests pass, but quick-match (M2) and scheduleReconnect token-fail loop (M1) are direct regressions of phase H1 intent — fix before merge.
