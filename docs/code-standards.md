# Code Standards

Patterns and constraints enforced in the dleague codebase. Updated for the Firebase Auth + Couchbase + Redis + Svelte/Phaser stack.

## File & Module Structure

### Go Files

- **Max 200 LOC per file** — Split early. Prefer small, focused modules.
- **kebab-case directory names** when there's a useful disambiguator (`store/composed/`).
- **snake_case Go filenames** (`router_test.go`, `cb_init.sh`).
- **Module layout:**
  - `cmd/{appname}/main.go` — single entry point per binary (`cmd/api`, `cmd/dleague-export`).
  - `internal/` — unexported packages.
  - `shared/pb/` — generated protobuf code (committed).
- **Store-interface boundary** is load-bearing for migration:
  - `gocb` import only inside `internal/store/couchbase/`.
  - `go-redis` import only inside `internal/store/redis/`.
  - Composed store wires both behind `store.Store`.
  - Memstore impl ships alongside as test backend + proof of seam.

### Frontend (Svelte 5 + Phaser 4) Files

- **Pluggable -dle variants** under `client/web/src/games/<name>/` — each variant exports a `GameVariant` (Scene class + Hud component).
- **Phaser scenes** in PascalCase (`WordleScene.ts`) — matches Phaser ecosystem.
- **Svelte components** in PascalCase (`WordleHud.svelte`, `Lobby.svelte`).
- **Pure utilities** in lowercase (`scoring.ts`, `registry.ts`, `eventbus-helpers.ts`).
- **Shared events** routed through `game/EventBus.ts` — typed wrappers in `games/runner/eventbus-helpers.ts`.

### Directory Structure

```
dleague/
├── client/web/                   # Svelte 5 + Phaser 4 + Capacitor
│   └── src/
│       ├── auth/                 # Firebase JS SDK wrapper
│       ├── components/           # Svelte UI: Lobby, SignIn, BetaBanner
│       ├── game/                 # Phaser game shell + EventBus
│       ├── games/                # Pluggable -dle variants (Phase 8)
│       │   ├── types.ts          # GameVariant interface
│       │   ├── registry.ts       # lazy variant loader
│       │   ├── runner/           # GameRunner + EventBus helpers
│       │   └── wordle/           # First concrete variant
│       └── net/                  # WS client + protobuf
├── server/
│   ├── cmd/api/                  # Main HTTP/WS server
│   ├── cmd/dleague-export/       # Migration export CLI (Phase 12)
│   └── internal/
│       ├── api/                  # Async-PvP REST under /api/v1
│       ├── auth/                 # Firebase Admin token verifier
│       ├── config/
│       ├── http/                 # Top-level router + health
│       ├── store/                # Migration seam
│       │   ├── store.go          # Store interface
│       │   ├── couchbase/        # gocb impl
│       │   ├── redis/            # go-redis impl
│       │   ├── composed/         # wires couchbase + redis
│       │   └── memstore/         # in-memory impl for tests
│       └── ws/                   # WebSocket hub
├── shared/pb/                    # Generated protobuf
├── proto/dleague/v1/             # .proto sources
├── docs/
└── plans/
```

## Wire Format & Serialization

### Protobuf (WebSocket frames)

- **Binary format always** — `proto.Marshal(msg)` over WS.
- **Schema sources** in `proto/dleague/v1/`.
- **Generated code** committed to git (`shared/pb/dleague/v1/*.pb.go`).
- **Validation:** `buf lint` + `buf breaking` in CI.

### REST (async-PvP HTTP API)

- **JSON over HTTP** under `/api/v1/`.
- **Auth:** `Authorization: Bearer <firebase-id-token>`; verified by `auth.Middleware`.
- **Public endpoints** never leak puzzle solutions; auth'd `/puzzles/me/*` returns the full puzzle (server still re-scores in `/attempts`).

### Debug Logging

- **Production build** (default `go build`): binary protobuf only.
- **Debug build** (`go build -tags debug`): every WS message also logged as `protojson`.
- **Implementation:** build-tag conditionals (`debug_log.go` / `debug_log_noop.go`).

## Transport & Network

