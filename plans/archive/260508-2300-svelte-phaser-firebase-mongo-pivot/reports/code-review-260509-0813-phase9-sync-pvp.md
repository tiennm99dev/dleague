# Phase 09 Code Review — Sync PvP over WebSocket

Pre-commit adversarial review. Read-only.

- HEAD: `9f66faa` (Phase 08)
- Diff: 17 modified + ~15 new files
- Build: `go build ./...` passes
- Tests: `go test -race -count=1 ./internal/ws/...` PASS
- Plan: `plans/260508-2300-svelte-phaser-firebase-mongo-pivot/phase-09-sync-pvp-websocket.md`

## Verdict

**DONE_WITH_CONCERNS** — landing-blockers: 1 High data race on `Conn.activeMatchID` and 1 High orphan-match window in `startSyncMatch`. Letters-never-leak invariant verified end-to-end (serialized-byte grep test is solid). Other findings are Med/Low and MVP-acceptable.

---

## Critical

None.

## High

### H1. Data race: `Conn.activeMatchID` mutated from multiple goroutines without synchronisation
- **Sites**: declared `server/internal/ws/conn.go:38`; read `:141`; written `match_room.go:247` (under `room.mu`); written `sync_match_handler.go:155, 205, 206` (no lock); read `disconnect.go:33` (no lock).
- **Race**: when a match resolves, `finishUnlocked` (room.mu held) clears `p.activeMatchID = ""`. **At the same time** the conn's defer in `conn.go:140-145` may execute `if conn.activeMatchID != "" → Schedule()`. No happens-before — read/write race on a `string` field (16 bytes on amd64 → torn read possible). Not caught by current tests because no test exercises both paths concurrently.
- **Impact**: tearing/inconsistency could yield a non-empty matchID read when value is mid-clear → spurious grace-timer scheduling for an already-resolved match (defended by `r.resolved` so harmless functionally but observable as wasted work). Race detector WILL fire under any load mixing match resolution with disconnects.
- **Fix**: add `sync.Mutex` to `Conn` and gate all reads/writes; OR use `atomic.Pointer[string]` / `atomic.Value`.

### H2. Orphan match window between `CreateSync` and `activeMatchID` assignment
- **Site**: `sync_match_handler.go:177-213`. `startSyncMatch` calls `MatchRepo.CreateSync` (Mongo round-trip ~10–50ms, p99 higher), THEN constructs Room, THEN `deps.Rooms.Add`, THEN `a.activeMatchID = matchID; b.activeMatchID = matchID`.
- **Race**: if conn `a` disconnects after `CreateSync` succeeds but before line 205, the conn's defer (`conn.go:137`) fires with `conn.activeMatchID == ""` → grace timer NOT scheduled. Match doc is in Mongo as `state="active"`, room is registered, B receives `QUEUE_MATCHED`. Player B will play alone for 5 minutes, then `HandleTimeout` resolves with `reason="timeout"` and `winner_uid=""` — B "loses" a match they should have won by forfeit.
- **Likelihood**: low (sub-100ms window) but real under network jitter / Mongo p99.
- **Fix**: set `a.activeMatchID` and `b.activeMatchID` BEFORE the long `CreateSync` call (use sentinel like `"pending:<gameID>"` if needed and overwrite with real matchID after); OR mark conn "in-match-pending" pre-CreateSync so the disconnect path can scrub the queue and abort.

## Medium

### M1. Queue re-insertion on `CreateSync` failure: only the last pusher gets an error envelope
- **Site**: `sync_match_handler.go:54-56`. On `startSyncMatch` error, both `a` and `b` are re-queued, but only `c` (the calling conn = the second pusher) receives 500. The first pusher (already returned `nil` from the prior handler call) gets no signal — they remain queued silently. Functional but confusing UX.
- **Fix (low)**: don't re-queue; send 500 to BOTH a and b explicitly. Cleaner contract.

### M2. `Queue.PopPair` does not check `userID` — pairs same user with self if two tabs queue
- **Site**: `queue.go:29-39`. If one user opens two tabs and both queue, `PopPair` pairs the user with themselves: `room.Players[0].userID == room.Players[1].userID`.
- **Impact**: `playerIndex` matches first slot only → opponent broadcasts go to player 0 (self). `IncrementStats` increments same UID once for win and once for loss. Confusing UX, no data corruption.
- **Fix**: in `PopPair`, skip pairs where `a.userID == b.userID` (defer to next push). Or reject duplicate at the handler.

### M3. `IncrementStats` moved OUTSIDE the `CompleteSync` transaction (agent-flagged carryover)
- **Site**: `match_room.go:217-229`. Spec called for atomic match+stats. Trade-off: stats are best-effort; on crash between Mongo commit and stats increment, leaderboard is inconsistent.
- **Acceptable for MVP** — stats are aggregate counters and Phase 10 leaderboard refresh will mask transient drift. Document in known-issues; consider a counter metric on stats-update failures.

### M4. `Queue.Remove` is O(n) per call, O(n²) for shutdown drain
- **Site**: `queue.go:43-59`. Negligible at MVP scale; document the ceiling. Phase 10 can use a `map[*Conn]struct{}` for O(1) removal.

