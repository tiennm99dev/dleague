# Code Standards

Patterns and constraints enforced in the dleague codebase. Updated for the MongoDB Atlas (Firebase Auth + Go backend + Svelte/Phaser client) stack.

## File & Module Structure

### Go Files

- **Max 200 LOC per file** — Split early. Prefer small, focused modules.
- **kebab-case directory names** when there's a useful disambiguator.
- **snake_case Go filenames** (`router_test.go`, `atlas_smoke.sh`).
- **Module layout:**
  - `cmd/{appname}/main.go` — single entry point per binary (`cmd/api`, `cmd/atlas-smoke`).
  - `internal/` — unexported packages.
  - `shared/pb/` — generated protobuf code (committed).
- **Store-interface boundary** is load-bearing for migration:
  - `go.mongodb.org/mongo-driver/v2` import only inside `internal/store/mongodb/`.
  - Memstore impl ships alongside as test backend + proof of seam.
  - Future swaps (Couchbase Capella, FerretDB, etc.) follow the same pattern. See `docs/migration-readiness.md`.

### Frontend (Svelte 5 + Phaser 4) Files

- **Pluggable -dle variants** under `client/web/src/games/<name>/` — each variant exports a `GameVariant` (Scene class + Hud component).
- **Phaser scenes** in PascalCase (`WordleScene.js`) — matches Phaser ecosystem.
- **Svelte components** in PascalCase (`WordleHud.svelte`, `Lobby.svelte`).
- **Pure utilities** in lowercase (`scoring.js`, `registry.js`, `eventbus-helpers.js`).
- **Shared events** routed through `game/EventBus.js` — JSDoc-typed wrappers in `games/runner/eventbus-helpers.js`.
- Plain JavaScript + JSDoc throughout (no TypeScript) — checked via `npm run check` against `client/web/jsconfig.json`.

### Directory Structure

```
dleague/
├── client/web/                   # Svelte 5 + Phaser 4 + Capacitor
│   └── src/
│       ├── auth/                 # Firebase JS SDK wrapper
│       ├── components/           # Svelte UI: Lobby, SignIn, BetaBanner
│       ├── game/                 # Phaser game shell + EventBus
│       ├── games/                # Pluggable -dle variants (Phase 8)
│       │   ├── types.js          # GameVariant JSDoc typedef
│       │   ├── registry.js       # lazy variant loader
│       │   ├── runner/           # GameRunner + EventBus helpers
│       │   └── wordle/           # First concrete variant
│       └── net/                  # WS client + protobuf
├── server/
│   ├── cmd/api/                  # Main HTTP/WS server
│   ├── cmd/atlas-smoke/          # MongoDB Atlas connectivity test
│   └── internal/
│       ├── api/                  # Async-PvP REST under /api/v1
│       ├── auth/                 # Firebase Admin token verifier
│       ├── config/
│       ├── http/                 # Top-level router + health
│       ├── store/                # Migration seam
│       │   ├── store.go          # Store interface + entity types
│       │   ├── mongodb/          # mongo-driver/v2 impl
│       │   └── memstore/         # in-memory impl for tests + proof of seam
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

```js
/**
 * @typedef {Object} GameVariant
 * @property {string} key                    // 'wordle', 'sumdle', etc.
 * @property {typeof Phaser.Scene} Scene
 * @property {typeof SvelteComponent} Hud
 * @property {{ title: string, difficulty: 'easy'|'medium'|'hard', tagline: string }} meta
 */
```

- Variants register in `client/web/src/games/registry.js` as lazy imports.
- `GameRunner.svelte` fetches puzzle + resume state, mounts the scene, mounts the HUD, posts the final attempt.
- Adding a new variant = copy `wordle/` folder + register in `registry.js`.

## Code Quality Rules

### Testing

- **Unit tests** in `*_test.go` (Go) / `*.test.js` (JSDoc-typed JavaScript via vitest).
- **Coverage target:** >70% on game logic, store impls, auth, API handlers.
- **Memstore** validates the upper-layer test surface; live MongoDB tests are gated by env var `MONGODB_TEST_URI`.

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

`.github/workflows/ci.yml.disabled` is the parked workflow from the WASM/MySQL
era. Re-enable for Go 1.25.5 + Svelte/Phaser + Atlas, running:

- `buf lint` + `buf breaking`.
- `golangci-lint run ./...`.
- `go test ./...` (server + shared modules).
- `make grep-isolation` (mongo-driver boundary check).
- `npm run build-nolog` + `npm test` (client).

## Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Go packages | lowercase, short | `api`, `store`, `auth` |
| Go exported types | PascalCase | `Store`, `AuthClaims`, `Puzzle` |
| Go unexported | camelCase | `puzzleHandler`, `dateInWindow()` |
| Go constants | UPPER_SNAKE | `MAX_GUESSES`, `MaxGuesses` (mixed; PascalCase preferred for new) |
| Protobuf messages | PascalCase | `AuthRequest`, `Envelope` |
| Protobuf fields | snake_case | `message_type`, `request_id` |
| JS modules (utility) | camelCase / kebab-case | `scoring.js`, `eventbus-helpers.js` |
| JS classes & components | PascalCase | `WordleScene`, `WordleHud.svelte` |

## Dependencies

- **Go core:**
  - `chi` — HTTP router.
  - `nhooyr.io/websocket` — WS.
  - `go.mongodb.org/mongo-driver/v2` — MongoDB client (only in `store/mongodb/`).
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
