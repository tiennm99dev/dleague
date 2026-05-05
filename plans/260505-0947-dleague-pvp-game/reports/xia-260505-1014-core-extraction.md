# Xia: Core Extraction Report

**Date:** 2026-05-05
**Mode:** --port (default)
**Target:** /config/workspace/tiennm99/dleague
**Scope:** Phase 1 (foundation) + Phase 2 (game core pluggable -dle), with architecture pivot to **protobuf schemas + single WebSocket transport**.

---

## Source Manifest

| Source | Local path | License | Verdict |
|--------|-----------|---------|---------|
| ratel-online/server | /config/workspace/ratel-online/server | MIT | ACCEPT (port with attribution) |
| ratel-online/core | /config/workspace/ratel-online/core | MIT | ACCEPT (port with attribution) |
| ratel-online/client | /config/workspace/ratel-online/client | MIT | ACCEPT (port with attribution) |
| hajimehoshi/ebiten/examples | /config/workspace/hajimehoshi/ebiten/examples | Apache-2.0 | ACCEPT (port with attribution) |
| ~~tslocum/bouncyblob~~ | (removed) | AGPLv3 | REJECTED, deleted from local |

## Source Anatomy

### ratel-online/core/protocol/protocol.go
- `Packet{Body []byte}` — opaque envelope with helpers `Int/String/Unmarshal/Marshal`
- `ReadWriteCloser` interface — Read/Write/Close/IP, transport-agnostic
- 4-byte BigEndian length prefix framing for raw TCP streams
- `consts.MaxPacketSize` ceiling

### ratel-online/server/network/wss.go
- `gorilla/websocket` upgrade with permissive CheckOrigin (dev-only OK)
- Global `http.HandleFunc("/ws", serveWs)` — single endpoint
- Wraps WS conn in a `ReadWriteCloser` adapter, dispatches to `handle()`

### ratel-online/server/state/state.go
- Global `states map[StateID]State` registry
- `register(id, state)` factory pattern, init-time wiring
- `State` interface: `Next(player) (StateID, error)`, `Exit(player) StateID`
- `Run(player)` blocks per-player goroutine in infinite loop, walks state graph

### ebiten/examples/2048/2048/tile.go
- `Tile{current, next TileData, movingCount, startPoppingCount, poppingCount int}`
- Counter-based animation: `Update()` decrements counters, `Draw()` interpolates via `mean()` linear blend
- Lifecycle: NewTile spawns with startPoppingCount → MoveTiles sets movingCount → Update transitions current←next when count reaches 0

### ebiten/examples/2048/2048/game.go
- Canonical Ebitengine `Game{Layout/Update/Draw}` interface
- Composition: `Game → Board → Tile → Input`
- Single boardImage cached on Game, blitted to screen each Draw

## Decision Matrix

| Decision | Source approach | Local approach | Verdict |
|----------|----------------|----------------|---------|
| Wire format | ratel: opaque `[]byte` Body + JSON helpers | Plan: typed Go structs | **PROTOBUF schemas via buf**. Generate Go structs. Wire = **binary** (`proto.Marshal`). Debug builds (`-tags debug`) additionally log every message as protojson to console (client) / stdout (server) for human-readable debugging. Production excludes protojson |
| Transport | ratel: TCP + WS dual; HTTP for upgrade only | Plan: HTTP REST + WS for sync | **SINGLE WEBSOCKET** for all messages. HTTP only serves static + WS upgrade |
| Length-prefix framing | 4-byte BigEndian length header | WS frames already framed | **DROP** — WebSocket message frames are self-delimiting |
| Per-player blocking goroutine | `Run(player)` infinite loop with `Next/Exit` | Need 1k+ concurrent conns | **DROP** for Hub. Use event-driven hub with per-conn write goroutine |
| Global `register()` factory | `register(id, State)` init-time | Plan: pluggable -dle game registry | **PORT pattern** as `shared/game/registry.go` for game types (Wordle, Music, Geo) |
| Tile animation | 2048 counter+interpolation | Wordle flip + shake animations needed | **PORT structure**, replace move/pop semantics with flip-rotation. Keep `Update/Draw` shape |
| Game interface | Ebitengine `Game{Layout/Update/Draw}` | Same Ebitengine API | **PORT directly**, preserve Apache-2.0 header in `client/internal/game/loop.go` |
| HTTP routing | `http.HandleFunc` global | Plan: chi for middleware chain | **chi wins** — even though we have minimal HTTP, chi's middleware (rate-limit, auth, CORS) matters |
| Canvas vs HTML | 2048: pure canvas | Plan: HTML overlay + canvas | **HYBRID** — HTML grid for input/a11y, canvas overlay for tile-flip animation |
| Authentication | ratel: in-band player struct | Plan: cookie-based session | Cookie set during HTML page load → WS upgrade reads cookie → bind to session in hub |
| Request/response correlation | ratel: state-machine implicit | WS asynchronous | Add `request_id` field in proto Envelope; server echoes in response |