### M5. `CompleteSync` UpdateOne lacks `state:"active"` filter — re-resolution overwrites silently
- **Site**: `matches.go:171-203`. The room's `r.resolved` flag is the only guard. Within one process, room.mu serializes all `finishUnlocked` paths, so safe today. Across processes (Redis migration v2): not safe.
- **Fix (defensive)**: filter `bson.M{"_id": oid, "state": "active"}`. Cheap and idempotent.

### M6. Queue 60s TTL is in spec but NOT implemented (carryover)
- **Spec line 126**, plan TODO line not checked. Player can sit in queue indefinitely. UX issue, not correctness. Phase 10.

### M7. `displayName()` returns Firebase UID not human name (carryover; Phase 10)
- **Site**: `sync_match_handler.go:235-243`. Acceptable for MVP; spec acknowledges UID-as-label is non-PII per Firebase stance, but contradicts the spirit of the security note ("never email/uid").

### M8. `handleMatchRejoin` does not notify the opponent that A reconnected
- **Site**: `sync_match_handler.go:128-149`. After rejoin, only the rejoining player gets `MatchRejoinAck`. Opponent's UI has no signal that A is back. Functional (server room state is shared) but UX gap.

### M9. `handleQueueJoin` returns `nil, nil` on success — no `QueueAck` envelope sent despite spec
- **Site**: `sync_match_handler.go:48,58`. The proto `QueueAck` is generated but only referenced via the compile-time anchor `var _ = (*dleaguev1.QueueAck)(nil)` (line 18). Spec line 39 promises an ack. Client uses fire-and-forget on join, so functionally OK; document deviation from spec.

### M10. `Match` BSON model has no `started_at` field; `CreateSync` only sets `created_at`
- **Site**: `models.go:53-72`, `matches.go:152-156`. `Room.StartedAt` is in-memory only and never persisted. Spec line 129 mentions `started_at:now`. Minor but breaks future audit.

## Low / Nit

