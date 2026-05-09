# System Architecture

**Status:** skeleton — diagrams + ERD landed by Phase 10.

## High-level

```
┌──────────────────────────┐         WebSocket           ┌──────────────────────────┐
│   Browser                │   binary protobuf envelope  │   Go server (Fly.io)     │
│ ┌──────────────────────┐ │ ◄───────────────────────►   │ ┌──────────────────────┐ │
│ │ SvelteKit (static)   │ │                             │ │ chi router           │ │
│ │   + Phaser canvas    │ │                             │ │   /health, /, /ws    │ │
│ │   + protobuf-es      │ │                             │ │ ws hub + dispatch    │ │
│ │   + firebase JS SDK  │ │ ─── Sec-WebSocket-Protocol ─┤ │ Firebase verifier    │ │
│ └──────────────────────┘ │     fb.<id_token>           │ │ Mongo repos          │ │
└──────────────────────────┘                             │ └──────────────────────┘ │
            │                                            └──────────────────────────┘
            │ ID token verify                                    │
            ▼                                                    ▼
   ┌──────────────────┐                              ┌──────────────────────┐
   │ Firebase Auth    │                              │ MongoDB Atlas (M0)   │
   │ (Google managed) │                              │ replica set, TLS     │
   └──────────────────┘                              └──────────────────────┘
```

TODO Phase 10: Mermaid version + deployment topology.

## Components

### Client (SvelteKit + Phaser)
TODO Phase 06.

### Server (Go)
TODO Phase 02–05.

### Persistence (MongoDB Atlas M0)

Driver: `go.mongodb.org/mongo-driver/v2` (v2.6.0+). One `*store.Client` per process; pool max 100. Atlas TLS is implicit via `mongodb+srv://`. Transactions are supported on M0's 3-node replica set.

#### Collections

| Collection | `_id` type | Purpose |
|---|---|---|
| `users` | Firebase UID string | Player profile + embedded stats |
| `games` | slug string ("wordle") | Game-type registry |
| `matches` | ObjectID | One PvP or solo match instance |
| `attempts` | ObjectID | Per-player guess log within a match |
| `daily_puzzles` | "YYYY-MM-DD" string | Daily puzzle seed + solution hash |
| `leaderboards` | "{game}_{period}_{date}" string | Pre-computed ranking snapshots |

All documents carry `schema_version: 1` for lazy in-place migration (Option A).

#### Indexes (8 explicit, created by `store.EnsureIndexes` at boot)

| Collection | Keys | Options |
|---|---|---|
| `users` | `display_name ASC` | unique |
| `matches` | `players ASC` | — |
| `matches` | `created_at DESC` | — |
| `matches` | `state ASC, created_at DESC` | — (ESR compound) |
| `attempts` | `match_id ASC` | — |
| `attempts` | `match_id ASC, player_uid ASC` | — |
| `daily_puzzles` | `_id DESC` | — |
| `leaderboards` | `game_id ASC, period_end DESC` | — |

`EnsureIndexes` is idempotent — re-runnable on every boot without error.

### Auth (Firebase Auth)

Firebase Auth is the sole identity provider. The Go server verifies Firebase ID tokens using the Admin SDK (`firebase.google.com/go/v4/auth`). No session cookies; no server-side sessions.

#### WebSocket upgrade flow (Pattern A)

```
Client                                          Server (UpgradeHandler)
──────                                          ───────────────────────
firebase JS SDK: user.getIdToken() → idToken
WS open: ws://.../ws
  Sec-WebSocket-Protocol: dleague.v1, fb.<idToken>
                                    ──────────────►
                                                  1. extractFirebaseToken(header)
                                                     → reject 401 if missing/malformed
                                                  2. verifier.VerifyIDToken(ctx, idToken)
                                                     → reject 401 on exp/sig failure
                                                  3. conn.userID = token.UID
                                                     conn.isAnonymous (sign_in_provider)
                                                     conn.tokenExpiresAt
                                                  4. userRepo.UpsertByUID(uid, profile)
                                                     (idempotent Mongo upsert — non-fatal)
                                                  5. websocket.Accept → 101 Switching
```

#### Auth gate (every dispatch)

```go
if requiresAuth(env.GetType()) && c.userID == "" {
    return errorEnvelope(req_id, 401, "unauthenticated"), nil
}
```

Messages exempt from auth: `UNSPECIFIED`, `PING`, `PONG`, `AUTH_REFRESH`, `AUTH_REFRESH_ACK`, `ERROR`. All future game/match message types require auth by default.

#### Token refresh (at ~50 min)

Firebase ID tokens expire after 1 hour. Clients send `AuthRefresh{id_token}` before expiry:

```
Client                                          Server (hub.dispatch)
──────                                          ─────────────────────
AuthRefresh{id_token: newToken}  ─────────────►
                                                verifier.VerifyIDToken(newToken)
                                                → on error: ERROR{401} + close conn
                                                → on ok: update conn.userID,
                                                         conn.tokenExpiresAt
                                                AuthRefreshAck{expires_at_unix}  ◄──
```

#### Anonymous users

Firebase Anonymous Auth issues a UID with `firebase.sign_in_provider == "anonymous"`. The server sets `conn.isAnonymous = true` and creates a Mongo user doc with no email/displayName. Anonymous users are excluded from leaderboards (Phase 08).

#### Credential management

| Environment | Method |
|---|---|
| Local dev | `FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099` (no creds needed) |
| CI | Same as local dev; emulator started in GH Actions before tests |
| Production (Fly.io) | Workload Identity via ambient OIDC; no JSON file |
| Dev/staging (non-Fly) | `GOOGLE_APPLICATION_CREDENTIALS=/path/to/serviceAccount.json` |

Service-account JSON files are `.gitignore`d (`serviceAccount*.json`). Never committed.

Revocation check (`VerifyIDTokenAndCheckRevoked`) deferred to Phase 10.

## Wire format
- **Envelope:** see `proto/dleague/v1/envelope.proto` (single oneof payload + request_id correlation).
- **Auth:** ID token piped via `Sec-WebSocket-Protocol: dleague.v1, fb.<id_token>` at upgrade.
- **Refresh:** client sends `AuthRefresh{id_token}` ~50 min into a connection.
- **Errors:** server emits `MESSAGE_TYPE_ERROR` envelope on malformed input — does NOT close the connection.

## Concurrency model
- One goroutine per WS connection for reads.
- One writer goroutine per connection drains a bounded `send` channel (Phase 02).
- Hub fans out broadcasts to per-conn channels with non-blocking sends (drop on slow client).

## Failure domains
TODO Phase 10. Atlas pause, Firebase outage, Fly region failure, etc.

## Security boundaries
TODO Phase 10. WS origin allowlist, conn cap, request_id length cap, security headers, etc.

## Observability
TODO Phase 10. Structured logs, metrics, tracing (if any).
