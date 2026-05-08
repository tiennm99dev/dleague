---
phase: 9
title: "Sync PvP (live race over WebSocket)"
status: pending
priority: P2
effort: 1.5w
dependencies: [8]
---

# Phase 09 — Sync PvP

## Context Links
- `plans/archive/260505-0947-dleague-pvp-game/phase-05-sync-pvp-websocket.md` (mine for spec; rebase to single-WS + protobuf + Mongo)
- `plans/reports/researcher-260508-2300-mongodb-atlas-go.md` §5 (transactions for atomic match-end)
- `server/internal/ws/handlers/match.go` (Phase 08)
- `server/internal/store/{matches.go,attempts.go}` (Phase 08)
- `proto/dleague/v1/match.proto` (extend with sync messages)
- `server/internal/ws/hub.go` (existing dispatch + per-conn `send` channel from Phase 02)

## Overview
Add real-time head-to-head Wordle: matchmaking queue (in-memory, single-region MVP), live opponent progress (color squares only — never letters), server-authoritative timing. First to solve wins; ties broken by attempts ASC, then time ASC. Atomic match-end recording via Mongo transaction. 30-second disconnect grace, then forfeit.

## Key Insights
- **In-memory queue + match rooms:** single-region only at MVP. Redis pub/sub deferred to v2.
- **Server-authoritative timing:** server timestamps every guess; client clocks ignored for tie-break (closes the "sync PvP fairness" risk in old plan §5).
- **Letters never leaked:** opponent receives only color hints + attempt number. Never the guess word.
- **Atomic match end:** Mongo transaction on `matches` (state=complete, winner_uid) + both `users` (stats) — research §5 confirms M0 supports.
- **Disconnect grace:** Conn close → 30s timer; if conn re-registers (same userID, same matchID) → resume. Otherwise opponent wins by forfeit.
- **Reconnect on page reload:** client stashes active `matchID` in `sessionStorage`; on reconnect, `MatchRejoin{match_id}` re-binds to the room.
- **Match timeout:** 5 min hard cap (resolves mongo report unresolved Q1) — both lose if neither wins by 5min.
- **Per-conn rate limit:** 10 messages/sec; drop excess. (Phase 02 deferred this here.)

## Requirements
**Functional:**
- New proto messages in `match.proto`:
  - `QueueJoin{string game_id}` → `QueueAck{string queue_id}`
  - `QueueLeave{}` → ack via `MESSAGE_TYPE_OK` or `ERROR`
  - `QueueMatched{string match_id; int64 seed; string opponent_display_name}` (server-pushed, fire-and-forget)
  - `MatchMove{string match_id; string guess}` → server replies with own state + broadcasts opponent-view to opposite peer
  - `MatchOpponentProgress{int32 attempt_num; repeated Color colors}` (server-pushed)
  - `MatchResolved{string match_id; string winner_uid; string reason}` where reason ∈ `solved`, `exhausted`, `forfeit`, `timeout`.
  - `MatchRejoin{string match_id}` → `MatchRejoinAck{state, opponent_progress}` for reconnect.
- New WS message types in envelope: `QUEUE_JOIN=17`, `QUEUE_LEAVE=18`, `QUEUE_MATCHED=19`, `MATCH_MOVE=20`, `MATCH_OPPONENT_PROGRESS=21`, `MATCH_RESOLVED=22`, `MATCH_REJOIN=23`, `MATCH_REJOIN_ACK=24`.
- `server/internal/ws/queue.go`: in-memory FIFO queue per game_id with mutex. On second user arrival → pair, create match doc (mode="sync", state="active"), broadcast `QUEUE_MATCHED` to both via their `Conn.send`.
- `server/internal/ws/match_room.go`: per-match room tracking 2 conn pointers, both wordle states, started_at, deadline (5min). Handler emits opponent-progress on each move.
- `server/internal/ws/disconnect.go`: on `Conn` close, if user is in active match, start 30s grace timer; on expiry → opponent wins by forfeit (transaction).
- `MatchRepo` extensions: `CreateSync(ctx, p1, p2, seed) → matchID`, `CompleteSync(ctx, matchID, winnerUID, reason)` (transactional).
- Per-conn rate limiter: token bucket, 10 msg/sec, drop on overflow with `MESSAGE_TYPE_ERROR{429}`.
- Client: `/quick-match` route — queue UI, then on `QUEUE_MATCHED` navigate to sync game scene with opponent panel.
- Client: sync game scene shows two grids side-by-side; opponent grid shows only colors + attempt number (no letters).
- Client: reconnect logic — `sessionStorage.activeMatchID`; on reconnect WS, send `MATCH_REJOIN` if value present.
- Server graceful shutdown: hub `CloseAll(reason)` drains active matches, persists state, closes conns. Closes Phase 1 edge case.

