---
phase: 6
title: "Wire-format auth handshake (protobuf)"
status: pending
priority: P1
effort: "0.5d"
dependencies: [5]
---

# Phase 6: Wire-format auth handshake

## Context Links

- Plan: [plan.md](plan.md)
- Existing wire definitions: `shared/proto/`
- Phase 1 of original project plan: protobuf + WS ping-pong (already shipped)

## Overview

Extend the protobuf wire format with an `AUTH` envelope. WS upgrade no longer authenticates immediately on the HTTP request — instead the connection opens, client sends `AUTH{idToken}` as the first frame; server verifies and either acks or closes with reason. After ack, the connection is bound to a UID for the rest of its lifetime.

## Key Insights

- HTTP-Bearer-on-upgrade is harder for browsers (custom headers don't survive `WebSocket()` constructor). First-frame auth is the conventional pattern.
- Existing `MessageEnvelope` in `shared/proto/` carries a `type` discriminator + payload bytes. Add new `AUTH_REQUEST` and `AUTH_RESPONSE` types.
- Server holds connection open ≤5s waiting for AUTH; closes with code 4001 (unauthenticated) on timeout/invalid.

## Requirements

- Functional: client → AUTH_REQUEST → server verifies → AUTH_RESPONSE{ok:true,uid} or close.
- Non-functional: handshake completes <100 ms after token-cache is warm; connection's UID is immutable for life of conn.

## Architecture

```
[Client]                         [Server]
   │      WS upgrade              │
   │ ───────────────────────────► │
   │      ws.Conn opens           │
   │ ◄─────────────────────────── │
   │                              │
   │ AUTH_REQUEST{idToken}        │
   │ ───────────────────────────► │
   │                              │ auth.Verify(token)
   │                              │ → claims
   │                              │ cb.users.UpsertIfMissing(claims)
   │                              │ conn.uid = claims.UID
   │ AUTH_RESPONSE{ok,uid}        │
   │ ◄─────────────────────────── │
   │                              │
   │  ... game frames ...         │
```

## Related Code Files

- Modify:
  - `shared/proto/dleague.proto` — add `AUTH_REQUEST`, `AUTH_RESPONSE`, regenerate Go bindings
  - `server/internal/ws/conn.go` — handshake state machine, auth-then-game phases
  - `server/internal/ws/hub.go` — track per-conn UID
  - `server/cmd/api/main.go` — pass `*auth.Firebase` into WS upgrade options
- Create:
  - `server/internal/ws/handshake.go` — first-frame auth logic (or inline in conn.go)
  - `server/internal/ws/handshake_test.go` — covers timeout, invalid-token, valid-token paths

## Implementation Steps

1. Edit `shared/proto/dleague.proto`: add `MessageType.AUTH_REQUEST = N`, `AUTH_RESPONSE = N+1`. Define `AuthRequest{string id_token}` and `AuthResponse{bool ok; string uid; string error}`.
2. Regenerate: `make proto` (existing target).
3. Update `ws.Hub` to optionally carry `Auth *auth.Firebase` and to expose per-conn UID.
4. Modify `conn.go` `readPump`: on first frame, expect AUTH_REQUEST; on success store UID; on failure close 4001.
5. After handshake, conn enters game phase; AUTH frames received later are ignored or treated as protocol error.
6. Tests: spin up hub with fake verifier; assert close-on-timeout, close-on-bad-token, ok-on-good-token.

## Todo List

- [ ] Proto AUTH messages added + regenerated
- [ ] Hub tracks per-conn UID
- [ ] Conn handshake state machine
- [ ] 5s handshake timeout enforced
- [ ] Tests cover all three paths
- [ ] Existing ping-pong test still passes

## Success Criteria

- [ ] Valid client connects, sends AUTH, gets ack with UID, exchanges ping-pong
- [ ] Client that never sends AUTH within 5s → server closes 4001
- [ ] Client with bad token → server closes 4001
- [ ] `go test ./server/internal/ws/...` green

## Risk Assessment

- **Race between hub.Register and AUTH** — register pre-auth or post-auth? Decision: post-auth (register only after handshake completes). Pre-auth conns aren't in the hub's broadcast pool.
- **Handshake DoS** — opening 1000 conns and never sending AUTH ties up goroutines for 5s. Mitigation: per-IP connection cap at WS-upgrade.
- **Protocol drift** — AUTH frame schema must stay stable. Version field in AuthRequest gives forward-compat.

## Security Considerations

- Token in WS frame is plaintext on the wire — TLS mandatory (Coolify provides).
- Don't log full tokens.
- Reject tokens with mismatched `aud` (handled by Admin SDK auto when `FIREBASE_PROJECT_ID` matches).

## Next Steps

Phase 7 client implements the handshake on the Svelte side (`src/net/ws.ts`). Phase 10 uses the per-conn UID for room authorization.

## Unresolved Questions

- Should handshake timeout be configurable via env? Default 5s; revisit if mobile networks need longer.
- Heartbeat ping: keep at 30s as today, or shorten to detect dead conns faster? Defer.
