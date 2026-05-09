# Codebase Summary

Current state after Phase 10. All 10 phases complete.

## Repository Layout

```
dleague/
├── Dockerfile               — multi-stage: go-builder → node-builder → distroless runtime
├── fly.toml                 — Fly.io app config (app=dleague, region=iad)
├── go.work / go.work.sum    — Go workspace linking server/ + shared/
├── Makefile                 — dev, build, test, lint, deploy, proto-gen targets
├── docker-compose.yml       — local dev: mongo:7 + mongo-express
├── .github/
│   ├── workflows/ci.yml     — lint + test + build + proto check (all actions SHA-pinned)
│   └── dependabot.yml       — weekly updates: actions, gomod (server+shared), npm (web)
├── proto/
│   ├── dleague/v1/          — .proto schema files (envelope, wordle, match)
│   ├── buf.yaml             — buf lint + breaking config
│   └── buf.gen.yaml         — codegen: Go (shared/pb) + TS (web/src/lib/pb)
├── shared/                  — exported Go types (imported by server/)
│   ├── game/                — Game interface + Registry factory
│   └── pb/dleague/v1/       — generated Go protobuf (committed)
├── server/                  — Go HTTP + WebSocket server
│   ├── cmd/api/main.go      — entry point: boot wiring
│   ├── cmd/admin/main.go    — admin CLI: promote-admin, revoke-token
│   ├── cmd/seed-wordlists/  — one-shot: upload wordlists to Mongo
│   └── internal/
│       ├── auth/            — Firebase ID token verifier + Admin client
│       ├── config/          — env-var loader (Config, IsProduction)
│       ├── game/wordle/     — server-authoritative Wordle engine
│       ├── http/            — chi router, /health, static file server + SPA fallback
│       ├── scheduler/       — background: leaderboard refresh + match sweep
│       ├── store/           — Mongo per-collection repos + models + EnsureIndexes
│       └── ws/              — Hub, Conn, dispatch, game/match handlers, queue, rooms
├── web/                     — SvelteKit + Phaser client
│   ├── src/
│   │   ├── routes/          — SvelteKit pages (/, /play, /leaderboard, /quick-match, /sync, /m/[token])
│   │   └── lib/
│   │       ├── firebase.ts  — initializeApp, connectAuthEmulator, sign-in helpers
│   │       ├── auth-store.ts — writable<User|null>, idToken(), onAuthStateChanged
│   │       ├── ws.ts        — WS client: binary protobuf, reconnect, requestId correlation
│   │       ├── pb/          — generated TS protobuf (committed)
│   │       ├── game/        — wordle client types + colors.ts scoring
│   │       ├── phaser/      — event-bus, phaser-game.svelte, scenes/
│   │       └── components/  — sign-in, board, keyboard, connection-status
│   ├── static/              — favicon, sprites
│   ├── package.json
│   └── svelte.config.js
├── scripts/
│   ├── set-fly-secrets.sh   — documentation: fly secrets set commands (DO NOT execute)
│   ├── seed-wordlists.sh    — one-shot: upload wordlists to prod Mongo
│   └── promote-admin.sh     — one-shot: set Firebase admin custom claim
├── docs/                    — project documentation (all filled as of Phase 10)
└── plans/                   — implementation plans + reports
```

## Module Roles

### `shared/` — exported interfaces
- `game.Game` — pluggable game interface (`Validate`, `Apply`, `ToProto`, `Score`)
- `game.Registry` — factory: `Register(id, factory)`, `New(id) Game`
- `pb/dleague/v1/` — generated Go protobuf types (Envelope, WordleMove, WordleState, etc.)

### `server/internal/auth/`
- `Verifier` — Firebase ID token verification (hot WS path)
- `Admin` — privileged ops: `SetAdminClaim`, `RevokeRefreshTokens`, `VerifyIDTokenAndCheckRevoked`
- Credential chain: emulator → `credsPath` → ADC (Fly.io Workload Identity or `GOOGLE_APPLICATION_CREDENTIALS`)

### `server/internal/config/`
- `Config` — all runtime settings from env vars
- `Load()` — validates required fields; fails fast on missing `MONGO_URI`
- `IsProduction()` — case-insensitive check for "production"/"prod"