**Non-functional:**
- WS message latency target <150ms p95 server→client (single Fly region).
- Server tolerates 1k concurrent matches (2k conns) on Fly.io shared-cpu-1x.
- Each new file <200 LOC.
- Match transaction p95 <50ms.
- Per-conn rate limiter <1µs overhead.

## Architecture
```
queue:
  Conn A → QUEUE_JOIN(wordle)        Conn B → QUEUE_JOIN(wordle)
              ↳ queue.Push(A)                    ↳ queue.PopPair(A,B)
                                                 ↳ matches.CreateSync(p1=A, p2=B, seed=rand)
                                                 ↳ rooms[matchID] = Room{A,B,wordle1,wordle2}
                                                 ↳ A.send <- QUEUE_MATCHED{matchID, seed, opponent=B.name}
                                                 ↳ B.send <- QUEUE_MATCHED{matchID, seed, opponent=A.name}

play:
  A → MATCH_MOVE{matchID, guess="CRANE"}
       ↳ rateLimiter.allow(A) → if no, ERROR{429}
       ↳ room.handleMove(A, guess):
            ├─ rooms[matchID].wordleA.Apply(guess) → state, hints
            ├─ A.send <- WordleState (full, own letters visible)
            ├─ B.send <- MATCH_OPPONENT_PROGRESS{attempt_num, colors only}
            ├─ if A wins → resolve(A)
            └─ if A out of attempts → check if B already lost too → exhaustion-tie
  resolve(winner):
       ↳ matches.CompleteSync (transaction: match state, both users stats)
       ↳ both.send <- MATCH_RESOLVED{winner_uid, reason}
       ↳ delete rooms[matchID]

disconnect:
  Conn close while in room:
       ↳ start 30s grace timer
       ↳ if reconnect+MATCH_REJOIN within 30s → resume
       ↳ else: resolve(other_player, reason="forfeit")

timeout (5min):
  rooms ticker every 1s checks deadline
  on expiry: resolve with reason="timeout", winner = whoever has more attempts solved (or both lose)
```

## Related Code Files
**Create:**
- `server/internal/ws/queue.go`
- `server/internal/ws/match_room.go`
- `server/internal/ws/match_rooms_registry.go` (`map[matchID]*Room` + mutex)
- `server/internal/ws/disconnect.go` (grace timers)
- `server/internal/ws/rate_limiter.go` (per-conn token bucket)
- `server/internal/ws/handlers/sync_match.go` (handlers for new message types)
- `server/internal/ws/queue_test.go`
- `server/internal/ws/match_room_test.go`
- `server/internal/ws/rate_limiter_test.go`
- `web/src/routes/quick-match/+page.svelte`
- `web/src/lib/components/sync-game-scene.svelte`
- `web/src/lib/components/opponent-panel.svelte`

