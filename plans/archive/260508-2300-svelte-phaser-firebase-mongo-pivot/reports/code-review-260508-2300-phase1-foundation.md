# Phase 1 Foundation — Code Review

**Date:** 2026-05-08
**Reviewer:** code-reviewer
**Scope:** Go workspace + proto + WS ping-pong + /health + Ebiten WASM title scene
**LOC:** 1,231 (hand-written, excluding generated pb/wasm_exec)

## Overall Assessment

Solid Phase 1 scaffolding. Idiomatic Go, all files <200 LOC, kebab/snake naming consistent, build-tagged debug logging is clean. Tests cover core handlers. **No blockers** for proceeding to Phase 2, but several pre-Phase-2 hygiene items below worth addressing before adding game/broadcast code (concurrency surface will grow fast).

## Critical Issues

None.

## High Priority

### H1. `Conn.ws.Write` not protected — race once Phase 2 broadcasts arrive
- **File:** `server/internal/ws/conn.go:104` + `hub.go:18` (no per-conn write mutex)
- Currently safe because only `readLoop`'s reply path writes. As soon as Phase 2 introduces hub→conn pushes (broadcast, async match update), concurrent `Write` calls on a single `*websocket.Conn` will race / corrupt frames. nhooyr/websocket `Write` is **not** safe for concurrent calls.
- **Fix:** add `writeMu sync.Mutex` to `Conn`; wrap every `c.ws.Write(...)` call. Or add a `send chan []byte` + dedicated writer goroutine (gorilla pattern) — preferred for slow-client backpressure (see H2).

### H2. No slow-client backpressure / drop policy
- **File:** `server/internal/ws/conn.go` whole, `hub.go:16-19`
- Hub stores conns directly; no outbound channel. When a slow/dead client doesn't drain, a future broadcast that calls `Write` directly with a 10s timeout will (a) block the broadcaster goroutine for up to 10s per conn, or (b) succeed but build TCP backpressure that kills throughput.
- **Fix before Phase 2:** introduce per-conn `send chan []byte` (bounded, e.g. 64) + writer goroutine. On full channel → close conn (standard "drop on overflow" pattern). Current code has no place to wire this without restructuring, so do it now while the surface is small.

### H3. `readLoop` idle timeout cancels every Read after 60s — kills idle clients
- **File:** `server/internal/ws/conn.go:60-68`
- `context.WithTimeout(ctx, idleTimeout)` per iteration means **any** client silent for 60s gets disconnected. There's no server-side keepalive (ping frame, control or app-level). For a turn-based -dle game this likely cuts users mid-thought.
- Either: (a) raise to 5–10 min, or (b) add periodic `c.ws.Ping(ctx)` and only cut on missed pong. The Phase 1 doc says "ping-pong" but it's app-level Envelope ping initiated by client, not WS control frame keepalive.
- **Fix:** add a writer-goroutine ping ticker (every 30s) with a deadline-extension on pong. Defer until H1/H2 restructuring.

