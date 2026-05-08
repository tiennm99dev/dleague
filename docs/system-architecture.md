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
TODO Phase 04. Collections: `users`, `games`, `matches`, `attempts`, `daily_puzzles`, `wordlists`, `leaderboards`. ERD Phase 10.

### Auth (Firebase Auth)
TODO Phase 05. Providers: Email/Password, Google, Anonymous.

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