## Dependency Matrix

Source files → local target file mapping with classification:

| Source | Target | Classification | Notes |
|--------|--------|---------------|-------|
| ratel-online/core/protocol/protocol.go (Packet, ReadWriteCloser concept) | server/internal/ws/conn.go | NEW (concept-port, not file-port) | Adapt `ReadWriteCloser` idea for WS; drop Packet envelope (replaced by protobuf) |
| ratel-online/server/network/wss.go (upgrade pattern) | server/internal/http/ws_handler.go | NEW (concept-port) | Use chi mount, gorilla/websocket or nhooyr.io/websocket |
| ratel-online/server/state/state.go (`register` pattern) | shared/game/registry.go | PORT-PATTERN | Adapt for game-type registry, not state-machine |
| ratel-online/core (json utilities) | (none) | DROP | Replaced by protobuf marshaling |
| ebiten/examples/2048/2048/tile.go (Tile struct + Update/Draw) | client/internal/scene/wordle/tile.go | PORT-FILE-WITH-MODIFICATION | Keep Apache-2.0 header. Replace move/pop with flip animation |
| ebiten/examples/2048/2048/game.go (Game struct shape) | client/internal/game/loop.go | PORT-PATTERN | Generic loop; specific games plug in via `Game` interface |
| ebiten/examples/2048/2048/input.go | client/internal/scene/wordle/input.go | PORT-PATTERN | Keyboard mapping pattern |
| ebiten/examples/2048/2048/board.go | client/internal/scene/wordle/board.go | PORT-PATTERN | Grid layout + tile placement |
| ebiten/examples/2048/2048/colors.go | client/internal/scene/wordle/colors.go | NEW | Wordle palette (green/yellow/gray), inspired-by but not copied |

## Architectural Pivot: Single-WebSocket Transport

Per user direction: all client↔server messages flow over WebSocket. No HTTP REST API for game logic. HTTP layer reduced to:
- Serving static assets (WASM, HTML, CSS)
- `/ws` upgrade endpoint
- `/auth/oauth/callback` (only if OAuth added later)

### Implications for plan phases

| Phase | Original | Revised |
|-------|----------|---------|
| Phase 3 (Backend + auth) | HTTP REST + sessions | WS message handlers + cookie-bound session at WS upgrade |
| Phase 4 (Async PvP) | REST endpoints for matches/leaderboard | WS message types `match.create`, `match.join`, `leaderboard.get` |
| Phase 5 (Sync PvP) | WebSocket | Same WebSocket, additional message types `queue.join`, `match.guess`, etc |

**Phase 3-5 message handlers can share dispatch infrastructure.** Consider merging Phase 3 auth-handlers + Phase 4 match-handlers into a single `server/internal/ws/handlers/` package with one file per concern.

### Protobuf schema layout

```
proto/
├── dleague/
│   └── v1/
│       ├── envelope.proto      # Envelope{type, request_id, payload Any}
│       ├── auth.proto           # Register, Login, Logout, Me
│       ├── game.proto           # Wordle attempt, hint colors, tile state
│       ├── match.proto          # Create/Join/Get match, Leaderboard
│       └── sync.proto           # QueueJoin, Matched, OpponentGuess, Resolved
├── buf.yaml
├── buf.gen.yaml
└── buf.lock
```

