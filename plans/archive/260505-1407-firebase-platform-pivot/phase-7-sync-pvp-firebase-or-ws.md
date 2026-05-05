# Phase 7: Sync PvP — WebSocket transport + Firestore persistence

## Context Links
- Supersedes: `plans/260505-0947-dleague-pvp-game/phase-05-sync-pvp-websocket.md`
- Research transport tradeoff: `plans/reports/researcher-260505-1407-firebase-as-backend-feasibility.md` § Real-Time Sync (Realtime DB vs Firestore listeners vs Go WebSocket)
- Phase-6 match infra reused (match doc, attempts subcollection)
- Locked: Go WebSocket as primary sync transport; RTDB only if WS proves inadequate; Firestore as durable store at game end

## Overview
- **Priority:** P1 (second PvP mode; engagement core)
- **Status:** pending
- **Effort:** 4d
- Two players, same puzzle, race in real-time. Each guess broadcast to opponent (without revealing the actual guess word — only attempt count + feedback color summary). Server sees both, computes server-authoritative timing, declares winner first to solve. Final match state persisted to Firestore at game end.

## Key Insights
- **Why Go WS, not Firestore listeners:** Firestore listeners cost 1 read per delta per subscriber. 2 players × 6 guesses each × 2 listeners = 24 reads PER MATCH. At 50 daily sync matches that's 1200 reads/day — burns 2.4% of read budget on real-time sync alone. WS = free (already running)
- **Why not RTDB:** RTDB free-tier 100 simultaneous connections cap. WS hub easily handles thousands; RTDB would force migration at modest growth. WS already validates auth (phase-2 AUTH_HELLO). Keep one transport
- **Server-authoritative timing:** server records `serverNowMS` on every guess; "first to solve" is decided by server's monotonic clock, not client wall-clocks. Eliminates lag-induced unfairness debates
- **Privacy of opponent guesses:** opponent should see "guess #3 submitted" not the actual word — prevents copying mid-game. Send only `feedback_summary` (count of greens/yellows/grays, not positions)
- **Matchmaking queue:** simple FIFO queue server-side; first 2 unmatched players paired. No skill-based MMR (out of scope per parent plan)
- **Reconnection:** if a player disconnects mid-match, opponent waits up to 60s; if no reconnect, opponent wins by default
- **NO Realtime DB use this phase.** RTDB infrastructure stays provisioned (phase-1) but unused; reserved as escape hatch if WS proves inadequate at scale (>500 concurrent matches — well past free-tier exit anyway)

## Requirements

### Functional

#### Wire format additions
- `MESSAGE_TYPE_QUEUE_JOIN = 18` — request: `{game_id}` → joins matchmaking queue
- `MESSAGE_TYPE_QUEUE_MATCHED = 19` — server pushes when paired: `{match_id, opponent_uid, opponent_display_name}`
- `MESSAGE_TYPE_QUEUE_LEAVE = 20` — request: leave queue
- `MESSAGE_TYPE_OPPONENT_PROGRESS = 21` — server pushes on opponent guess: `{match_id, opponent_uid, attempts_used, summary: {green, yellow, gray}, won}`
- `MESSAGE_TYPE_MATCH_FORFEIT = 22` — request OR push (disconnect): `{match_id, reason}`

(Reuses MATCH_RESULT from phase-6.)

#### Sync match lifecycle
1. Client A: `QUEUE_JOIN{game_id: 'wordle'}`
   - server: append A to in-memory queue per game
   - if queue len < 2: A waits
2. Client B: `QUEUE_JOIN{game_id: 'wordle'}`
   - server: pop A and B from queue; create match (kind=sync); link both conns to match
   - server → both: `QUEUE_MATCHED{match_id, opponent_uid, opponent_name}`
3. Both clients send `GAME_START{match_id}`; standard puzzle flow
4. Each `GAME_GUESS` from one is processed normally (phase-5):
   - server validates + persists guess+feedback for that player
   - server computes summary `{green, yellow, gray}` for that guess
   - server pushes `OPPONENT_PROGRESS` to OTHER conn (not the guesser)
5. First player to win:
   - server marks attempt won + records `won_at_server_ms`
   - server pushes `GAME_END` to winner immediately
   - opponent continues until they finish or forfeit
6. Both done → server picks winner by `(won, attempts_used, won_at_server_ms ASC)`; pushes `MATCH_RESULT` (phase-6)
7. Mid-match disconnect:
   - server detects WS close; starts 60s grace timer
   - if reconnect within 60s + same uid + valid AUTH_HELLO + same match_id: resume
   - else: forfeit; opponent wins by default; persist match completed