- **L1**: `cryptoSeed()` fallback to `42` on `rand.Read` error (`sync_match_handler.go:255-263`). Linux crypto/rand failure is "should never happen", but if it does, every match gets seed=42 → same word repeatedly → trivially exploitable. Better: `log.Fatal` or return error.
- **L2**: `match_room.go:201` perfect-tie → player 0 wins. Deterministic but unfair. Spec said "ties broken by attempts ASC, then time ASC" — time not tracked at room layer. Phase 10.
- **L3**: `match_room.go:254` — `go deps.Rooms.Remove(...)` launches a goroutine to avoid a "deadlock". Registry lock is a sibling lock not held by `r.mu`; the goroutine launch is unnecessary. Harmless.
- **L4**: `Hub.CloseAll` (`hub.go:64-67`) calls `c.enqueue(503)` then `c.cancelRead()`. Race: writeLoop may select `<-ctx.Done()` before draining the just-enqueued frame. Sometimes 503 is not delivered. Acceptable for MVP graceful shutdown.
- **L5**: `disconnect.go:44-58` timer callback captures `*Conn` and `deps` — keeps stale Conn alive 30s after player gone. Tiny memory pressure, fine for MVP.
- **L6**: No "match resolved → cancel pending grace timer" hook. If player B disconnects, grace fires after natural resolution; `HandleForfeit` short-circuits via `r.resolved`. Defended; document.
- **L7**: `disconnect_test.go:29-46` bypasses `GraceTimers.Schedule(c, deps)` to use a 50ms `time.AfterFunc` directly. Does NOT exercise the real Schedule code path. **Coverage gap.**
- **L8**: `handleMatchRejoin` calls `oppWordle.ToProto()` twice (lines 147, 148) under lock. Redundant.
- **L9**: `+layout.svelte:62` calls `removeHandler(MessageType.MATCH_REJOIN_ACK)` but no `onMatchRejoinAck` is registered (rejoin uses `sendRequest` Promise correlation). Dead code; harmless.
- **L10**: `sync-game-scene.svelte:106-109` row growth assumes contiguous `attempt_num`. If a server-pushed progress is dropped (rate-limit deny on opponent's enqueue is impossible — limiter is on inbound), this is robust enough.
- **L11**: `match_room_test.go:135-147` letter-leak detection by byte-grep on `"CRANE"` in serialized payload — gold standard.
- **L12**: `MatchRejoinAck.opponent_hints` is `repeated WordleHint` (`match.proto:126`); WordleHint contains only `Colors`. **No letter leak via rejoin.** ✓
- **L13**: Auth gate (`auth_gate.go`) — all 8 sync types fall through default → require auth. ✓
- **L14**: `IncrementStats` filters anonymous via Mongo `is_anonymous != true` (`users.go:88`). Anonymous users may queue/play; stats are no-op. ✓
- **L15**: TS strictness in `ws.ts` — no `any`, protobuf-es v2 API used correctly (`create`/`toBinary`/`fromBinary`). ✓
- **L16**: `quick-match/+page.svelte:26-31` — `sendQueueLeave()` only on `searching=true`. After `QUEUE_MATCHED`, navigation pre-empts onDestroy → leave is correctly suppressed. ✓

## Letters-Never-Leak Verification ✓

Verified end-to-end:
- **`MatchOpponentProgress`** (`match.proto:101-105`, `match.pb.go:753-810`): only `match_id`, `attempt_num`, `colors []Color`. No string field. Server constructs at `match_room.go:107-111` using `colors` only.
- **Test** (`match_room_test.go:88-148`): byte-grep on serialized payload for `"CRANE"` — solid.
- **`MatchRejoinAck.opponent_hints`** (`match.proto:122-127`): `repeated WordleHint`, hint = `Colors []Color` only. No letter field.
- **`MatchResolved`**, **`QueueMatched`**: no letter fields. ✓
- Client doesn't bundle answers list (`web/src/lib/game/wordle/wordle.ts` is logic-only). Seed → solution requires server-side `Answers`. Cannot be derived client-side.

## Concurrency / Resource Leaks Summary

| Concern | Status |
|---|---|
| Room.HandleMove serialized via `r.mu` across full critical section | ✓ |
| `r.resolved` short-circuits late frames | ✓ |
| Disconnect grace Schedule/Cancel uses `g.mu` | ✓ but reads `c.activeMatchID` outside lock (H1) |
| Grace timer cleared on natural resolution | NO — self-fires after 30s; `r.resolved` guards forfeit |
| Hub.CloseAll: snapshot under RLock, then iterate | ✓ |
| Rate limiter per-conn; `mu` serializes | ✓ |
| Queue: single mutex, all ops under lock | ✓ |
| writeLoop / readLoop ctx coupling | ✓ |
| Timeout ticker tied to signal ctx | ✓ |
| Goroutine leak on grace success | timer self-cleans (`disconnect.go:46`) |
| `c.send` capacity 64; full → cancelRead | ✓ |

## Test Coverage

| Test | Verdict |
|---|---|
| `queue_test.go` | Solid: FIFO + concurrent + remove + empty. Missing: pair-with-self (M2). |
| `match_room_test.go` | Strong. Letters-leak grep + solve, exhaust, timeout, forfeit, idempotent resolution. Missing: simultaneous HandleMove from both players. |
| `rate_limiter_test.go` | Good: burst, refill, deny, concurrent. |
| `disconnect_test.go` | **Gap**: bypasses real `GraceTimers.Schedule` to use 50ms `time.AfterFunc` — does not exercise production Schedule path. |
| Handler-level integration test | **Missing entirely**. `handleQueueJoin` / `handleMatchMove` / `startSyncMatch` end-to-end untested. |

## Backwards-Compatibility / API Contracts

- Envelope MessageType extended with values 16–23. Existing types unchanged. ✓
- `Match` BSON model unchanged; new `seed`, `mode="sync"`, `state="active"`, `reason` values do not collide with Phase 08 reads. ✓
- `CompleteSync` writes `reason` field — new on doc; null on async docs (Mongo tolerates). ✓

## Build / Test Status

- `go build ./...`: PASS
- `go vet ./...`: PASS
- `go test -race -count=1 ./internal/ws/...`: PASS (1.4s)
- 200-match load test: NOT RUN (Phase 10).

---

## Recommended Actions Pre-Commit

1. **(H1)** Protect `Conn.activeMatchID` with `sync.Mutex` or `atomic.Pointer[string]`. Required for race-cleanliness under load.
2. **(H2)** Set `a.activeMatchID` / `b.activeMatchID` before the `CreateSync` Mongo call, so a mid-create disconnect triggers the grace path.

Both are small, surgical changes (~10 LOC). Other findings can land with this commit + Phase 10 follow-up.

## Phase 10 Follow-up

M2 (self-pair guard), M5 (`CompleteSync` state filter), M6 (60s queue TTL), M7+M10 (real display name lookup), L1 (cryptoSeed fail-loud), L7 (disconnect_test real-Schedule), M10/L17 (persist `started_at`), 200-match load test, handler-integration tests, simultaneous-move test, "opponent reconnected" UX signal (M8).

---

## Unresolved Questions

1. Should the queue 60s TTL be implemented now or punted to Phase 10? Spec mandates now; agent deferred.
2. Sync PvP for anonymous users — confirm acceptable risk (forfeit-farming via throwaway accounts)?
3. On `CreateSync` Mongo failure, is re-queueing both players the desired behaviour, or should both be 500'd and forced to retry manually?
4. `Match.metadata bson.M` is unused for sync — reserve for `started_at` / ELO delta or remove?
5. Should the opponent receive a "your opponent reconnected" notification on rejoin (M8)?

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Phase 09 implements sync PvP correctly in the happy path. Letters-never-leak invariant is verified end-to-end via serialized-byte grep. Two High-priority concurrency findings (H1: data race on `Conn.activeMatchID`; H2: orphan-match window during `CreateSync`) should be fixed before commit; remaining findings are MVP-acceptable.
**Concerns/Blockers:** H1 + H2 are landing-blockers. M1–M10 and L1–L16 can ship now with Phase 10 follow-ups documented.