### `server/internal/store/`
- One repo struct per collection, constructed via `New*Repo(db)`
- `UserRepo` — upsert by Firebase UID; increment win/loss stats (idempotent on `ModifiedCount == 1`)
- `MatchRepo` — create async/sync matches; join; complete (with state filter); sweep expired. `Complete` is idempotent via `state:"pending"` guard.
- `AttemptRepo` — per-player guess log; handles unique constraint on `(match_id, player_uid)` via `ErrAttemptExists`
- `DailyPuzzleRepo` — date-keyed puzzle seed + solution
- `WordlistRepo` — answers + dictionary; fallback to embedded binary
- `LeaderboardRepo` — pre-computed ranking snapshots
- `EnsureIndexes` — 9 explicit indexes created at boot (idempotent), including unique compound index on attempts

### `server/internal/ws/`
- `Hub` — connection registry with `sync.RWMutex`; fan-out broadcast; max-conns cap
- `Conn` — one WS connection: read/write loops, send channel, auth fields (guarded by `mu`), rate limiter. Auth fields accessed via `UserID()`, `IsAnonymous()`, `IsAdmin()` methods for race safety.
- `dispatch` — routes `Envelope.Type` to handler; `requiresAuth` gate
- `Queue` — FIFO matchmaking with TTL eviction and self-pair guard; stale conns removed on disconnect
- `RoomsRegistry` — concurrent-safe map of live sync match rooms
- `MatchRoom` — per-match state: two Wordle engines, move handling, forfeit/timeout
- `GraceTimers` — 30 s disconnect grace via `time.AfterFunc`

### `server/internal/game/wordle/`
- `Wordle` — game state machine: `New`, `Validate`, `Apply`, `Score`, `ToProto`
- `Score` — two-pass color algorithm (correct-position first, then present)
- `EnsureToday` — idempotent daily puzzle seeding from SHA-256 seed
- `LoadAnswers` / `LoadDictionary` — Mongo first, embedded fallback

### `server/internal/http/`
- `NewRouter` — chi router: `/health` (JSON), `/ws` (WS upgrade), `/*` (static SPA)
- `spa_fallback.go` — returns `index.html` for unknown GET paths (client-side routing)

### `server/internal/scheduler/`
- `Run(ctx, cfg, repos)` — goroutine: leaderboard refresh every 5 min, match sweep every 15 min

## Build Commands

```bash
# Server
make dev                # go run ./server/cmd/api (port 8080)
make dev-debug          # with -tags debug (protojson logging)
make build              # go build → bin/dleague-server
make test               # go test -race ./shared/... ./server/...
make lint               # golangci-lint on shared/ + server/

# Web client
make web-install        # npm ci in web/
make web-build          # npm run build → web/dist/
make web-dev            # Vite dev server on :5173 (proxies /ws to :8080)

# Protobuf
make proto-gen          # buf generate → shared/pb/ + web/src/lib/pb/
make proto-lint         # buf lint
make proto-breaking     # buf breaking (against main branch)

# Infrastructure
make deploy             # fly deploy --remote-only (production)
make deploy-staging     # fly deploy --remote-only --app dleague-staging
make compose-up         # docker compose up -d (local Mongo)
make firebase-emulator  # start Firebase Auth emulator on :9099
make seed-wordlists     # upload wordlists to local Mongo
```

## Common Operations

```bash
# Run full test suite (requires Mongo + Firebase emulator).
MONGO_TEST_URI=mongodb://localhost:27017/dleague_test \
FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099 \
make test

# Regenerate protobuf after .proto changes.
make proto-gen && git diff --exit-code -- shared/pb web/src/lib/pb

# Check bundle size after web changes.
cd web && npm run build 2>&1 | grep -E "gzip|KB"

# Promote a user to admin.
bash scripts/promote-admin.sh <firebase-uid>

# View production logs.
fly logs --app dleague
```

## Key Files

| File | Purpose |
|------|---------|
| `server/cmd/api/main.go` | Boot wiring: decode secrets, connect Mongo, init Firebase, start Hub + HTTP server |
| `server/internal/ws/conn.go` | WS upgrade, read/write loops, auth token extraction, `tokenToProfile` |
| `server/internal/ws/hub.go` | Connection registry, dispatch routing |
| `server/internal/ws/queue.go` | Matchmaking FIFO with TTL eviction |
| `server/internal/ws/sync_match_handler.go` | Queue join/leave, match start, move/rejoin handlers |
| `server/internal/ws/match_room.go` | Per-match state: moves, forfeit, timeout, resolution |
| `server/internal/game/wordle/wordle.go` | Core game engine (server-authoritative) |
| `web/src/lib/ws.ts` | WS client: reconnect, requestId correlation, token refresh |
| `web/src/lib/auth-store.ts` | Firebase auth state reactive store |
| `proto/dleague/v1/` | Single source of truth for all message types |