- **WebSocket** for sync PvP and presence (`/ws`).
- **REST** for async daily puzzle / attempts / leaderboards (`/api/v1/*`).
- **Auth handshake:** first WS frame is `AUTH_REQUEST` containing the Firebase ID token; HTTP API uses Bearer header per request.
- **Reconnect:** client refreshes the ID token before each reconnect (1h Firebase expiry).

## Game Architecture

### Pluggable variants (Phase 8)

```typescript
interface GameVariant {
  key: string;                    // 'wordle', 'sumdle', etc.
  Scene: typeof Phaser.Scene;
  Hud: typeof SvelteComponent;
  meta: { title: string; difficulty: 'easy'|'medium'|'hard'; tagline: string };
}
```

- Variants register in `client/web/src/games/registry.ts` as lazy imports.
- `GameRunner.svelte` fetches puzzle + resume state, mounts the scene, mounts the HUD, posts the final attempt.
- Adding a new variant = copy `wordle/` folder + register in `registry.ts`.

## Code Quality Rules

### Testing

- **Unit tests** in `*_test.go` (Go) / `*.test.ts` (TypeScript via vitest).
- **Coverage target:** >70% on game logic, store impls, auth, API handlers.
- **Memstore** validates the upper-layer test surface; live Couchbase/Redis tests are gated by env vars (`COUCHBASE_TEST_CONN`, etc.).

### Error Handling

- **Sentinel errors** in `internal/store/errors.go` (`ErrNotFound`, `ErrClosed`).
- **Wrap errors** with `fmt.Errorf("%w", …)` to preserve sentinel.
- **Panic only for** programming errors. Use `errors.New()` / `fmt.Errorf()` for operational errors.

### Comments

- **Doc comments** on every exported Go func.
- **Explain the *why*** for non-obvious decisions (e.g. "first-write-only beta fields via CAS").
- Skip comments that just narrate what the code already says.

## CI/CD & Validation

### Pre-commit

- `make lint` (`golangci-lint`).
- `go test ./...` (server).
- `npm test` + `npm run build-nolog` (client).
- `make proto-gen && git diff --exit-code proto/` for protobuf regressions.

### CI Pipeline (GitHub Actions)

- Re-enabled in Phase 12 for Go 1.25.5 + Svelte/Phaser client.
- `buf lint` + `buf breaking`.
- `golangci-lint run ./...`.
- `go test ./...`.
- `npm run build-nolog` + `npm test`.

## Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Go packages | lowercase, short | `api`, `store`, `auth` |
| Go exported types | PascalCase | `Store`, `AuthClaims`, `Puzzle` |
| Go unexported | camelCase | `puzzleHandler`, `dateInWindow()` |
| Go constants | UPPER_SNAKE | `MAX_GUESSES`, `MaxGuesses` (mixed; PascalCase preferred for new) |
| Protobuf messages | PascalCase | `AuthRequest`, `Envelope` |
| Protobuf fields | snake_case | `message_type`, `request_id` |
| TS modules (utility) | camelCase / kebab-case | `scoring.ts`, `eventbus-helpers.ts` |
| TS classes & components | PascalCase | `WordleScene`, `WordleHud.svelte` |

## Dependencies

- **Go core:**
  - `chi` — HTTP router.
  - `nhooyr.io/websocket` — WS.
  - `gocb v2` — Couchbase (only in `store/couchbase/`).
  - `go-redis v9` — Redis (only in `store/redis/`).
  - `firebase.google.com/go/v4` — Admin SDK token verifier.
- **Frontend core:**
  - `svelte` 5, `@sveltejs/kit`, `vite`.
  - `phaser` 4 — game canvas.
  - `firebase` JS SDK — Auth.
  - `@capacitor/core` + `@capacitor-firebase/authentication` — mobile shell.
  - `protobufjs` — WS framing.
  - `vitest` — unit tests.

## Attribution & Licensing

- **Proprietary code:** All rights reserved (see LICENSE).
- **Third-party code:** copyright + license header on ported files.
- **NOTICE** updated when adding attributed dependencies.

## Process

- New phase work follows the `plans/` directory templates.
- Plans link to the active `plan.md` overview; phase files capture detailed steps.
- Status fields in plan frontmatter are kept current — `pending` → `in_progress` → `completed`.