### H4. `migrate.go:splitStatements` breaks on semicolons inside string literals / triggers
- **File:** `server/internal/store/migrate.go:149-167`
- Naive split on `;` will mangle any future migration that contains a function/trigger body, JSON default with `;`, or a string literal containing `;`. Phase 1 init.sql is safe today.
- Also strips `--` line comments only — not `/* ... */` block comments.
- **Fix:** either (a) document constraint "one statement per line until end-of-line `;`, no embedded semicolons" and assert in CI via a lint step, or (b) drop in a real splitter (e.g. `vitess/go/vt/sqlparser` is overkill; `xo/dburl` won't help — write a 30-line tokenizer that respects `'…'`, `"…"`, `\``…`\``, `--`, `/* */`). Document for now; revisit before any non-trivial migration.

### H5. `migrate.go` `pending` counter is dead code
- **File:** `server/internal/store/migrate.go:46-63`
- `pending` is incremented but never used; both branches `return nil`. Either log "applied N migrations" or remove. Trivial but `unused`/`ineffassign` lint should catch (it doesn't because var is read in nothing — it's just incremented).
- **Fix:** `log.Printf("migrate: applied %d", pending)` after the loop — useful operational signal.

## Medium Priority

### M1. `Hub.dispatch` swallows unknown message types — silent prod failure mode
- **File:** `server/internal/ws/hub.go:50-52`
- Logs and returns `(nil, nil)`. Fine for forward-compat (older server, newer client), but no metric/counter, so a deployment where every message is "unknown" looks healthy. Add a counter once observability lands; for now leave a comment marking this as a known soft-fail surface.

### M2. `Conn.handle` returning error from dispatch tears down the connection
- **File:** `server/internal/ws/conn.go:74-77`
- Any malformed payload from a buggy/malicious client → connection closed. Aggressive but defensible at Phase 1. Reconsider once auth lands: malformed game move ≠ should-disconnect. Note for Phase 3 review.

### M3. `health.go:30-34` — error from DB Ping leaks to client as "degraded: db unreachable"
- **File:** `server/internal/http/health.go:30-34`
- The literal text is fine. But `_ = err` discards the actual error before logging. At minimum add `log.Printf("health: db ping: %v", err)` so ops can correlate 503s with DB issue without grepping the DB itself.

### M4. `router.go:NewRouter` static file fallback serves index.html for **every** unknown path — including `/main.wasm` if missing
- **File:** `server/internal/http/router.go:51-53`
- `http.FileServer(http.Dir(abs))` returns 404 for missing files (not SPA fallback) — actually correct here. But `/health` and `/ws` are `Get` and `Get`, while wildcard is `Handle("/*",…)`. Chi prioritizes specific routes. Fine. **Edge case to verify post-Phase-2:** when SPA routing arrives (e.g. `/match/:id`), wildcard FileServer will return 404 instead of falling back to `index.html`. Will need a fallback handler then.

### M5. `client/internal/net/ws_client.go:60` — unmarshal error printed to stdout via `fmt.Printf`, not browser console
- **File:** `client/internal/net/ws_client.go:60, 70`
- In WASM `fmt.Printf` goes to stdout which the JS runtime usually maps to `console.log`, but this is fragile (depends on `wasm_exec.js` version). The debug log file uses `js.Global().Get("console").Call("log", ...)` directly. Inconsistent. Use the same path everywhere.
- **Fix:** route both error sites through `console.error` via syscall/js.

### M6. `client/internal/net/ws_client.go:Close` — `c.ws.Call("close")` may run after socket already closed by server, no idempotency guard
- **File:** `client/internal/net/ws_client.go:99-105`
- Doc-comment says "Safe to call once". Actually JS `WebSocket.close()` IS idempotent (no-op if CLOSING/CLOSED). But `f.Release()` on an already-released `js.Func` panics. `c.funcs = nil` after release is the only guard, so calling Close twice → panic on second call. Doc says no-op; actual is panic.
- **Fix:** either guard with `sync.Once`, or check `if c.funcs == nil { return }` at top of Close.

### M7. `client/cmd/web/main.go:connectAndPing` — goroutine leak on dial failure
- **File:** `client/cmd/web/main.go:38, 50-79`
- Goroutine returns on error, fine. But on success, `c` is never `Close()`'d when the WASM main exits (which it doesn't, but conceptually). More importantly: no reconnection, no retry, no error surface to the title scene. For Phase 1 demo this is OK; tag with TODO for Phase 2.

### M8. No test for `hub.register` race with `hub.dispatch`
- **File:** `server/internal/ws/hub_test.go`
- `register`/`unregister` write under `mu`, `dispatch` reads no shared map (no broadcast yet), so technically race-free today. Once H2 lands, add `go test -race` with a fan-out broadcast scenario. Note for Phase 2.

### M9. `users.go` stub — `_ = ctx` / `_ = u` will trigger `unused-parameter` revive
- **File:** `server/internal/store/users.go:29-30, 37-38`
- The blank assignments are noise; revive's `unused-parameter` is off in default config but on if anyone enables it. Replace with named blank: `func (s *Store) CreateUser(_ context.Context, _ User) error`. Bigger issue: stub returning `ErrNotImplemented` is **not exported in any test** — easy to forget to swap when Phase 3 lands. Add a `// TODO(phase-3): implement` line.

### M10. `Makefile:proto-breaking` — uses `origin/main` only available in CI; locally fails silently
- **File:** `Makefile:72`, `.github/workflows/ci.yml:30`
- Makefile uses `branch=main`, CI uses `branch=origin/main`. Both work in CI (post-checkout); locally, Makefile's `branch=main` works. Inconsistency only; document.

## Low Priority

### L1. `Makefile:1` — `SHELL := /usr/bin/env bash` will fail on systems where bash isn't at that path or `env` is missing — not portable to nix devshell etc. Use `SHELL := bash` and let PATH resolve.

