# Codebase Summary

**Status:** skeleton — populated incrementally. Authoritative version generated from final state in Phase 10 polish.

## Top-level layout
See [`../README.md`](../README.md) §Repo layout.

## Modules

### `server/` — Go HTTP + WebSocket
TODO (Phase 02–05). Currently:
- `cmd/api/main.go` — boot wiring (config → http → ws hub → store → verifier)
- `internal/config/` — env-var loader
- `internal/http/` — `chi` router, `/health`, static file server, WS upgrade
- `internal/ws/` — connection hub, per-conn dispatch, ping/pong, debug-log split (`debug_log.go` + `debug_log_noop.go`)
- `internal/store/` — Mongo per-collection repos (Phase 04)
- `internal/auth/` — Firebase ID token verifier (Phase 05)
- `internal/game/` — server-authoritative Wordle logic (Phase 07)

### `shared/` — exported Go types
- `pb/dleague/v1/` — generated protobuf (committed)
- `game/` — `Game` interface + `Registry`

### `web/` — SvelteKit + Phaser client
TODO (Phase 06–07). Will contain:
- `src/lib/pb/` — generated TS protobuf
- `src/lib/ws.ts` — WebSocket client + reconnect + request_id correlation
- `src/lib/auth.ts` — Firebase JS SDK wrapper
- `src/lib/game/` — Phaser scenes + Svelte board components
- `src/routes/` — SvelteKit pages
- `static/` — sprites, fonts, audio

### `proto/` — protobuf schema
- `dleague/v1/envelope.proto` — single Envelope wrapping every WS message
- `buf.yaml`, `buf.gen.yaml` — codegen config (Go + TS targets after Phase 06)

### `plans/` — implementation plans
- `260508-2300-svelte-phaser-firebase-mongo-pivot/` — active
- `archive/` — superseded plans (do not edit)
- `reports/` — review + research reports

### `docs/` — this directory
See README of [`../README.md`](../README.md).

## Build entrypoints
- `make tools` — install buf + protoc-gen-go + protoc-gen-es (post-Phase-06)
- `make proto-gen` — regenerate Go + TS protobuf
- `make dev` — full stack: mongo + emulator + go + svelte (post-pivot, Phase 06)
- `make dev-debug` — same with `-tags debug` for protojson logging
- `make test` — `go test -race ./...` + `npm --prefix web test`
- `make lint` — `golangci-lint` + `buf lint`

## File size discipline
- Go files <200 LOC. Split modules early.
- Svelte / TS files <200 LOC where practical.
- Markdown / config / SQL exempt.

## Key patterns
- **Single-WS transport** — auth, game, match, admin all over one connection.
- **Binary protobuf wire** — `proto.Marshal`/`Unmarshal` both sides; `-tags debug` adds protojson logging.
- **Build-tag debug split** — `*_debug_log.go` paired with `*_debug_log_noop.go` for zero-cost prod builds.
- **Server-authoritative game** — client never knows the answer; only color feedback.
