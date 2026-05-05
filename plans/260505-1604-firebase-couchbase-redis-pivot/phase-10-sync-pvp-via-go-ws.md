---
phase: 10
title: "Sync PvP via Go WS (auth-gated)"
status: pending
priority: P2
effort: "1d"
dependencies: [4, 6]
---

# Phase 10: Sync PvP via Go WS

## Context Links

- Plan: [plan.md](plan.md)
- Existing WS hub: `server/internal/ws/{hub,conn,ping}.go`
- Prior superseded: `260505-1407-firebase-platform-pivot/phase-7-sync-pvp-firebase-or-ws.md`

## Overview

Reuse the existing Go WS hub. Add a "room" concept for paired sync PvP. Use the per-conn UID (from Phase 6 handshake) for room authorization. Optional Redis pub/sub for future multi-instance fan-out — not implemented yet (single Coolify VM instance).

## Key Insights

- Hub already handles connection lifecycle + ping-pong. Only adds: room registry + per-room broadcast.
- Match docs (Couchbase `matches` collection, key=`<matchId>`) carry `players: [uid1, uid2]`. Server checks UID is in the list before joining the room.
- Sync PvP = real-time turn submission; cadence ≥1 turn / 2s.
- Final result of the sync match is persisted via `store.UpsertMatch` (Couchbase) + `store.SubmitScore` (Redis) — handlers don't import either backend directly.

## Requirements

- Functional: client joins room via WS frame `JOIN_ROOM{matchId}`; server verifies UID ∈ match.players; on success forwards turn frames to other player; on match end persists results.
- Non-functional: turn-frame round-trip <50 ms in-region; <300 ms cross-region.

## Architecture

```
hub.go          (existing) — manages Conns
rooms.go        (new) — manages map[matchId]*Room; each Room is a small set of Conns
match_lifecycle.go (new) — JOIN_ROOM, TURN, MATCH_END handlers; persist to CB on end
```

Wire frames added (extend protobuf):
- `JOIN_ROOM{matchId}` → `JOIN_ROOM_ACK{ok,role:p1|p2,opponentUid}` or close 4003 (forbidden)
- `TURN{matchId,turn}` → forwarded to opponent + appended to match doc
- `MATCH_END{matchId,winnerUid}` → server validates, persists, ZADD leaderboard, broadcasts

## Related Code Files

- Create:
  - `server/internal/ws/rooms.go`
  - `server/internal/ws/match_lifecycle.go`
  - `server/internal/ws/rooms_test.go`
- Modify:
  - `shared/proto/dleague.proto` — add JOIN_ROOM, TURN, MATCH_END
  - `server/internal/ws/conn.go` — dispatch frame types post-handshake
  - `server/internal/ws/hub.go` — expose room registry

## Implementation Steps

1. Add proto messages, regenerate.
2. `rooms.go`: `Room{matchId; conns []*Conn; createdAt}`. Hub-level `JoinRoom(conn, matchId)` calls `store.GetMatch(matchId)`, verifies conn.UID, registers conn into the room.
3. `match_lifecycle.go`: handlers for the three new frame types.
4. On `MATCH_END`: validate winner is in players, `store.UpsertMatch(updated)`, `store.SubmitScore` for both leaderboards.
5. Integration test with two fake conns: open, AUTH, JOIN_ROOM, exchange TURN, MATCH_END; assert CB+Redis state.

## Todo List

- [ ] Proto frames added
- [ ] Room registry + lifecycle
- [ ] JOIN_ROOM authorization (UID ∈ players)
- [ ] TURN forwarding
- [ ] MATCH_END persistence + leaderboard
- [ ] Integration test
- [ ] Presence (Phase 4) integration: SADD on AUTH, SREM on disconnect

## Success Criteria

- [ ] Two clients with valid match doc: join room, exchange ≥10 turns, end match, both see leaderboard updated
- [ ] Client not in players: JOIN_ROOM rejected with 4003
- [ ] One side disconnects: other side notified; match marked `abandoned` after 60s

## Risk Assessment

- **Disconnect handling** — what if both sides disconnect? Match stays open. Mitigation: 5-min stale-room reaper.
- **Cheating via TURN forgery** — server should validate turn legality against match state, not just forward. v1 forwards; v2 validates.
- **Coolify proxy WS quirks** — verify proxy doesn't drop long-lived conns.

## Security Considerations

- TURN frames carry game state — client can't forge another player's UID since per-conn UID is server-set in handshake.
- Rate-limit TURN frames per conn: max 5/sec (anti-spam).
- Match doc immutable to clients post-creation (only Phase 9's matchmaking endpoint creates them; covered in v2).

## Next Steps

Phase 11 deploys the whole stack to Coolify.

## Unresolved Questions

- Matchmaking endpoint (`POST /matches/queue`) — out of scope for v1; defer. v1 assumes match docs are created admin-side or via dev console.
- Spectator mode — defer.