### L2. `Makefile:tools` target uses `@latest` — non-reproducible. Pin versions: `buf@v1.50.0`, `protoc-gen-go@v1.36.11` (match shared/go.mod).

### L3. `.golangci.yml:1` — version "2" is golangci-lint v2 schema. CI uses `version: latest` which currently is v2 — but pin to a specific release (e.g. `v2.5.0`) for reproducibility. Same `@latest` problem as L2.

### L4. `ci.yml:14` — `go-version: "1.26"` is unreleased as of 2026-05 (Go 1.26 lands Aug 2026). If `1.26` here means `1.26.x` once it ships, fine; today CI must be using a beta/rc. Verify CI is actually green.

### L5. `proto/buf.gen.yaml` — only generates Go for one out path. Once a TS client lands (or another lang), this stays a single-language gen. Not a Phase 1 issue.

### L6. `envelope.proto:18-20` — `bytes payload` of inner message is fine but means each message is 2 marshals. For tiny Ping/Pong this is wasteful. Acceptable trade-off (one Envelope decoder, no oneof bloat) — but document the choice in the proto comment so future maintainers don't "fix" it.

### L7. `shared/game/registry.go:26-28` — `Register` panics on duplicate id. OK for `init()` use; bad if any code dynamically registers (test fixtures, plugins). Comment already says "in package init" — fine.

### L8. `shared/game/game.go:19` — `State = []byte` type alias. Comment says JSON-serializable but type is bytes. If games are encouraged to use protobuf (consistent with envelope choice), say so. Cross-format ambiguity invites inconsistency.

### L9. `client/internal/scene/title.go` — magic numbers (200, 280, 170, 320). Trivial. Extract once another scene exists.

### L10. `web/index.html` — no error boundary if `wasm_exec.js` fails to load (only catches `instantiateStreaming` rejection). Add `<script src="..." onerror="...">` for a user-visible message. Cosmetic.

### L11. `web/index.html:17` — `WebAssembly.instantiateStreaming` requires `Content-Type: application/wasm`. Go's `http.FileServer` serves `.wasm` correctly on Go 1.21+. Verify in dev.

### L12. `docker-compose.yml` — Postgres credentials hardcoded `dleague/dleague`. The repo uses MySQL HeatWave per `store.go` and migrations (`utf8mb4`, `BINARY(16)`). **This compose file is dead/wrong** — it'll never be used. Either delete or replace with MySQL service for local dev.

## Edge Cases (Scout)

- **Slow-client broadcast** — H2 above. No backpressure scaffold.
- **WebSocket close from server side** — `defer c.CloseNow()` (conn.go:53) sends RST-style close. Fine for read errors; for graceful shutdown of all conns on `srv.Shutdown`, hub has no `CloseAll()`. Phase 2 must add hub-coordinated drain.
- **`signal.NotifyContext` + `srv.Shutdown` doesn't drain WS** — `srv.Shutdown` closes listener + waits for HTTP handlers. WS handlers block in `readLoop` until `ctx.Done()`, but the request context for an upgraded connection lives until the underlying conn closes. Result: `srv.Shutdown` will hang for 5s, then return. Active WS clients are dropped abruptly. Acceptable Phase 1; needs `hub.CloseAll(reason)` Phase 2.
- **`config.splitCSV` accepts whitespace** but not URL-encoded entries; OK for env-var origins.
- **`store.New` ping is bounded by `bootCtx` 15s in main.go:25** — if MySQL HeatWave is provisioning, 15s might be too short. Increase or add a configurable boot timeout.
- **`store.Migrate` runs on every boot** — racy if two server instances boot concurrently against same DB (e.g. blue/green deploy). MySQL `CREATE TABLE IF NOT EXISTS` is safe but the migration row INSERT could race producing duplicate-key error. For single-instance Coolify deploy fine; flag for Phase 4 multi-instance.
- **`Conn.handle` on UNSPECIFIED type** — returns `(nil, nil)` from dispatch (default case). Test exists. ✓
- **Read limit 1 MiB** (conn.go:17) — fine for pings, may be too low for game state JSON in later phases. Note for revisit.
- **`websocket.Accept` with empty `OriginPatterns`** — nhooyr falls back to same-origin only by reading `Host` header. Behind a reverse proxy that rewrites Host (Coolify/traefik), this could either fail-open or fail-closed depending on setup. Test in staging.

## Test Coverage Gaps

