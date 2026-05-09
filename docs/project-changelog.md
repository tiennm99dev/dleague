# Project Changelog

All notable changes to Dleague are documented here. Format: most recent first.

## [Unreleased] — pivot in progress

### Changed
- **Stack pivot** — Client: Ebitengine WASM → SvelteKit + Phaser. DB: Postgres / MySQL HeatWave → MongoDB Atlas M0. Auth: session cookie → Firebase Auth. Decision recorded in active plan.
- **Active plan** — [`260508-2300-svelte-phaser-firebase-mongo-pivot`](../plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md) supersedes three prior plans (now under [`plans/archive/`](../plans/archive/README.md)).

### Added
- 6 new docs scaffolded: `codebase-summary.md`, `system-architecture.md`, `deployment-guide.md`, `development-roadmap.md`, `project-overview-pdr.md`, this changelog.
- Three review reports (code, security, docs-audit) under `plans/reports/`.
- Three research reports (svelte-phaser, firebase-admin-go, mongodb-atlas-go) under `plans/reports/`.

### Changed (Phase 03)
- `nhooyr.io/websocket` (archived) replaced by `github.com/coder/websocket` v1.8.14 — API-compatible fork by the same author. No behavior change.

### Deprecated (pending Phase 06)
- `client/` Ebitengine module — to be deleted in Phase 06.

## 2026-05-08 — Phase 1 foundation
*Commit `9937c7d`*

### Added
- Go workspace (`go.work`) with `server/`, `client/`, `shared/` modules.
- Protobuf wire format: single `Envelope` over WebSocket. Generated Go committed at `shared/pb/dleague/v1/`.
- `/health` HTTP endpoint with DB-ping degradation status.
- `/ws` upgrade endpoint with ping-pong.
- Build-tag debug logging (`-tags debug` adds protojson on both sides).
- Ebitengine WASM client title scene.
- Initial CI: `buf lint`, `buf breaking`, `golangci-lint`, `go test`, WASM build.

## 2026-05-05 — Initial plan
*Commit `bb81b15`*

### Added
- Repository, README, LICENSE (proprietary), initial plan dir.
