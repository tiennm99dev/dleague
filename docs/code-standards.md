# Code Standards

This document captures the patterns and constraints enforced in the dleague codebase. New contributors should follow these rules starting in Phase 2 of the active plan ([`plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`](../plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md)).

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
├── web/                 # SvelteKit + Phaser client
│   ├── src/
│   │   ├── lib/
│   │   │   ├── pb/     # Generated TS protobuf (committed)
│   │   │   ├── ws.ts   # WebSocket client + reconnect
│   │   │   ├── auth.ts # Firebase JS SDK wrapper
│   │   │   └── game/   # Phaser scenes + Svelte board components
│   │   └── routes/     # SvelteKit pages
│   ├── static/         # Sprites, fonts, audio
│   ├── package.json
│   └── svelte.config.js
├── server/              # Go HTTP + WebSocket
│   ├── cmd/api/        # main.go only
│   ├── internal/
│   │   ├── http/       # Router + handlers
│   │   ├── ws/         # WebSocket hub + conn + dispatch (uses coder/websocket)
│   │   ├── game/       # Server-authoritative game logic (Phase 07)
│   │   ├── store/      # Mongo per-collection repos (Phase 04)
│   │   ├── auth/       # Firebase ID token verifier (Phase 05)
│   │   └── config/
│   └── go.mod
└── shared/              # Exported types + interfaces
    ├── game/           # Game interface + Registry
    ├── pb/             # Generated Go protobuf (committed)
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
- **Production build** (default `go build`): Binary protobuf only. No serialization overhead.
- **Debug build** (`go build -tags debug`): Every WS message also serialized to `protojson` and logged:
  - **Server:** stdout (human-readable JSON)
  - **Client:** browser console (dev tools)
- **Implementation:** Build-tag conditionals in `**/debug_log.go` and `**/debug_log_noop.go` (zero-cost abstraction in production)

## Transport & Network

- **Single WebSocket endpoint** — all game, auth, and match messages travel over one `/ws` connection
- **HTTP only for static serving** — no REST endpoints for game state (use WS messages instead)
- **Connection upgrade:** Firebase ID token presented via `Sec-WebSocket-Protocol: dleague.v1, fb.<id_token>` and verified at upgrade time (Phase 05)
- **Token refresh:** Client refreshes ID token at ~50min via Firebase JS SDK and sends `AuthRefresh` envelope before expiry
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

### Logging (Phase 02+)
- **Never log raw Firebase UIDs at INFO+** — use `ws.RedactUID(uid string)` for HMAC-hashed output (format: `u_<8-hex>`).
- **Never log full bearer tokens** — use `ws.TruncateToken(token string)` for first 8 chars only.
- **No credential material in error envelopes** — do not include tokens, UIDs, or passwords in ERROR payloads sent to client.

### Authentication & Authorization (Phase 03+)
- **Sign-in errors must not distinguish wrong-password from user-not-found** — anti-enumeration pattern: map both to a generic "Incorrect email or password" message to prevent account existence probing.
- **Friendly error message mapping (web):** Firebase codes (`auth/wrong-password`, `auth/invalid-email`, `auth/too-many-requests`) mapped to user-facing text; never expose raw error codes to users (Phase 03).

## MongoDB Conventions (Phase 04+)

- **Driver:** `go.mongodb.org/mongo-driver/v2`. One `*mongo.Client` per process.
- **Repository pattern:** one struct per collection (`UserRepo`, `MatchRepo`, …) constructed via `NewUserRepo(db *mongo.Database)`. No god-object `Store`.
- **`_id` strategy:**
  - `users._id = <firebase_uid>` (string) — auth-driven primary key
  - `matches._id`, `attempts._id`, `daily_puzzles._id` — see schema in active plan
- **`bson:` tags** required on every persisted struct field. Match Go field names where reasonable.
- **`schema_version` field** on every document. Lazy-migrate on read; bump on shape change.
- **Indexes** declared in code at startup (`EnsureIndexes` per repo). Never created ad-hoc.
- **Transactions:** `session.WithTransaction()` callback API for atomic sync-PvP match-end. M0 supports replica-set transactions out of the box.
- **State-machine filters:** All conditional updates that mutate state (e.g., `match → complete`, `attempt insert`) must filter on source state. Mark each with a comment: `// MUST: filter on source state to prevent double-resolve under transaction retries`. Non-state-machine updates (e.g., increment counters) may omit the filter; mark as `// not a state machine`.
- **Timeouts:** `ConnectTimeout: 10s`, `ServerSelectionTimeout: 5s`. Per-op contexts inherit caller's deadline.

