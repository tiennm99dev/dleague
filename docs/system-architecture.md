# System Architecture

## Stack overview

```
┌─────────────────────────────────────────────────────────────┐
│  Client (Svelte 5 + Phaser 4 + Capacitor)                   │
│                                                              │
│  ┌────────────────┐    ┌────────────────────────────────┐  │
│  │ Svelte shell   │    │ Phaser 4 canvas (per-variant)  │  │
│  │ Lobby / SignIn │◄──►│  WordleScene, …                │  │
│  │ HUD overlays   │    │  via game/EventBus.ts          │  │
│  └────────────────┘    └────────────────────────────────┘  │
│         │                          │                        │
│  ┌──────▼──────┐           ┌───────▼────────┐              │
│  │ Firebase JS │           │ ws.ts client + │              │
│  │ SDK (Auth)  │           │ /api fetch     │              │
│  └─────────────┘           └────────────────┘              │
└─────────│───────────────────────────│───────────────────────┘
          │ID token                   │Bearer / WS frames
          ▼                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Coolify VM (OCI A1 Flex, ARM64)                            │
│                                                              │
│   Internet ── port 8080 ──┐                                  │
│                            ▼                                 │
│   ┌────────────────────────────────────────┐                │
│   │  Go server (cmd/api)                   │                │
│   │  ┌─────────────┐  ┌────────────────┐   │                │
│   │  │ chi HTTP    │  │ nhooyr.io WS   │   │                │
│   │  │ /api/v1/*   │  │ /ws            │   │                │
│   │  └──────┬──────┘  └────────┬───────┘   │                │
│   │         │                  │           │                │
│   │  ┌──────▼─────── auth ─────▼───────┐   │                │
│   │  │ Firebase Admin SDK token verify │   │                │
│   │  └─────────────────┬───────────────┘   │                │
│   │                    │                   │                │
│   │  ┌─────────────────▼──────────────┐    │                │
│   │  │ store.Store (mongodb impl)     │    │                │
│   │  └────────────────┬───────────────┘    │                │
│   └───────────────────┼────────────────────┘                │
└───────────────────────┼─────────────────────────────────────┘
                        │ TLS (SCRAM-SHA-256)
                        ▼
              ┌──────────────────────┐
              │ MongoDB Atlas (M0)   │
              │ ap-southeast-1       │
              │ collections:         │
              │   users / puzzles /  │
              │   attempts / matches │
              │   leaderboards /     │
              │   presence / cache   │
              └──────────────────────┘

              ┌──────────────────────┐
              │ Firebase Auth        │
              │ Spark plan           │
              │ Email / Google /     │
              │ Anonymous            │
              └──────────────────────┘
```

## Key flows

### Sign-in & first WS connect

```
1. User taps "Sign in with Google" in SignIn.svelte
2. Firebase JS SDK returns User + ID token
3. Lobby.svelte mounts → opens WS to /ws
4. Client sends AUTH_REQUEST envelope (token in body)
5. Server: auth.Gate verifies via Firebase Admin SDK
6. On first verify per UID: UpsertUserOnFirstAuth stamps
   isBetaTester=true + betaSignupAt; subsequent calls no-op those
   ($setOnInsert in MongoDB)
7. Server replies AUTH_RESPONSE{ok, uid}
8. Client transitions to "connected"; presence MarkOnline writes
   {_id: uid, expireAt: now+ttl} to Mongo (TTL index purges).
```

### Async daily puzzle

```
Lobby → "Play today's puzzle" → GameRunner.svelte
  → GET /api/v1/puzzles/me/today (Bearer; returns puzzle.word)
  → GET /api/v1/attempts/me/{date} (resume, 404 if none)
  → Phaser game starts WordleScene with solution + resume
  → User makes guesses; scoring.ts evaluates client-side for UX
  → On win/lose: EventBus emits attempt-complete
  → POST /api/v1/attempts {date, guesses}
  → Server re-scores via api.Score; UpsertAttempt + SubmitScore
  → Leaderboards update via $max-on-doc in Mongo
```

### Sync PvP

WS-mediated; both players' guesses stream to a hub goroutine that
interleaves state updates and broadcasts results. Auth gates the WS upgrade.

## Migration seam

```
HTTP handlers ─┐
WS hub        ─┤
Sync PvP      ─┴──► store.Store interface (server/internal/store/store.go)
                      │
                      ├── memstore  (tests; in-memory)
                      └── mongodb   (production; Atlas)
```

Imports of `go.mongodb.org/mongo-driver/v2` are confined to
`internal/store/mongodb/`. `make grep-isolation` enforces that boundary in
CI. A future swap (e.g. away from Atlas) ships as a third sibling impl.

## Atomicity contracts (MongoDB-backed)

- **`SubmitScore`** — `updateOne({board, uid}, {$max: {score}}, upsert:true)`. `$max` is atomic at the single-doc level; concurrent submits serialize on the doc and the highest score wins.
- **TTL purge** — Mongo's TTL background scan runs ~every 60s, so a doc with `expireAt = now+30s` may live up to ~90s. Every read on `presence`/`cache` therefore includes `expireAt: {$gt: now()}` to mask the lag and return an accurate liveness answer regardless of physical purge.
- **`CacheSet(ttl=0)`** — parity with Redis SET (no EX) and `memstore`: stores the value and `$unset` any prior `expireAt`. Doc persists indefinitely. `CacheGet` accepts both expireAt-bearing and expireAt-absent docs.

## Security model

- **Auth boundary:** every protected HTTP route uses `auth.Middleware`; every WS connection completes the AUTH handshake before any frame is processed.
- **Solution leak:** public `/puzzles/{date}` returns hint/length/difficulty only. Authenticated `/puzzles/me/{date}` returns the full puzzle for client-side per-guess feedback. Server re-scores in `/attempts.submit` so a tampered client cannot inflate its score.
- **Atlas access:** SCRAM-SHA-256 over public TLS. IP allowlist set per `docs/atlas-setup.md` (`0.0.0.0/0` during beta — auth still required; tighten before non-beta launch).
- **Service-account JSON:** injected as env var (`FIREBASE_CREDENTIALS_JSON`); never committed.

## Observability (post-beta)

Currently minimal — `/health` returns "ok" if Mongo `Ping` succeeds. Future
additions: request log middleware, Prometheus metrics, structured logger via
`slog`, Atlas alerting on connection-count + slow-query breaches.
