# Code Review — Phase 1 Foundation

Plan: `plans/260505-0947-dleague-pvp-game/phase-01-foundation-monorepo.md`
Branch: main | Scope: 18 Go files (783 LOC total) + proto + infra

## Overall

Plan delivered cleanly. Build/run/tests verified. Build-tag split keeps `protojson` strictly out of prod (grep confirms only two `protojson` import sites, both behind `//go:build debug`). Files all <100 LOC, well under the 200 cap. Below are concrete issues, prioritized.

---

## High

### H1. `js.Func` leak on every WS Dial — `client/internal/net/ws_client.go:46,53,69`
Three `js.FuncOf(...)` allocations are never `Release()`d. Per Go syscall/js docs, `js.Func` MUST be released or it pins the Go closure for the WASM lifetime. At Phase 1 there is one `Dial`, so impact is bounded — but Phase 2 reconnection logic will multiply this. Fix now to set the precedent: store the three `js.Func` on `Client`, expose a `Close()` that calls `removeEventListener` and `f.Release()`.

### H2. Race on `openOnce` — `client/internal/net/ws_client.go:46-52,76`
`openOnce` (bool) is set by the JS event-loop callback; `WaitOpen` reads `openCh`. While JS callbacks for one socket are serialized so the duplicate-open guard is safe in practice, mixing a bool flag with a channel close is brittle. Replace with `sync.Once.Do(func(){ close(c.openCh) })` — same intent, race-free, `go vet -race` clean.

### H3. Path traversal / directory listing on `/*` — `server/internal/http/router.go:25-26`
`http.FileServer(http.Dir(webRoot))` follows symlinks within `webRoot` and serves any file that resolves there, including dotfiles, and renders directory indexes if `index.html` is absent. With `webRoot` env-controlled, a misconfig (`DLEAGUE_WEB=/`) leaks the host fs. Mitigations: (1) wrap with `http.StripPrefix` + custom handler that rejects requests whose cleaned path escapes root, (2) disable autoindex by serving 404 for dirs without `index.html`, (3) validate `webRoot` at startup (must exist + contain `index.html`). Phase 1 risk is low (only `make dev`), but harden before any deploy.

### H4. No origin check on WS upgrade — `server/internal/ws/conn.go:32-38`
`websocket.Accept` with no `OriginPatterns` defaults to same-origin only, which is fine for browser usage. But `InsecureSkipVerify` is left to default (false), so cross-origin tools/tests via `wsURL` work because they don't send `Origin`. That's the intended dev setup. **Action:** add a TODO + future `OriginPatterns: []string{cfg.AllowedOrigin}` slot in config so deploy doesn't accidentally widen the gate. CSRF on `/ws` is the real concern in Phase 2+ when sessions/cookies attach.

---

## Medium

### M1. `ReadHeaderTimeout` is the only timeout — `server/cmd/api/main.go:30-34`
HTTP server lacks `ReadTimeout`/`WriteTimeout`/`IdleTimeout`. The chi static handler can stall on slow clients indefinitely. nhooyr WS bypasses these via hijack so it's safe to set them. Add `ReadTimeout: 15s`, `WriteTimeout: 15s`, `IdleTimeout: 60s`.

### M2. `dispatch` swallows unknown types silently — `server/internal/ws/hub.go:49-51`
Returning `nil, nil` is fine, but log at Info level so unknown enums during phase rollout are visible. Right now a misversioned client gets zero feedback. One-liner: `log.Printf("ws unknown type=%v", env.GetType())`.

### M3. Read deadline collapses on every frame — `server/internal/ws/conn.go:52-54`
Per-`Read` `WithTimeout(idleTimeout)` is correct for idle detection. Confirm on partial-frame error (`websocket.CloseError` w/ `StatusAbnormalClosure`) the loop exits — current code returns on any non-Canceled err, which is correct. No change required, but the comment should call out the contract: "60s of silence kills the conn."

### M4. `proto-breaking` swallows failures — `Makefile:72`
`buf breaking ... || true` makes the target always succeed locally. CI doesn't run it at all. Either drop the `|| true` and have devs git-init before running, or add `proto-breaking` to CI. The plan checklist requires it (line 31).

