# Codebase Summary

A directory-by-directory tour of the dleague repo, current as of the
MongoDB Atlas consolidation
(`plans/archive/260507-1648-mongodb-atlas-only-migration/`).

## Top-level layout

```
dleague/
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
│   ├── firebase.js           # Firebase JS SDK config + helpers
│   └── auth.svelte.js        # Rune-backed singleton auth state ($state)
├── components/
│   ├── BetaBanner.svelte     # "Beta — data may reset" banner
│   ├── SignIn.svelte         # Email + Google + Anonymous sign-in
│   └── Lobby.svelte          # Authenticated landing; mounts GameRunner
├── game/
│   ├── EventBus.js           # Shared Phaser.Events.EventEmitter
│   ├── main.js               # Legacy Phaser bootstrap (template scenes)
│   └── scenes/               # Boot, Preloader, MainMenu, Game, GameOver
├── games/                    # Pluggable -dle variants
│   ├── types.js              # GameVariant JSDoc typedef
│   ├── registry.js           # Lazy variant loader
│   ├── runner/
│   │   ├── GameRunner.svelte # Generic shell: fetch puzzle → mount scene + HUD → POST attempt
│   │   └── eventbus-helpers.js
│   └── wordle/               # First concrete variant
│       ├── WordleScene.js    # Phaser scene: tile grid, keyboard, animations
│       ├── WordleHud.svelte  # Attempt counter, win/lose modal, share
│       ├── scoring.js        # Pure scoring + per-guess evaluation
│       ├── scoring.test.js   # vitest unit tests
│       └── index.js          # Default-exports the GameVariant
├── net/
│   ├── proto.js              # Envelope encode/decode helpers
│   └── ws.js                 # WsClient with AUTH handshake + auto-reconnect
└── PhaserGame.svelte         # (Legacy) component for the template scenes
```

## `server/`

Single Go module. Two binaries.

```
server/
├── cmd/
│   ├── api/main.go            # Main HTTP/WS server
│   └── atlas-smoke/main.go    # MongoDB Atlas connectivity smoke test
└── internal/
    ├── api/                   # /api/v1/* (puzzles, attempts, leaderboards, scoring)
    ├── auth/                  # Firebase Admin verifier + middleware + WS gate
    ├── config/                # Env var schema + parsing
    ├── http/                  # Top-level chi router + /health
    ├── store/                 # Migration seam
    │   ├── store.go           # Store interface + entity types (json + bson tags)
    │   ├── errors.go          # Sentinel errors
    │   ├── mongodb/           # mongo-driver/v2 impl (NOT used outside this dir)
    │   └── memstore/          # In-memory impl for tests + dev
    └── ws/                    # WebSocket hub + connection state
```

### Notable contracts

- `store.Store` (`internal/store/store.go`) — the migration seam. Every other package depends on this interface, never on a concrete impl. `make grep-isolation` enforces that `go.mongodb.org/mongo-driver/v2` stays inside `internal/store/mongodb/`.
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
├── archive/      # All shipped + superseded plans (latest: Atlas consolidation)
├── reports/      # Research + brainstorm + code-review reports
└── journals/     # Technical journal entries
```

No "active" plan dir at present — every implementation plan ships or gets
superseded into `archive/`. The most recent shipped plan is
`archive/260507-1648-mongodb-atlas-only-migration/` (Atlas consolidation,
2026-05-07).

## `docs/`

```
docs/
├── project-overview-pdr.md   # Elevator pitch + stack + scope
├── system-architecture.md    # Diagrams + key flows + security model
├── codebase-summary.md       # This file
├── code-standards.md         # Patterns + conventions
├── deployment-guide.md       # docker-compose + Coolify setup
├── atlas-setup.md            # MongoDB Atlas provisioning runbook
├── migration-readiness.md    # Store-interface boundary + outbound recipe
└── project-roadmap.md        # Phase status + post-beta milestones
```

## Removed / deferred

- **`internal/store/couchbase/`, `internal/store/redis/`, `internal/store/composed/`** — removed in the Atlas consolidation (2026-05-07). Behavior absorbed into `internal/store/mongodb/`.
- **`cmd/dleague-export`** — retired in favor of native `mongodump`. The `Export` method on `store.Store` is preserved; the CLI wrapper is no longer needed.
- **`client/` Ebitengine WASM (Go module)** — removed in the predecessor plan's Phase 12. Only `client/web/` remains. Recoverable via git history.
- **`web/`** — minimal static shell from the WASM era; superseded by SvelteKit's `adapter-static` output. Retained pending a future cleanup pass; no longer load-bearing.
- **MySQL HeatWave path** — fully removed. Plans archived at `plans/archive/260505-1319-mysql-heatwave-integration/`.
- **Firebase platform pivot path** — superseded by self-hosted plan. Archived at `plans/archive/260505-1407-firebase-platform-pivot/`.