**Modify:**
- `proto/dleague/v1/match.proto` — 8 new messages
- `proto/dleague/v1/envelope.proto` — 8 new MessageType enum values
- `server/internal/ws/hub.go` — wire queue + rooms registry; `CloseAll(reason)`
- `server/internal/ws/conn.go` — rate-limit gate before dispatch; on close hook for grace timer
- `server/internal/store/matches.go` — `CreateSync`, `CompleteSync`
- `server/cmd/server/main.go` — pass queue/rooms into hub; rooms-tick goroutine for timeouts
- `web/src/lib/ws.ts` — handle server-pushed messages (QUEUE_MATCHED, MATCH_OPPONENT_PROGRESS, MATCH_RESOLVED) via `onMessage`
- `web/src/routes/+page.svelte` — title scene adds "Quick Match" button
- `docs/system-architecture.md` — fill sync PvP section

**Delete:** none.

## Implementation Steps
1. **Proto:** add 8 messages + 8 envelope enum values. `make proto-gen`.
2. **Queue (`queue.go`):** struct `Queue{mu sync.Mutex; entries map[string][]*Conn}`. Methods: `Push(gameID, conn)`, `PopPair(gameID) (a, b *Conn, ok bool)`. On 2nd push for same gameID → call PopPair. 60s queue TTL — if no match in 60s, kick with `ERROR{queue_timeout}`.
3. **Match-room registry (`match_rooms_registry.go`):** `map[string]*Room` keyed by matchID. Methods `Add`, `Get`, `Remove`. `mu sync.RWMutex`.
4. **Match-room (`match_room.go`):** `Room{matchID; players [2]*Conn; wordles [2]*wordle.Wordle; deadline time.Time}`. Method `HandleMove(conn, guess) error` — validates conn is one of the players, applies move, sends self-state + opponent-progress, resolves if terminal.
5. **MatchRepo `CreateSync`:** InsertOne match with `mode:"sync"`, `state:"active"`, `started_at:now`, `seed`, `players:[uid1,uid2]`.
6. **MatchRepo `CompleteSync`:** session.WithTransaction:
   - matches.UpdateOne (state, winner_uid, ended_at, reason).
   - users.UpdateOne (winner.stats.wins++) — skip if anonymous.
   - users.UpdateOne (loser.stats.losses++) — skip if anonymous.
7. **Disconnect grace (`disconnect.go`):** `OnClose(conn *Conn)` — if conn has active matchID, start 30s timer. On expiry without reclaim → `Room.HandleForfeit(loserUID)` → `CompleteSync(reason="forfeit")` → broadcast `MATCH_RESOLVED`.
8. **Rate limiter (`rate_limiter.go`):** per-conn token bucket, refill 10 tokens/sec, max 10. `Allow() bool`. Called in `Conn.handle` before dispatch. On deny → `MESSAGE_TYPE_ERROR{429}` enqueued.
9. **Timeout ticker:** background goroutine in `main.go`: every 1s, iterate rooms; if `room.deadline < now` → `ResolveTimeout(matchID)`.
10. **Hub `CloseAll(reason)`:** iterate `hub.conns`, send `MESSAGE_TYPE_ERROR{server_shutdown, reason}`, close conn. Called from main.go shutdown signal handler.
11. **Conn integration:** `OnClose` callback hooked into `defer hub.unregister(c)`. If `c.activeMatchID != ""`, schedule grace timer.
12. **Handler dispatch:** new `handlers/sync_match.go` — `handleQueueJoin`, `handleQueueLeave`, `handleMatchMove`, `handleMatchRejoin`. Hub.dispatch routes by enum.
13. **Client `/quick-match`:** on mount, send `QUEUE_JOIN`. UI shows "Searching..." with cancel button (sends `QUEUE_LEAVE`). On `QUEUE_MATCHED` push, save `matchID` to sessionStorage, navigate to sync game.
14. **Client `sync-game-scene.svelte`:** two-pane layout. Left: own Board + Keyboard. Right: opponent panel rendering color squares per attempt. Listens for `MATCH_OPPONENT_PROGRESS` events. On terminal `MATCH_RESOLVED` → results screen.
15. **Client `opponent-panel.svelte`:** displays `attempts_count` of color rows; never receives letters.
16. **Reconnect:** `+layout.svelte` on auth-state-changed connect-flow extension: after WS connects, if `sessionStorage.activeMatchID`, send `MATCH_REJOIN`. On `MATCH_REJOIN_ACK`, restore game state. On error → clear sessionStorage + redirect home.
17. **Tests:**
    - `queue_test.go`: pair on 2nd push; FIFO order; concurrent pushes safe under -race.
    - `match_room_test.go`: HandleMove correct color responses; opponent progress emitted with no letter content (assert structure); first-to-solve wins; exhaustion → both lost; timeout resolution.
    - `rate_limiter_test.go`: burst handling, refill, deny path.
    - `disconnect_test.go`: 30s grace; reclaim within window; forfeit if not.
    - End-to-end load test: 200 concurrent matches with `vegeta` + custom Go harness; assert p95 <200ms.