## CI/CD & Validation

### Pre-commit
- Run `make lint` to catch `golangci-lint` issues (gofmt, revive, unused)
- Run `make test` to ensure tests pass
- Run `make proto-gen && git diff --exit-code proto/` to verify no protobuf regressions

### CI Pipeline (GitHub Actions)
- All third-party action references pinned to immutable SHAs (no mutable `@vN` tags)
- `buf lint` — protobuf schema linting
- `buf breaking` — backward-incompatible schema changes rejected
- `golangci-lint run ./...` — Go linting
- `go test -race ./...` — unit tests with race detector
- `npm --prefix web ci && npm --prefix web run build` — client build
- Artifact upload (`web/dist/`)

### Bundle Size
- **Target:** JS bundle <400 KB gzipped (Svelte runtime + Phaser + protobuf-es + game code)
- **Monitor:** Each new `npm` dep, each Phaser plugin add, each protobuf schema add
- **If over-budget:** Defer feature to v2, use Phaser custom build, or split routes

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

## TypeScript / Svelte (web/)

### File Naming
- **kebab-case** for all `.ts` and `.svelte` files — e.g. `auth-store.ts`, `sign-in.svelte`, `phaser-game.svelte`
- SvelteKit route files follow the framework convention: `+page.svelte`, `+layout.ts`, etc.
- Phaser scene classes use PascalCase *class name* inside a kebab-case file: `title-scene.ts` exports `TitleScene`

### TypeScript Rules
- **No `any`** — use `unknown` + type guards or proper generated protobuf types
- **ES2022 target** — `crypto.randomUUID()`, top-level await, and nullish coalescing are all available
- **Strict mode on** — `noImplicitAny: true`; tsconfig extends `.svelte-kit/tsconfig.json`
- **`@bufbuild/protobuf` v2 API** — use module functions `create(Schema, init)`, `toBinary(Schema, msg)`, `fromBinary(Schema, bytes)`. Do NOT use `new MessageClass()` (v1 class API)
- **Generated protobuf** in `web/src/lib/pb/` is committed to git; run `make proto-gen` after any `.proto` change

### Component Rules
- Each Svelte component **<200 LOC**
- No inline `<script>` logic >30 lines — extract to a `.ts` module
- Use Svelte 5 runes (`$state`, `$derived`, `$props`) for reactive state; avoid legacy `$:` where practical

### EventBus Conventions
- Event names: `kebab-case` namespaced by scene — `'title:start'`, `'game:move'`, `'game:over'`
- Single shared `eventBus` instance exported from `src/lib/phaser/event-bus.ts`
- Phaser scenes emit; Svelte components listen (one-way: scene → Svelte)
- Always `off()` listeners in Svelte `onMount` cleanup functions to prevent leaks

### WS Client Conventions
- All WS sends go through `sendRequest()` or fire-and-forget via the socket directly with a constructed `Envelope`
- Payload bytes are `Uint8Array` at the `ws.ts` boundary; callers are responsible for encoding/decoding inner messages
- Token refresh is handled internally in `ws.ts` — callers do not need to manage it

## Dependencies & Vendoring

- **No Go vendoring** — use `go.mod` + `go.sum`. Lock file committed to git.
- **Review before adding heavy deps** — impacts client JS bundle (TS deps) and server binary size
- **Current core deps:**
  - **Server:** `github.com/coder/websocket` (WS, maintained fork of archived nhooyr), `chi` (HTTP router), `protobuf-go` (generated code only), `go.mongodb.org/mongo-driver/v2`, `firebase.google.com/go/v4`
  - **Client:** `svelte`, `@sveltejs/kit`, `@sveltejs/adapter-static`, `phaser`, `@bufbuild/protobuf`, `firebase` (JS SDK)

## Attribution & Licensing

- **Proprietary code:** All rights reserved (see LICENSE)
- **Third-party code:** Copyright + license header on ported files
  - MIT: ratel-online patterns (e.g., `shared/game/registry.go`)
  - MIT: phaserjs/template-svelte patterns (e.g., `web/src/lib/game/EventBus.ts`)
- **Update NOTICE** when adding new attributed dependencies

## Process & Next Steps

These standards apply to **Phase 2+** work. Phase 1 is scaffolding; Phase 2 introduces game core logic where these patterns become critical.

**Before opening Phase 2 PRs:**
1. Ensure all files <200 LOC
2. Run `make lint && make test`
3. No uncommitted generated code (`buf generate` should produce no diff)
4. New exported funcs have doc comments
