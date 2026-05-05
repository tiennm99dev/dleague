# Phase 1 Foundation Shipped — Go Monorepo + Protobuf Wire + WS Ping-Pong

**Date**: 2026-05-05 11:27  
**Severity**: N/A (Milestone)  
**Component**: Foundation — Go workspace, protobuf, CI, WASM scaffold  
**Status**: Completed  

## What Happened

Phase 1 landed: 3-module Go workspace (`client/`, `server/`, `shared/`), protobuf-over-binary wire protocol, `/health` HTTP endpoint, `/ws` WebSocket with ping-pong handshake, and CI pipeline. Build 9937c7d ships code-reviewer's critical fixes merged mid-session. WASM target hit: 3.5MB gzipped (budget: 8MB).

## The Brutal Truth

This felt **clean and surprisingly friction-free**. Code-reviewer's intervention mid-session (4 high-priority catches) was actually the *right* call — js.Func memory leaks in WASM, environment validation gaps, missing CORS headers on WS upgrade, HTTP timeout defaults. Without that review, we'd have shipped bugs that manifest only under real load or cross-origin requests. That stung a bit (implied sloppy first pass), but better caught here than Phase 3.

The protobuf-over-binary strategy works exactly as intended: prod build has zero protojson code (verified: `strings | grep -c protojson = 0`), debug build logs every frame. Separation via build tags isn't theoretical anymore — it's proven.

Tester's shell tests were annoying (duplicate coverage, zero actual assertions), but the trim was surgical. Real error-path tests remain.

## Technical Details

**What code-reviewer caught (all fixed before merge):**
1. **js.Func leaks**: WASM event handlers never released. Added `defer frame.Release()` + `sync.Once` per handler.
2. **webRoot path traversal**: Env var unchecked. Now validates at startup: `webRoot` must exist and be readable.
3. **WS OriginPatterns missing**: Default allowed all origins. Wired `DLEAGUE_WS_ORIGINS` env (comma-separated), validated on upgrade.
4. **HTTP timeouts**: Server lacked Read/Write/Idle timeouts. Set sensible defaults (5s read, 10s write, 30s idle).
5. **CI proto-breaking circular**: Job ran self-reference on `main`. Moved to PR-only (feature branches compare against merged `main`).

**Build verification:**
```
go build -tags debug   # 16MB raw WASM, includes protojson
go build               # 16MB raw → 3.5MB gzipped, zero protojson symbols
```

**Deviations from plan (intentional):**
- Go 1.26 vs 1.23 planned: no impact, forward-compatible.
- WASM path: `web/main.wasm` (not `dist/wasm/`) — web root already serves `web/`, simpler.
- Deferred scaffolds: `client/internal/{ui,game}` and `server/internal/{store,config}` — YAGNI. When Phase 2 needs them, we'll create with context.
- Database: `docker-compose.yml` defined, Postgres setup deferred (no docker in current env). Activate Phase 3.

## What We Tried

1. **buf.gen.yaml with remote plugin** — avoided (requires buf.build account, overkill MVP). Used local `protoc-gen-go` instead.
2. **Air file-watcher for hot-reload** — punted. Manual `make build-wasm` acceptable for Phase 1. Auto-rebuild adds fragility.
3. **Gorilla WebSocket** — chose `nhooyr.io/websocket` instead (lighter, modern API, no deprecated CGO requirements).
4. **CI on all commits** — proto-breaking self-references failed. Moved to PRs only.

## Root Cause Analysis

The mid-session code-review catches weren't negligence — they were the normal cost of "features first, hardening second." WASM memory management and HTTP server defaults aren't visible until you stress-test. The fix was catch-early-review-hard, not more planning. That's the right tradeoff for MVP.

The tester's shell test duplication came from a "write lots of tests" checklist mentality. Real insight: *coverage numbers lie*. Two tests of the same path add nothing. We corrected it.

## Lessons Learned

1. **Mid-session review beats end-stage firefighting.** Code-reviewer's timely intervention prevented production bugs. Make this a reflex, not a luxury.
2. **Build tags work if you verify them.** Don't trust the separation is real — `strings | grep` the binary. Empirical > theoretical.
3. **Deferred scaffolding is fine.** `client/internal/ui`, `server/internal/store` don't exist yet. When Phase 2 needs them, create with actual use case. YAGNI saved us 200+ LOC of dead code.
4. **Monorepo-at-scale is real work.** `go.work` + 3 modules feels light now; once shared types proliferate, we'll need discipline. Document interfaces early.

## Next Steps

**Phase 2 (Game core + pluggable -dle interface):**
- Implement `game.Game` interface methods (Init, Tick, Render, Result, IsTerminal).
- Plug in first -dle variant (e.g., wordle-mode).
- Client-side game state machine + key input dispatch.
- Extend protobuf schema: GameState, Move, Result messages.
- WS hub routes game messages to active sessions.

**Phase 3 (Postgres + persistence):**
- Activate `server/internal/store/` (repo pattern).
- User table, game history, leaderboard schema.
- Migrations tooling (goose or sql-migrate).
- Session lifecycle: create, join, play, persist result.

**Outstanding questions:**
- Ebitengine scene lifecycle (title → game → results) — implementation detail, but do we need scene abstraction or direct state machine?
- Game registry: dynamic plugin loading, or compile-time registered variants? (Currently compile-time; defer plugin design to Phase 3+.)
- CORS/auth on future API endpoints (user signup, profile, leaderboard) — session strategy (JWT cookies, OAuth)? (Out of scope Phase 1–2.)
