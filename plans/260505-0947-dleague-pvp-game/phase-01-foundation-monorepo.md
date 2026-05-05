---
phase: 1
title: "Foundation & monorepo"
status: pending
priority: P1
effort: 1w
dependencies: []
---

# Phase 1: Foundation & monorepo

## Overview

Set up the Go workspace monorepo, Ebitengine WASM scaffold, Postgres dev environment, and CI pipeline. Outcome: `git clone && make dev` brings up a hello-world Ebitengine canvas in browser + a Go API responding to `/health`.

## Requirements

- **Functional:**
  - `go.work` workspace with `client/`, `server/`, `shared/` modules
  - `client/` builds to `dist/wasm/main.wasm` via `GOOS=js GOARCH=wasm`
  - `server/` runs on `:8080` with `/health` endpoint
  - `shared/` holds DTOs, game-logic interfaces, constants reused by both
  - Postgres runs locally via `docker-compose.yml`
  - `make dev` starts server + watches WASM rebuild
  - GitHub Actions CI: lint (`golangci-lint`), test (`go test ./...`), build WASM
- **Non-functional:**
  - WASM bundle <8MB at this phase (just hello world)
  - Each Go file <200 LOC
  - kebab-case directories, snake_case Go files

## Architecture

```
dleague/
├── client/                  # Ebitengine WASM
│   ├── cmd/web/main.go      # entry point
│   ├── internal/
│   │   ├── scene/           # title, game, results scenes
│   │   ├── ui/              # HTML overlay bridge (syscall/js)
│   │   └── game/            # client-side game state
│   └── go.mod
├── server/                  # Go HTTP + WebSocket API
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── http/            # routes, handlers
│   │   ├── ws/              # WebSocket hub (placeholder)
│   │   ├── store/           # Postgres repo
│   │   └── config/
│   └── go.mod
├── shared/                  # cross-module types + interfaces
│   ├── dto/
│   ├── game/                # Game interface (pluggable -dle)
│   └── go.mod
├── web/                     # static HTML shell + CSS overlay
│   ├── index.html
│   ├── wasm_exec.js         # from Go SDK
│   └── styles.css
├── docker-compose.yml       # postgres only at this phase
├── go.work
├── Makefile
└── .github/workflows/ci.yml
```

## Related Code Files

**Create:**
- `go.work`
- `client/go.mod`, `client/cmd/web/main.go`, `client/internal/scene/title.go`
- `server/go.mod`, `server/cmd/api/main.go`, `server/internal/http/router.go`, `server/internal/http/health.go`
- `shared/go.mod`, `shared/game/game.go` (interface stub)
- `web/index.html`, `web/wasm_exec.js`, `web/styles.css`
- `docker-compose.yml` (postgres:16)
- `Makefile` (targets: dev, build-wasm, build-server, test, lint, db-up)
- `.github/workflows/ci.yml`
- `.gitignore`, `README.md`, `LICENSE` (Apache-2.0 to match user's MathMax pattern)
- `.golangci.yml`

**Modify:** none (greenfield)

## Implementation Steps

1. `go work init` → `go work use ./client ./server ./shared`
2. Scaffold `shared/game/game.go` with `Game` interface stub: `Init() / HandleInput() / Render() / IsSolved() / Result()`
3. Scaffold `client/cmd/web/main.go` — Ebitengine boilerplate, blank canvas with "Dleague" title text
4. Add `web/index.html` loading `wasm_exec.js` + `main.wasm`, CSS overlay placeholder div
5. Scaffold `server/cmd/api/main.go` — `chi` router, `/health` returns `{"status":"ok","version":"dev"}`
6. Add `docker-compose.yml` with `postgres:16` on `:5432`, persistent volume
7. Write `Makefile` targets (dev runs server + watches WASM via `air` or simple loop)
8. Configure `golangci-lint` (default + `gofmt`, `revive`, `unused`)
9. CI workflow: matrix Go 1.23, run lint + test + build WASM, upload artifact
10. README with `make dev` quickstart, repo layout, license badge

## Todo List

- [ ] Init Go workspace + 3 modules
- [ ] Define `shared/game.Game` interface
- [ ] Ebitengine hello-world WASM
- [ ] HTML shell + CSS overlay scaffold
- [ ] Go API with /health
- [ ] docker-compose Postgres
- [ ] Makefile dev/build/test targets
- [ ] golangci-lint config
- [ ] GitHub Actions CI
- [ ] README + LICENSE + .gitignore

## Success Criteria

- [ ] `make dev` starts server on :8080 and serves WASM at `:8080/` (or via separate static server)
- [ ] Browser loads "Dleague" title screen
- [ ] `curl localhost:8080/health` returns 200 OK
- [ ] CI green on push: lint + test + build WASM
- [ ] WASM bundle <8MB
- [ ] All Go files <200 LOC

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Ebitengine WASM tooling unfamiliar | Use official `ebitengine.org` examples as starting point |
| `wasm_exec.js` version drift with Go upgrades | Pin Go version in CI + Makefile |
| Workspace + 3 modules feels heavy day-1 | Keep `shared/` minimal — only types/interfaces, no logic |
| Air / file-watcher complexity | Skip auto-rebuild MVP; just `make build-wasm` manually |