### Non-functional
- OPPONENT_PROGRESS push latency <100ms p95 (server local broadcast)
- Queue match within 30s of second player joining (just FIFO; no skill matching)
- 60s reconnect grace window enforced server-side
- Per-match in-memory state <2KB (just refs to 2 conns + match_id)

## Architecture

### Files to create

#### Server
- `server/internal/matchmaking/queue.go` — FIFO queue per game_id (~80 LOC)
- `server/internal/matchmaking/queue_test.go` (~80 LOC)
- `server/internal/matchmaking/manager.go` — `Manager{queues, activeMatches}; Join; Leave; OnConnClose` (~120 LOC)
- `server/internal/sync_match/session.go` — runtime state for a sync match (2 conns, match_id, grace timer) (~120 LOC)
- `server/internal/sync_match/broadcast.go` — `BroadcastOpponentProgress(session, fromUID, summary)` (~60 LOC)
- `server/internal/sync_match/forfeit.go` — `HandleForfeit(session, uid)` + grace timer (~80 LOC)
- `server/internal/ws/handlers/queue_join.go` (~60 LOC)
- `server/internal/ws/handlers/queue_leave.go` (~30 LOC)
- `server/internal/ws/handlers/forfeit.go` (~50 LOC)

#### Client
- `web/src/components/queue-screen.tsx` — "Searching for opponent..." spinner + cancel button (~60 LOC)
- `web/src/components/sync-game-screen.tsx` — own grid + opponent's tile-summary indicator (~150 LOC)
- `web/src/components/opponent-progress.tsx` — small component: opponent's row count + summary dots (~50 LOC)
- `web/src/hooks/use-sync-match.ts` — wires QUEUE → MATCHED → game flow (~120 LOC)

### Files to modify
- `proto/dleague/v1/envelope.proto` — add 5 message types
- `server/internal/ws/conn.go` — add `match_id string` to Conn for cross-broadcast lookup
- `server/internal/ws/hub.go` — register queue + forfeit handlers
- `server/internal/ws/handlers/game_guess.go` (phase-5) — after persisting feedback, if match.kind=='sync' call `BroadcastOpponentProgress`
- `web/src/App.tsx` — add "Quick Match" button → goes to QueueScreen
- `web/src/ws/client.ts` — typed helpers for queue messages

### In-memory data structures
- `Manager.queues: map[gameID]*list.List` (FIFO; mutex-protected)
- `Manager.activeMatches: map[matchID]*Session` (each Session holds *Conn for both players)
- `Manager.userToMatch: map[uid]matchID` (for reconnect lookup)

All in-memory. Restart on server crash → all sync matches drop. Clients see `MATCH_FORFEIT{reason: 'server_restart'}` on reconnect when their match doesn't exist anymore. Acceptable at testing scale.

## Implementation Steps

### Server
1. Add 5 message types to envelope.proto + regen
2. Implement matchmaking/queue.go: thread-safe FIFO; `Push`, `Pop`, `Remove(conn)`, `Len`
3. Implement matchmaking/manager.go:
   - `Join(conn, gameID)`: lock; if queue has another waiter → pop both, call `manager.startMatch(connA, connB, gameID)`; else push self
   - `startMatch`: create match doc (Firestore via `matches.create`), link both conns to session, push QUEUE_MATCHED
   - `Leave(conn)`: remove from any queue
   - `OnConnClose(conn)`: if conn was in queue → remove; if conn was in active match → start grace timer
4. Implement sync_match/session.go: state machine; tracks both conns + match_id + start_unix_ms
5. Implement sync_match/broadcast.go: lookup other conn via session, marshal + send OPPONENT_PROGRESS
6. Implement sync_match/forfeit.go: 60s timer; on expire, declare opponent winner via matches/result.go (phase-6)
7. WS handlers for queue_join, queue_leave, forfeit; wire dispatch
8. Modify game_guess.go to call BroadcastOpponentProgress when match.kind=='sync'
9. Modify conn.go close path to call `manager.OnConnClose(c)`
10. Tests: queue_test (concurrent push/pop), session_test (mock conns + assert broadcast)