- **No `go test -race`** invocation in Makefile/CI. Add `-race` to `test:` target.
- **No test for `config.Load`** — splitCSV edge cases (trailing comma, only-whitespace), missing DATABASE_URL, empty Addr. Easy 30-line test file.
- **No test for `migrate.go`** beyond `splitStatementsBasic`. `parseMigrationID`, malformed filenames, empty body, comment-only file, all uncovered.
- **No integration test for `/health`** with real `*store.Store` (connected vs unreachable). MYSQL_TEST_DSN gate exists in store_test but not health_test.
- **No test for `Hub` under concurrent register/unregister** — `go test -race -cpu=4` with N goroutines registering would exercise the mutex and catch a future regression.
- **No test for `client/internal/net`** — js+wasm only, but at least a build-tag-gated stub test would pin the API.
- **No test for `router.go` static file serving** — covered indirectly by `TestHealthOK` setup but no GET to `/index.html` to confirm FileServer works.

## Pre-Phase-2 Readiness Checklist

Before adding game core / broadcast:

1. **H1 + H2: per-conn writer goroutine + bounded send channel** — restructure now while only Ping handler exists; later this is a much bigger surgery.
2. **L12: delete `docker-compose.yml` Postgres** — actively misleading (project uses MySQL).
3. **H5: log applied migration count** — operational visibility.
4. **M3: log DB ping error** — debug aid.
5. **Add `go test -race` to Makefile** — catch regressions as concurrency grows.
6. **M5/M6: client `fmt.Printf` → console.error + Close idempotency** — small but easy wins.

## Positive Observations

- **Build-tag debug logging** (`debug_log.go` / `debug_log_noop.go`) — clean pattern, isolates protojson cost from prod binary. Both server + client mirror. Excellent.
- **Single Envelope wire format** — clear contract, request_id propagation correct in `handlePing`. Test `TestWSEndToEndPingPong` validates the full server stack.
- **`store.Store` nil-safe receivers** (`Close`, `Ping`) — defensive and idiomatic.
- **Migration framework** is small, embedded, idempotent at the table-create level, and unit-tested at the parser level. Good Phase 1 minimum.
- **`UpgradeOptions.AllowedOrigins`** plumbed end-to-end from env to nhooyr — no hardcoded `CheckOrigin = true` security footgun.
- **`NewRouter` validates webRoot exists at boot** — fail-fast over per-request 500s.
- **`Conn.SetReadLimit`** prevents trivially OOMing the server.
- **Files all well under 200 LOC** (max: `migrate.go` at 168). Modularity strong.
- **Tests prefer behavior over implementation** — `TestHubRegisterUnregister`, `TestDispatchPingProducesPong` are good shape.
- **Game registry pattern** lifted from ratel-online with attribution in NOTICE — clean reuse.

## Metrics

| Metric | Value |
|---|---|
| Hand-written Go LOC | 1,231 |
| Largest file | `migrate.go` (168) |
| Files >200 LOC | 0 |
| Test files | 4 (`router_test`, `conn_test`, `hub_test`, `store_test`) |
| Test count | ~13 |
| Linters enabled | errcheck, govet, ineffassign, revive, staticcheck, unused |
| `-race` in CI | **No** ⚠ |

## Unresolved Questions

1. **Go 1.26 in `go.work` and `ci.yml`** — is this an unreleased version on a Go tip toolchain, or did this review's date assumption (2026-05) mean 1.26 is real? Verify CI actually compiles.
2. **`docker-compose.yml` Postgres** — is this a leftover from pre-pivot or intentionally kept for future flexibility? Recommend deleting if MySQL HeatWave is the committed path.
3. **WS keepalive policy** — is the 60s `idleTimeout` on read intentional (cut idle) or oversight (should be 10min + ping)? Affects UX significantly.
4. **Phase 2 broadcast model** — fan-out from hub or per-match room? Determines whether H1/H2 fix should be a writer goroutine per conn (general) or a room-based pub/sub.
5. **`State = []byte` alias** — proto-encoded or JSON? Decide before any concrete game ships.
6. **Multi-instance migration safety** — single-replica forever, or eventually scale out? Affects whether `Migrate` needs an advisory lock.

---

**Status:** DONE_WITH_CONCERNS
**Summary:** Phase 1 foundation is solid and idiomatic; no critical defects. Five high-priority items (concurrency restructuring before Phase 2, idle timeout policy, dead docker-compose) should be addressed before adding game core / broadcast paths.
