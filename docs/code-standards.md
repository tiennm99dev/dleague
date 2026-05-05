# Code Standards

This document captures the patterns and constraints enforced in the dleague codebase. New contributors should follow these rules starting in Phase 2.

## File & Module Structure

### Go Files
- **Max 200 LOC per file** — Split early if approaching limit. Prefer small, focused modules.
- **kebab-case directory names** (e.g., `internal/game-state/`, `internal/ws-hub/`)
- **snake_case Go filenames** (e.g., `ws_client.go`, `debug_log.go`)
- **Modules must compile independently** — each module has its own `go.mod` (workspace root uses `go.work`)
- **Module layout:**
  - `cmd/{appname}/main.go` — single entry point per app
  - `internal/` — unexported packages (not importable from other modules)
  - `shared/pb/` — generated protobuf code only (committed to git, not gitignored)

### Directory Structure
```
dleague/
├── client/              # Ebitengine WASM entry
│   ├── cmd/web/        # main.go only
│   ├── internal/       # Unexported
│   │   ├── game/       # Client-side game state
│   │   ├── net/        # WS client + debug logging
│   │   └── ui/         # HTML/CSS overlay
│   └── go.mod
├── server/              # Go HTTP + WebSocket
│   ├── cmd/api/        # main.go only
│   ├── internal/
│   │   ├── http/       # Router + handlers
│   │   ├── ws/         # WebSocket hub + conn + dispatch
│   │   ├── game/       # Game state + match logic (Phase 2)
│   │   ├── store/      # Postgres repos (Phase 3)
│   │   └── config/
│   └── go.mod
└── shared/              # Exported types + interfaces
    ├── game/           # Game interface + Registry
    ├── pb/             # Generated protobuf (committed)
    └── go.mod
```

## Wire Format & Serialization

### Protobuf
- **Binary format always** — messages sent as `proto.Marshal(msg)` over WebSocket
- **Schema sources** live in `proto/dleague/v1/` (`.proto` files)
- **Generated code** committed to git (`shared/pb/dleague/v1/*.pb.go`)
  - CI verifies `make proto-gen` produces no diff
  - Consumers build without needing protoc/buf installed
- **Validation:** `buf lint` and `buf breaking` run in CI

### Debug Logging
- **Production build** (default `go build`): Binary protobuf only. No serialization overhead, no protojson symbols in WASM.
- **Debug build** (`go build -tags debug`): Every WS message also serialized to `protojson` and logged:
  - **Server:** stdout (human-readable JSON)
  - **Client:** browser console (dev tools)
- **Implementation:** Build-tag conditionals in `**/debug_log.go` and `**/debug_log_noop.go` (zero-cost abstraction in production)

## Transport & Network

- **Single WebSocket endpoint** — all game, auth, and match messages travel over one `/ws` connection
- **HTTP only for static serving** — no REST endpoints for game state (use WS messages instead)
- **Connection upgrade:** Session cookie bound at WS upgrade time (Phase 3 auth)
- **Message frame type:** `websocket.MessageBinary` for all WS sends
- **Recovery:** Connection drop = client reconnects; server resets player state. State durability added in Phase 4 (async PvP).

## Game Architecture

### Game Interface (Phase 2)
- **Define:** `shared/game/Game` interface with pluggable `-dle` types (Wordle, music, geography, etc.)
- **Registry:** Factory pattern in `shared/game/registry.go` — register games by ID, lookup at match start
- **Constraint:** Single active game per match. Variants ship in separate releases, not runtime selection.

## Code Quality Rules

### Testing
- **Unit tests** in `*_test.go` files alongside implementation
- **Coverage target:** >70% on game logic + WS handlers
- **Test-first for bug fixes** — failing test before fix

### Error Handling
- **Named return errors** only for recoverable conditions (e.g., "this player already in queue")
- **Panic only for** programming errors (e.g., unregistered game type). Use `errors.New()` for operational errors.
- **Wrap errors** with `fmt.Errorf()` to preserve context

### Comments
- **Capitalize comment sentences** — "This function handles X, not "this function handles X"
- **Exported functions must have doc comments** — `// FunctionName does Y.`
- **Complex logic:** Explain *why*, not *what* (code shows what)

## CI/CD & Validation

### Pre-commit
- Run `make lint` to catch `golangci-lint` issues (gofmt, revive, unused)
- Run `make test` to ensure tests pass
- Run `make proto-gen && git diff --exit-code proto/` to verify no protobuf regressions

### CI Pipeline (GitHub Actions)
- `buf lint` — protobuf schema linting
- `buf breaking` — backward-incompatible schema changes rejected
- `golangci-lint run ./...` — Go linting
- `go test ./...` — unit tests
- Build WASM (`GOOS=js GOARCH=wasm`)
- Artifact upload (dist/wasm/main.wasm)

### Bundle Size
- **Target:** WASM <10MB gzipped (Phase 1 baseline: ~5-8MB protobuf-go + Ebitengine hello-world)
- **Monitor:** Each `buf` schema add, each `go` dependency
- **If over-budget:** Defer feature to v2 or switch implementation approach

## Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Packages | lowercase, short | `game`, `net`, `store` |
| Exported types | PascalCase | `Connection`, `GameState`, `Envelope` |
| Unexported vars/funcs | camelCase | `connHub`, `readMessage()` |
| Constants | UPPER_SNAKE_CASE | `MAX_MESSAGE_SIZE`, `PING_INTERVAL` |
| Interfaces | PascalCase (often `-er` suffix) | `Game`, `Reader`, `Handler` |
| Protobuf messages | PascalCase | `Envelope`, `Ping`, `GameMove` |
| Protobuf fields | snake_case | `message_type`, `request_id`, `payload` |

## Dependencies & Vendoring

- **No Go vendoring** — use `go.mod` + `go.sum`. Lock file committed to git.
- **Review before adding heavy deps** — impacts WASM bundle size
- **Current core deps:**
  - `nhooyr.io/websocket` (WS, lighter than gorilla)
  - `chi` (HTTP router, minimal)
  - `protobuf-go` (generated code only, ~400KB baseline)
  - `ebitengine` (Go → WASM game engine)

## Attribution & Licensing

- **Proprietary code:** All rights reserved (see LICENSE)
- **Third-party code:** Copyright + license header on ported files
  - Apache-2.0: Ebitengine examples (e.g., `client/cmd/web/main.go`)
  - MIT: ratel-online patterns (e.g., `shared/game/registry.go`)
- **Update NOTICE** when adding new attributed dependencies

## Process & Next Steps

These standards apply to **Phase 2+** work. Phase 1 is scaffolding; Phase 2 introduces game core logic where these patterns become critical.

**Before opening Phase 2 PRs:**
1. Ensure all files <200 LOC
2. Run `make lint && make test`
3. No uncommitted generated code (`buf generate` should produce no diff)
4. New exported funcs have doc comments
