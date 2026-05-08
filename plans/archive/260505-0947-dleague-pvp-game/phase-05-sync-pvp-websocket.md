---
phase: 5
title: "Sync PvP (WebSocket)"
status: pending
priority: P2
effort: 2w
dependencies: [4]
---

# Phase 5: Sync PvP (WebSocket)

## Overview

Add real-time head-to-head: matchmaking queue, live opponent progress (blurred guesses or attempt count, not letters), server-authoritative race timing. This is the marquee competitive mode — first to solve wins, ties broken by attempts.

## Requirements

- **Functional:**
  - Matchmaking: queue → pair two players → both load same seed
  - WebSocket connection per player; server hub broadcasts opponent events
  - Each guess submission relayed: opponent sees "Guess 3/6" + colored squares (no letters)
  - First to win or first to exhaust 6 attempts → match resolved server-side
  - Surrender / disconnect = forfeit after 30s grace
  - Reconnect within 30s resumes; otherwise forfeit
  - Player can choose: "Quick match" (random) or "Custom" (room code)
- **Non-functional:**
  - WebSocket message latency target <150ms p95
  - Server tolerates 1k concurrent matches (2k connections) on single Fly.io shared-cpu-1x
  - Graceful shutdown drains active matches
  - Single-source-of-truth: server tracks both players' attempts; client UI is reactive

## Architecture

**Hub pattern (server):**

```
ws.Hub
 ├── Register(conn)        // attach session
 ├── Join(matchID, conn)   // bind to match room
 ├── Broadcast(matchID, evt)
 └── Tick()                // disconnect timeouts, match expiry

ws.Match
 ├── Players [2]Player
 ├── State (waiting | playing | finished)
 ├── Seed
 ├── ApplyGuess(playerID, guess) -> events
 └── ResolveWinner()
```

**Message protocol (JSON over WS):**

```
client → server:
  {type: "queue.join", game: "wordle"}
  {type: "queue.cancel"}
  {type: "match.guess", matchId, guess: "CRANE"}
  {type: "match.surrender", matchId}
  {type: "ping"}

server → client:
  {type: "queue.matched", matchId, seed, opponent: {displayName}}
  {type: "match.opponentGuess", attemptNum, hints: ["GREEN","GRAY",...]}  // colors only, no letters
  {type: "match.youGuess", hints, won}
  {type: "match.resolved", winnerId, reason: "solved"|"exhausted"|"forfeit"}
  {type: "error", code, message}
```

**Schema additions:**

```sql
ws_matches (extends matches table)
  -- reuses matches table, adds:
  ALTER TABLE matches ADD COLUMN match_kind text DEFAULT 'async';  -- 'async' | 'sync'
  ALTER TABLE matches ADD COLUMN started_at timestamptz;
  ALTER TABLE matches ADD COLUMN forfeit_by uuid;
```

**Matchmaking queue (in-memory MVP):**
- Single-region only at MVP — Go map + mutex
- Future: Redis pub/sub when multi-region

## Related Code Files

**Create:**
- `server/internal/ws/hub.go`
- `server/internal/ws/match.go`
- `server/internal/ws/queue.go`
- `server/internal/ws/protocol.go` (message types)
- `server/internal/ws/conn.go` (read/write pumps, auth)
- `server/internal/http/ws_handler.go` (upgrade endpoint with session check)
- `server/internal/store/migrations/0006_sync_matches.sql`
- `client/internal/net/ws_client.go` (Go WebSocket via `nhooyr.io/websocket` or `gorilla/websocket` js bridge)
- `client/internal/scene/queue.go` (matchmaking screen)
- `client/internal/scene/sync_game.go` (game scene with opponent panel)
- `client/internal/ui/opponent_panel.go` (live progress squares)
- `shared/dto/ws.go` (protocol types reused)

**Modify:**
- `server/internal/http/router.go` (mount /ws)
- `client/internal/scene/title.go` (add "Quick Match" entry)

## Implementation Steps

1. Migration adding `match_kind`, `started_at`, `forfeit_by` columns
2. `nhooyr.io/websocket` server upgrade with session-cookie auth
3. Hub: register/unregister with per-connection write goroutine
4. Queue: simple FIFO, pair on second arrival, persist match row, broadcast `queue.matched`
5. Match: server validates each guess via `shared/game/wordle`, tracks state for both, emits events
6. Hide opponent letters: send only color hints, never the actual guess word
7. Server-authoritative win: first valid solve wins regardless of network ordering (server timestamp, not client)
8. Disconnect handling: 30s reconnect window, then `forfeit_by` set + match resolved
9. Heartbeat ping/pong every 15s; close idle conns
10. Client: Ebitengine + JS bridge for WebSocket (`syscall/js` to browser WS)
11. Sync game scene: 2-pane layout (you on left, opponent on right with mini grid)
12. Reconnect logic: stash matchId in localStorage; resume on page reload
13. Load test: simulate 200 concurrent matches with `vegeta` or custom Go harness

## Todo List

- [ ] WS migration columns
- [ ] WS hub + connection pump
- [ ] Auth on upgrade
- [ ] Queue matchmaking
- [ ] Match state machine + protocol
- [ ] Server-authoritative guess validation
- [ ] Opponent-hint privacy (no letters leaked)
- [ ] Disconnect/forfeit grace
- [ ] Client WS bridge from WASM
- [ ] Queue scene + sync game scene
- [ ] Reconnect resume
- [ ] Load test 200+ matches

## Success Criteria

- [ ] Two browsers in queue → matched <2s → both load same puzzle
- [ ] Each player's guess appears on opponent's panel as colored squares within 200ms
- [ ] Letters never leak (verified by inspecting WS frames)
- [ ] First to solve wins; ties broken by attempts asc, then duration
- [ ] Disconnect → 30s grace → opponent gets win if no reconnect
- [ ] Refresh during match → resume same match state
- [ ] Load test: 200 concurrent matches, p95 latency <200ms

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| WASM WebSocket instability | Use browser-native WS via `syscall/js`, not Go-side library |
| Race: both players solve same tick | Server timestamp tie-break with monotonic clock; document in protocol |
| Opponent grid leaks letters via timing | Only emit final hint array, never per-letter; rate-limit per-attempt to 1 event |
| Hub goroutine leaks on crash | Defer-close all writers; supervised by parent context |
| Queue deadlock if odd player count | Timeout queued players after 60s with friendly message |
| Single-region scaling ceiling | Document as known limit; Redis migration plan in v2 |

## Security Considerations

- WS upgrade requires valid session cookie (rejected otherwise)
- Per-connection rate limit: max 10 messages/sec, drop excess
- Validate every incoming message shape; reject unknown types
- No PII in opponent broadcast — only display name, never email/id
- Forfeit cannot be triggered by opponent — only by self or server timeout