18. **Manual smoke:** two browsers (incognito + regular) → both /quick-match → matched <2s → race → first solver wins → both see resolution.

## Todo List
- [ ] Proto: 8 sync messages + envelope values
- [ ] In-memory queue
- [ ] Match-room + registry
- [ ] CreateSync + CompleteSync (transaction)
- [ ] Disconnect grace timer
- [ ] Per-conn rate limiter
- [ ] Timeout ticker
- [ ] Hub.CloseAll on shutdown
- [ ] sync_match handlers
- [ ] /quick-match route
- [ ] sync-game-scene.svelte
- [ ] opponent-panel (colors only)
- [ ] Reconnect + MATCH_REJOIN
- [ ] queue_test, match_room_test, rate_limiter_test, disconnect_test
- [ ] 200-match load test
- [ ] Manual smoke (two browsers)

## Success Criteria
- [ ] Two queue arrivals matched in <2s, both load same seed
- [ ] Each guess: opponent panel updates with colors only within 200ms
- [ ] Letters never traverse to opponent (verified by inspecting WS frames)
- [ ] First to solve wins; ties broken by attempts then time
- [ ] Disconnect mid-match → 30s grace; if no reclaim, opponent wins by forfeit
- [ ] Reload mid-match → MATCH_REJOIN restores state
- [ ] 5-minute match timeout fires correctly
- [ ] Per-conn rate limit: 11+ msg/sec → ERROR{429}
- [ ] 200-match load: p95 latency <200ms
- [ ] Server graceful shutdown: active matches finish or get clean error

## Risk Assessment
| Risk                                                   | Likelihood | Impact | Mitigation                                                       |
|--------------------------------------------------------|------------|--------|------------------------------------------------------------------|
| Race: both players solve same tick                     | Medium     | Medium | Server-side `started_at + monotonic offset` deterministic order. |
| Opponent grid leaks letters via timing                 | Low        | High   | Only emit final hint array; rate-limit per-attempt to 1 event.   |
| Hub goroutine leak on crash                            | Medium     | Medium | All goroutines tied to hub ctx; shutdown closes via `CloseAll`.  |
| Queue deadlock on odd player count                     | High       | Low    | 60s queue TTL → kick with friendly error.                        |
| Single-region scaling ceiling                          | High       | Low    | Document; Redis migration plan in v2.                            |
| sessionStorage matchID stale → bad rejoin              | Medium     | Low    | Server returns clean error → client clears storage.              |
| Mongo transaction abort under high contention          | Low        | Medium | WithTransaction retries; log retry count.                        |

## Security Considerations
- WS upgrade requires valid Firebase token (Phase 05).
- Per-conn rate limit: max 10 msg/sec; closes flood / churn DoS.
- No PII in opponent broadcast — only display_name, never email/uid (uid leak via debug envelope is acceptable; display_name is intended).
- Forfeit cannot be triggered by opponent — only by self disconnect or server timeout.
- Validate every incoming message shape; unknown subtypes ignored (envelope already does this).
- Anonymous users may queue; if anonymous user wins, no stats update (transaction skip).

## Next Steps
- Phase 10 — Deploy + polish — depends on full game loop landed here.