### M5. CI missing `proto-breaking` — `.github/workflows/ci.yml`
Plan calls for "proto-lint + proto-breaking" in CI; only lint is wired. Add a step (`buf breaking --against` previous main).

### M6. `go 1.26` everywhere, plan said 1.23 — `*/go.mod`, `go.work`
All modules and CI pin Go 1.26; plan says "Go 1.23". Likely intentional upgrade, but worth confirming nothing in CI/docker images caps at <1.26.

---

## Low

- L1. `web/index.html:14-21` — `WebAssembly.instantiateStreaming` needs `Content-Type: application/wasm`. `http.FileServer` does set it via mime, but verify after H3 fix doesn't break it.
- L2. `client/cmd/web/main.go:38` — `connectAndPing` goroutine has no cancellation. OK at Phase 1 (one ping then idle), but plan to thread a context before reconnection logic lands.
- L3. `shared/game/registry.go:23-30` — `Register` panics on duplicate. Fine for `init()` use, but document so users don't call it dynamically and crash the server.
- L4. `server/internal/ws/conn.go:44` — `defer c.CloseNow()` runs after `defer hub.unregister(conn)`; LIFO means unregister fires first. Intentional (avoid sending to a torn-down ws), worth a one-line comment.

---

## Plan-vs-Impl Gaps

- **CI missing proto-breaking** (M5). Plan step 16 + checklist line 31.
- **`make proto-breaking` is a no-op** (M4).
- **No `client/internal/ui/` or `client/internal/game/` dirs** — plan architecture lists them as scaffolds. Acceptable since they're empty placeholders; create on first need (YAGNI). Note in plan completion: "ui/game scaffolds deferred to Phase 2".
- **`server/internal/store/` and `internal/config/` directories absent** — plan marks them "Phase 3+", so OK to defer.
- All other todos and success criteria satisfied.

---

## Modularization Forecast (Phase 2-3)

- `server/internal/ws/hub.go` (52 LOC) — `dispatch` switch will explode as message types grow. When it hits ~6 cases, split per-type handlers into `ws/handle_<type>.go` (already done for ping). On track.
- `server/internal/ws/conn.go` (96 LOC) — adding write pump + per-conn send chan in Phase 2 will push past 200. Pre-split now: `conn.go` (lifecycle), `read_loop.go`, `write_loop.go`.
- `client/internal/net/ws_client.go` (89 LOC) — once H1/H2 fixes land plus reconnect logic, splits cleanly into `client.go` + `callbacks.go`.

---

## Positive

- Build-tag isolation is clean and verified (no protojson in prod symbols).
- Tests cover both unit dispatch and end-to-end WS roundtrip.
- `replace` directives in client+server `go.mod` correctly pin shared.
- NOTICE attributions properly mirrored in source files.
- Generated `.pb.go` excluded from revive/staticcheck via `.golangci.yml:17-20` — correct.

---

## Recommended Actions (priority)

1. Fix H1 (`js.Func.Release()`) and H2 (`sync.Once`) in `ws_client.go`.
2. Harden H3: validate `webRoot` at startup; reject paths outside root.
3. Wire CI `proto-breaking` (M5) and remove `|| true` (M4).
4. Add HTTP server timeouts (M1).
5. Pre-split `conn.go` along read/write boundaries before Phase 2 lands.

---

## Unresolved Questions

1. Was Go 1.26 intentional? Plan says 1.23; is there a runtime feature you need from 1.26?
2. Is `webRoot` ever served from a non-controlled path in deploy, or always the embedded `web/`? If always embedded, switch to `embed.FS` and kill H3 entirely.
3. Should `dispatch` panic on `MESSAGE_TYPE_UNSPECIFIED` (programmer error from a new client) vs unknown enum (forward-compat)? Current code treats both the same.
4. CI doesn't run any client-side WASM test build with `-race`. Do you want a `js && wasm` race-test pass once `Client` gets stateful?
