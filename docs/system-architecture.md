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
│   │  │ store.Store (composed)         │    │                │
│   │  └────────┬───────────────────┬───┘    │                │
│   └───────────┼───────────────────┼────────┘                │
│               │                   │                          │
│       ┌───────▼─────┐      ┌──────▼──────┐                  │
│       │ Couchbase   │      │ Redis 8.4   │                  │
│       │ Community   │      │ AOF, ZSETs  │                  │
│       │ 8.0         │      │             │                  │
│       └─────────────┘      └─────────────┘                  │
│       (internal:8091)      (internal:6379)                   │
└─────────────────────────────────────────────────────────────┘
              │                                ▲
              │                                │
              ▼                                │
        ┌──────────────────────────────────────┴────────┐
        │ Firebase Auth (Spark plan)                    │
        │ Email/Password, Google, Anonymous             │
        └───────────────────────────────────────────────┘
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
7. Server replies AUTH_RESPONSE{ok, uid}
8. Client transitions to "connected"; presence MarkOnline (Redis)
```

### Async daily puzzle (Phase 9)

```
Lobby → "Play today's puzzle" → GameRunner.svelte
  → GET /api/v1/puzzles/me/today (Bearer; returns puzzle.word)
  → GET /api/v1/attempts/me/{date} (resume, 404 if none)
  → Phaser game starts WordleScene with solution + resume
  → User makes guesses; scoring.ts evaluates client-side for UX
  → On win/lose: EventBus emits attempt-complete
  → POST /api/v1/attempts {date, guesses}
  → Server re-scores via api.Score; UpsertAttempt + SubmitScore
  → Leaderboards update in Redis ZSETs
```

### Sync PvP (Phase 10)

WS-mediated; both players' guesses stream to a hub goroutine that interleaves
state updates and broadcasts results. Auth gates the WS upgrade.

## Migration seam

```
HTTP handlers ─┐
WS hub        ─┤
Sync PvP      ─┴──► store.Store interface (server/internal/store/store.go)
                      │
                      ├── memstore         (tests; in-memory)
                      └── composed         (production)
                            ├── couchbase  (gocb v2)
                            └── redis      (go-redis v9)
```

`gocb` and `go-redis` are confined to their own packages so a future swap to
Capella / Atlas / managed Redis costs one wiring line in `cmd/api/main.go`.

## Security model

- **Auth boundary:** every protected HTTP route uses `auth.Middleware`; every WS connection completes the AUTH handshake before any frame is processed.
- **Solution leak:** public `/puzzles/{date}` returns hint/length/difficulty only. Authenticated `/puzzles/me/{date}` returns the full puzzle for client-side per-guess feedback. Server re-scores in `/attempts.submit` so a tampered client cannot inflate its score.
- **Internal services:** Couchbase + Redis bound to docker-compose internal network; only `:8080` exposed to the host.
- **Service-account JSON:** injected as env var (`FIREBASE_CREDENTIALS_JSON`); never committed.

## Observability (post-beta)

Currently minimal — `/health` returns "ok". Future additions: request log middleware, Prometheus metrics, structured logger via `slog`.