Generated code → `shared/pb/dleague/v1/*.pb.go` (committed, not gitignored, so consumers don't need protoc).

### Tooling additions to Phase 1

- `buf` CLI in dev dependencies (Makefile installs via `go install github.com/bufbuild/buf/cmd/buf@latest`)
- `protoc-gen-go` for Go codegen
- Makefile targets: `proto-gen`, `proto-lint`, `proto-breaking`
- CI: lint + breaking-change check on every PR
- Wire format library: `google.golang.org/protobuf/encoding/protojson` (smaller WASM cost than full grpc)

### Bundle size budget revision

| Item | Production estimate | Debug estimate |
|------|---------------------|----------------|
| Ebitengine runtime | 4-6MB | 4-6MB |
| protobuf-go runtime (binary only) | ~400KB | ~400KB |
| protojson (debug only via build tag) | 0 | ~300KB |
| Application code | <1MB | <1MB |
| Wordlist (embedded) | ~50KB | ~50KB |
| **Total target gzipped** | **<10MB** (unchanged) | ~10.5MB (debug, only used in dev) |

Drop full `google.golang.org/grpc` (~3MB) — never needed for our pattern.

Build-tag pattern keeps debug logging code out of production WASM:
- `*_debug.go` files have `//go:build debug` — calls into protojson
- `*_noop.go` files have `//go:build !debug` — empty stubs
- Default `go build` excludes protojson entirely

## Challenge Outcomes

| Question | Answer |
|----------|--------|
| Port ratel TCP framing? | **NO** — WS frames are self-delimiting |
| Port ratel state-machine? | **Pattern only** — adapt for game-type registry, not per-player loop |
| Port 2048 tile animation verbatim? | **Structure yes, semantics no** — replace move/pop with flip |
| Canvas-only or hybrid? | **Hybrid** — HTML for a11y, canvas overlay for animation |
| Use Packet `[]byte` envelope? | **NO** — replaced by protobuf typed messages |
| Use `register()` factory pattern? | **YES** — for game-type registry in `shared/game/` |
| HTTP REST + WS, or WS-only? | **WS-only** for game logic; HTTP only for static + upgrade |
| gRPC, gRPC-Web, Connect, or proto-only? | **Proto-only** schemas, WebSocket transport, **binary** wire + debug-tag protojson logging |
| Commit generated `.pb.go` files? | **YES** — committed. CI verifies regen produces no diff |

## Risk Assessment

| Risk | Level | Mitigation |
|------|-------|-----------|
| Bundle bloat from protobuf-go | LOW | Use `protojson` not full gRPC. Measured ~700KB. Within <10MB budget |
| Single-WS dependency = single point of failure | MEDIUM | Reconnect logic + message replay on resume. Fallback: long-polling not needed since WS is universally supported |
| Proto schema breaking changes | MEDIUM | `buf breaking` check in CI from day 1. Versioned package `dleague.v1` allows v2 alongside |
| Attribution drift | LOW | NOTICE file at repo root. Apache headers preserved on direct file ports. Periodic audit |
| Loop-based animation timing on slow devices | LOW | Counters tied to Update() ticks; Ebitengine handles tick rate. 60Hz default |
| Concept similarity to AGPLv3 bouncyblob | LOW | Repo deleted from local. Patterns we use are from MIT/Apache sources, not bouncyblob |

**Overall risk score: LOW**

## Implementation Handoff

Plan files updated in companion edits:
- `plan.md` — added single-WS + protobuf decisions
- `phase-01-foundation-monorepo.md` — added buf tooling, proto/ scaffold, NOTICE file
- `phase-02-game-core-pluggable.md` — added 2048 Tile port reference

Phase 3-5 phases need re-scoping later (merge HTTP handler work into WS handlers). Defer until Phase 1+2 ship.

**Ready to implement Phase 1.** Run:

```
/ck:cook /config/workspace/tiennm99/dleague/plans/260505-0947-dleague-pvp-game/phase-01-foundation-monorepo.md
```

## Unresolved Questions

- ~~Wire format binary upgrade trigger~~ → **RESOLVED:** binary from v1, debug builds add protojson via build tags
- ~~Generated `*.pb.go` committed or gitignored?~~ → **RESOLVED:** committed
- Cookie-based auth at WS upgrade vs token-in-first-message: which is simpler for WASM client? (decide in Phase 3 redesign)
- Should `proto/` be a sibling top-level dir or nested under `shared/`? (cosmetic — prefer top-level for `buf` convention)
- Phase 3-5 merge: combine into one phase "all-WS server" or keep separate? (post-Phase-2 decision)