### Client
1. Regen protobuf-ts
2. <QueueScreen/>: button "Quick Match"; on click sends QUEUE_JOIN; spinner; on QUEUE_MATCHED transitions to <SyncGameScreen/>
3. <SyncGameScreen/>: like <GameScreen/> from phase-5 but with sidebar showing opponent's progress (rows + green/yellow/gray dot summary)
4. <OpponentProgress/>: receives OPPONENT_PROGRESS via use-sync-match hook
5. use-sync-match: wires WS message handlers; manages match phase enum
6. Cancel queue: leave button → QUEUE_LEAVE
7. Smoke test: 2 browser tabs, anon sign-in, both click Quick Match, both get matched, race to solve

## Todo List

### Server
- [ ] Add 5 message types to envelope.proto + regen
- [ ] matchmaking/queue.go + tests
- [ ] matchmaking/manager.go
- [ ] sync_match/session.go + broadcast.go + forfeit.go
- [ ] WS handlers (queue_join, queue_leave, forfeit) + hub wiring
- [ ] Hook game_guess.go → BroadcastOpponentProgress when kind==sync
- [ ] Hook conn.go close → manager.OnConnClose
- [ ] Tests (concurrent queue, session broadcast, forfeit grace)
- [ ] go build + lint + make test

### Client
- [ ] Regen protobuf-ts
- [ ] <QueueScreen/>
- [ ] <SyncGameScreen/>
- [ ] <OpponentProgress/>
- [ ] use-sync-match hook
- [ ] Smoke test 2-tab race

## Success Criteria
- [ ] Two clients on same browser (different tabs, different anon UIDs) match within 30s of both clicking Quick Match
- [ ] Each guess on one tab triggers OPPONENT_PROGRESS visual update on other tab within 200ms
- [ ] Opponent's guess word NOT visible (only attempts_used + summary counts)
- [ ] First to win triggers MATCH_RESULT correctly (server-authoritative timing)
- [ ] Mid-match tab close → 60s grace; opponent wins on timer expiry
- [ ] Reconnect within 60s with fresh ID token + same match_id resumes session (best-effort; doc state authoritative)
- [ ] After match end, persisted state in `/matches/{id}` allows post-game review
- [ ] Server restart drops in-flight matches; clients see clean error message
- [ ] No file >200 LOC

## Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Server crash drops all in-flight matches | Med | Low | Document; testing scale = trivial. Sync matches by design ephemeral |
| Race in queue: same conn joins twice | Low | Low | Manager.Join checks `userToMatch` first; idempotent |
| Forfeit grace timer leak after match ends | Med | Med | Cancel timer in `endMatch` and on conn close cleanup |
| OPPONENT_PROGRESS reveals too much (regex on summary leaks info) | Low | Low | Summary is just (green, yellow, gray) counts — no positional info; matches Wordle public conventions |
| Both win simultaneously (same attempts_used + same server_ms) | Very low | Low | Tiebreak: lower-uid wins (deterministic); document |
| Queue starvation if game_id has only 1 player ever | High | Low | UI shows "still searching..." with leave button; accept |
| Memory bloat from in-flight match map | Low | Low | Each session ~2KB; 100 concurrent = 200KB; trivial |
| Reconnect resume conflicts with grace timer | Med | Med | Resume cancels timer atomically inside Manager mutex |
| Cross-tab same-uid sync match (same anon UID in 2 tabs) | Med | Low | Reject second QUEUE_JOIN if userToMatch[uid] exists; clearer error |

## Security Considerations
- All WS messages already auth-gated via phase-2 AUTH_HELLO; sync handlers trust `conn.uid`
- Server validates conn.uid is participant before broadcasting/persisting any sync action
- OPPONENT_PROGRESS contents reviewed: only metadata (counts), no answer/guess leakage
- Forfeit message can be CLIENT-initiated → server must validate conn.uid matches match participant
- Reconnect resume: re-validate ID token via standard AUTH_HELLO; `match_id` is participant-scoped via Firestore doc check
- Rate-limit QUEUE_JOIN per uid (max 5/min) to prevent queue spam

## Next Steps
- **Unblocks:** phase-9 (parent phase-05 marked superseded)
- **Future:** Realtime DB fallback (out of scope this phase; document escape hatch)

## Unresolved Questions
1. Should QUEUE_JOIN scope to game_id or be global? MVP: per game_id; trivial change later
2. Should opponent SEE that I just typed (mid-guess) or only after submission? MVP: only after submission (simpler, less spammy)
3. Should we show opponent's tile colors but not letters? Privacy debate. MVP: only counts; revisit per UX
4. Spectator mode (3rd party watches a sync match)? Out of scope
5. Re-queue option after match end without leaving sync screen? Defer to UX iteration
