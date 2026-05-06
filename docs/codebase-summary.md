# Codebase Summary

A directory-by-directory tour of the dleague repo, current as of the
self-hosted Couchbase + Redis pivot (`plans/260505-1604-…`).

## Top-level layout

```
dleague/
├── client/         # Legacy Ebitengine WASM (deprecated; pending Phase 12 decision)
├── client/web/     # Active web client: Svelte 5 + Phaser 4 + Capacitor
├── server/         # Go HTTP + WS hub
├── shared/         # Shared Go module (protobuf-generated only)
├── proto/          # .proto sources for the WS wire format
├── web/            # Static legacy shell (index.html, wasm_exec.js)
├── docker-compose.yml
├── Makefile
├── docs/           # This directory
└── plans/          # Implementation plans (active + archived)
```

## `client/web/src/`

Svelte 5 + Phaser 4 + Capacitor app. Entry is via SvelteKit.

```
src/
├── app.html, app.d.ts        # SvelteKit shell
├── routes/                   # SvelteKit pages (single page: +page.svelte)
├── auth/
│   ├── firebase.ts           # Firebase JS SDK config + helpers
│   └── auth.svelte.ts        # Rune-backed singleton auth state ($state)
├── components/
│   ├── BetaBanner.svelte     # "Beta — data may reset" banner
│   ├── SignIn.svelte         # Email + Google + Anonymous sign-in
│   └── Lobby.svelte          # Authenticated landing; mounts GameRunner
├── game/
│   ├── EventBus.ts           # Shared Phaser.Events.EventEmitter
│   ├── main.ts               # Legacy Phaser bootstrap (template scenes)
│   └── scenes/               # Boot, Preloader, MainMenu, Game, GameOver
├── games/                    # Phase 8: pluggable -dle variants
│   ├── types.ts              # GameVariant interface
│   ├── registry.ts           # Lazy variant loader
│   ├── runner/
│   │   ├── GameRunner.svelte # Generic shell: fetch puzzle → mount scene + HUD → POST attempt
│   │   └── eventbus-helpers.ts
│   └── wordle/               # First concrete variant
│       ├── WordleScene.ts    # Phaser scene: tile grid, keyboard, animations
│       ├── WordleHud.svelte  # Attempt counter, win/lose modal, share
│       ├── scoring.ts        # Pure scoring + per-guess evaluation
│       ├── scoring.test.ts   # vitest unit tests
│       └── index.ts          # Default-exports the GameVariant
├── net/
│   ├── proto.ts              # Envelope encode/decode helpers
│   └── ws.ts                 # WsClient with AUTH handshake + auto-reconnect
└── PhaserGame.svelte         # (Legacy) component for the template scenes
```

## `server/`

Single Go module. Two binaries.

```
server/
├── cmd/
│   ├── api/main.go               # Main HTTP/WS server
│   └── dleague-export/main.go    # Migration export CLI
└── internal/
    ├── api/                      # /api/v1/* (puzzles, attempts, leaderboards, scoring)
    ├── auth/                     # Firebase Admin verifier + middleware + WS gate
    ├── config/                   # Env var schema + parsing
    ├── http/                     # Top-level chi router + /health
    ├── store/                    # Migration seam
    │   ├── store.go              # Store interface + entity types
    │   ├── errors.go             # Sentinel errors
    │   ├── couchbase/            # gocb v2 impl (NOT used outside this dir)
    │   ├── redis/                # go-redis v9 impl (NOT used outside this dir)
    │   ├── composed/             # Wires couchbase + redis behind Store
    │   └── memstore/             # In-memory impl for tests + dev
    └── ws/                       # WebSocket hub + connection state
```

### Notable contracts

- `store.Store` (`internal/store/store.go`) — the migration seam. Every other package depends on this interface, never on a concrete impl.
- `auth.Verifier` + `auth.Upserter` (`internal/auth`) — minimal interfaces so tests can supply fakes.
- `api.Score` (`internal/api/scoring.go`) — pure scoring func used to re-derive score on attempt submit (cheat-resistant).

## `shared/`

```
shared/
└── pb/dleague/v1/    # Generated protobuf Go code (committed)
```

Protobuf is the wire format for the WebSocket transport only. Generated via
`make proto-gen` (buf + protoc-gen-go). REST API uses plain JSON.

## `proto/`

```
proto/dleague/v1/
└── *.proto           # Wire-format definitions
```

CI runs `buf lint` + `buf breaking` to keep schema changes safe.

## `plans/`

```
plans/
├── 260505-0947-dleague-pvp-game/         # Original master plan (parent)
├── 260505-1604-firebase-couchbase-redis-pivot/  # Active plan
├── archive/                              # Superseded plans
├── reports/                              # Research reports
└── journals/                             # Technical journal entries
```

## `docs/`

```
docs/
├── project-overview-pdr.md   # Elevator pitch + stack + scope
├── system-architecture.md    # Diagrams + key flows + security model
├── codebase-summary.md       # This file
├── code-standards.md         # Patterns + conventions
├── deployment-guide.md       # docker-compose + Coolify setup
├── migration-readiness.md    # Store-interface boundary + export usage
└── project-roadmap.md        # Phase status + post-beta milestones
```

## Removed / deferred

- **`client/` Ebitengine WASM** — Phase 12 decision pending. Currently retained as historical artifact. New work targets `client/web/` only.
- **`web/`** — minimal static shell from the WASM era; superseded by SvelteKit's adapter-static output but still present for the legacy WASM build.
- **MySQL HeatWave path** — fully removed. Plans archived at `plans/archive/260505-1319-mysql-heatwave-integration/`.
- **Firebase platform pivot path** — superseded by self-hosted plan. Archived at `plans/archive/260505-1407-firebase-platform-pivot/`.
