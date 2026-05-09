# Code Review — Phase 02 Server Hardening

**Reviewer:** code-reviewer (independent)
**Date:** 2026-05-09
**Scope:** commit-pending diff vs HEAD `1819359`
**Verdict:** ship-ready with 2 minor doc/code clarifications and 4 test-coverage gaps. **No blockers.**

---

## Summary

- Build clean (`go vet ./...`, `go build ./...`).
- `go test -race ./...` green; ran `-count=3` on `internal/ws` — no flakes (1.0s).
- All 14 phase TODOs implemented. Fail-fast assertion, cap, headers, error envelope, request_id cap, /health body removal, migration log, `-race` in CI all present.
- Concurrency design (writeLoop owns `ws.Write` + `Ping`; readLoop enqueues; ctx cancellation tears down both halves) is **correct**.
- File LOC discipline preserved (max touched file is `conn_test.go` @ 179 LOC; all production files <200).

---

## Critical
**None.**

---

## High
**None.** The cap-race, deadlock paths, and ping/pong semantics check out.

---

## Medium

### M1 — Production env detection is too narrow (string equality)
- File: `server/cmd/api/main.go:45`
- `if cfg.Env == "production"` — operators commonly set `DLEAGUE_ENV=prod`, `Production`, etc. Any of those silently bypass the WSOrigins fail-fast assertion → CSWSH risk reopens in misconfigured prod deploys.
- Fix: case-fold + accept short form. e.g. `strings.EqualFold(cfg.Env, "production") || strings.EqualFold(cfg.Env, "prod")`. Or invert: assert in *every* env except `development`/`dev`/`test`.
- Severity: Medium (silent security regression on misconfig).

### M2 — `Hub.MaxConns` is read concurrently, set after construction
- Files: `server/internal/ws/hub.go:26` (public field), `conn.go:46,50` (read in handler), `main.go:50` (write).
- `hub.MaxConns = cfg.MaxConns` runs before `srv.ListenAndServe()` so there is a happens-before via goroutine start in practice; `-race` doesn't flag it. But the field is **public**, mutable, and read without `mu`. Future refactor that touches MaxConns at runtime introduces a silent race.
- Fix options: (a) take `MaxConns` as a `NewHub(maxConns int)` constructor arg and make the field unexported; (b) document on the field that it must only be set during boot.
- Severity: Medium (latent footgun, not a current bug).

---

## Low

### L1 — Buffered response may be dropped on graceful close
- Files: `conn_write.go:27-36`, `conn.go:88-91`.
- When `readLoop` returns (e.g. client closes), main goroutine calls `cancelRead()`. `writeLoop`'s `select` then races between `<-ctx.Done()` and `<-c.send`. Go picks randomly; a final response sitting in `c.send` may be dropped.
- Spec doesn't promise graceful flush on close. Accept for Phase 02. If we ever want it: drain `c.send` non-blockingly in `writeLoop` before returning on ctx.Done.
- Severity: Low (acceptable per spec).

### L2 — Repeated "send buffer full" log spam after cancel
- File: `conn.go:155-158`.
- Once the buffer overflows, `c.cancelRead()` fires but readLoop may still process additional buffered inbound frames before noticing. Each one re-runs `enqueue` → channel still full → another `"ws send buffer full"` log line.
- Fix: track an atomic `closed` flag and skip enqueue once cancelled, OR rely on the (yet-to-be-added) per-conn rate limit (Phase 09).
- Severity: Low.

### L3 — `TrustedProxies` documented as CIDR list, used as on/off flag
- Files: `config/config.go:36-40`, `http/router.go:21-25,55-57`.
- Comment promises CIDR semantics; code only checks `len(...) > 0` and registers `middleware.RealIP` unconditionally. RealIP itself doesn't validate the source IP against the CIDR list. So setting `DLEAGUE_TRUSTED_PROXIES=10.0.0.0/8` and `DLEAGUE_TRUSTED_PROXIES=anything` behave identically.
- This is fine for the current single-proxy Coolify deploy, but the doc is misleading. Either tighten doc to "non-empty enables RealIP" or wire chi's `RealIP` replacement that honours the CIDR allowlist (chi's stock middleware doesn't).
- Severity: Low (doc bug, not a security regression vs. status quo).

### L4 — Pre-accept cap check reads `len(hub.conns)` under RLock
- File: `conn.go:46-54`.
- Pre-check uses RLock; final check in `register` uses Lock. Race between pre-check and Accept is acknowledged in code and handled (`StatusTryAgainLater`). No counter drift; verified by reading `register` (hub.go:40-44) which holds Lock for both check and insert. **Correct.**
- Nit: pre-check is purely an optimisation — could be deleted with no correctness loss. Keep for now to avoid wasted handshake under load.

### L5 — `MaxConns=0` means "unlimited" but `parseIntOr` rejects 0
- Files: `config/config.go:96-99` rejects values `<= 0`. So operators cannot opt out of the cap via env. They get default 1000 or a positive override.
- Hub still treats 0 as unlimited (`hub.go:40`). Slight inconsistency. Either document that the env var is always positive, or allow 0 explicitly.
- Severity: Low.

---

## Nits

- N1: `errorEnvelope` swallows `proto.Marshal` error silently (`error.go:15`). For `Error{code, message}` it cannot fail in practice, but worth a `log.Printf` for paranoia.
- N2: `enqueue` calls `logSend(env)` *before* attempting to enqueue. On overflow drop, debug log claims a send happened. Reorder: log only on successful enqueue.
- N3: `ws.UpgradeHandler` line 78 `defer func() { _ = c.CloseNow() }()` — the explicit `_ =` is fine but the wrapping closure is unnecessary; nhooyr's `c.CloseNow()` returns error. `defer c.CloseNow()` works (lint may warn re unhandled return; the `//nolint` style of `migrate.go:128` is precedented).
- N4: `conn_write.go:36` writeLoop passes outer `ctx` to `c.ws.Write`. If a slow write blocks past pingTimeout, ctx isn't cancelled by ping logic — but write timeouts are governed by HTTP server's `WriteTimeout` (30s) at upper layer. Acceptable.

---

## Test Coverage Gaps

- **G1** — No test for `dispatch` returning error → MESSAGE_TYPE_ERROR with code=500 (`conn.go:131-135`). The two ERROR paths covered are unmarshal-fail and oversized-request_id. Add a third test using a stub dispatcher that returns error.
- **G2** — No HTTP-layer test asserting **429** when `MaxConns` cap is exceeded via real upgrade. `hub_test.go` covers the `register` layer only. A `httptest.NewServer` with `MaxConns=1` + two parallel `websocket.Dial`s would close this gap.
- **G3** — No test for writeLoop ↔ readLoop teardown ordering. The race-prone ctx cancel handoff is exercised only by `TestWSEndToEndPingPong` happy path. Consider a test where the server intentionally cancels mid-frame.
- **G4** — No test confirming `/health` returns 503 with **empty body** when DB ping fails. Existing `TestHealthOK` only covers the `st == nil` branch (router test passes nil store). Hard to test without a DB stub; defer is acceptable.

None of these gaps are blockers. G1 is cheap to add.

---

## Spot-check Observations (positive)

- writeLoop uses `defer ticker.Stop()` — no ticker leak (mitigation R2 honoured).
- Origin assertion at boot is unconditional once `Env=production` — fail-fast correct.
- `request_id` length cap uses `len()` on a Go string — that's bytes, not runes, matching spec ("128B").
- nhooyr `Conn.Ping` is documented safe to call concurrently with `Write` (verified via pkg.go.dev) — the writeLoop owning both is correct.
- nhooyr `Accept` defaults to same-origin when `OriginPatterns` is empty — same-origin clients work even without `DLEAGUE_WS_ORIGINS`. Origin assertion is therefore strictly for cross-origin support; spec intent preserved.
- Security headers scoped via `chi.Group` to `/*` only — verified by reading router.go: `/health` and `/ws` registered before the group. Headers won't break `/ws` upgrade.
- Migration log line `applied %d migrations` correctly placed after the loop, counts only newly-applied (skips already-applied). Replaces dead counter (H5).
- Pre-existing 60s read idle timeout removed from `readLoop` — confirmed by absence of `WithTimeout` calls inside the loop.
- `errors.Is(err, context.Canceled)` filter on read error suppresses noise on graceful shutdown.
- `Conn.cancelRead` field is set once in `UpgradeHandler` and read from goroutines — no concurrent assignment, safe.
- Error envelope on enqueue-marshal failure: silently no-ops. Defensible: `Error{code, msg}` cannot fail to marshal.

---

## Concurrency Verdict (the meat of the review)

Walked through these scenarios:

| Scenario | Outcome |
|---|---|
| Read fails, writeLoop blocked on `ws.Write` | ctx.Cancel propagates, nhooyr Write returns, writeLoop exits via cancelRead() path. OK. |
| Write fails, readLoop blocked on `ws.Read` | writeLoop calls cancelRead() → Read returns ctx.Canceled → readLoop returns. OK. |
| Ping times out (90s) | Ping returns err → cancelRead() → Read returns. nhooyr also closes conn on ctx timeout (per docs) — slight redundancy but no correctness issue. |
| `enqueue` overflows during readLoop | `default` fires, `cancelRead()` called from readLoop's own goroutine, frame dropped. Read loop continues until next `Read` call sees ctx.Err. OK. |
| Cap-race: pre-check passes, register fails | `cancelRead()` + `c.Close(StatusTryAgainLater)` + early return; `defer hub.unregister` not yet registered → no spurious unregister. Counter accurate. OK. |
| Final `<-writeDone` after both cancellations | writeLoop's select hits ctx.Done, returns, closes writeDone. Then `c.CloseNow()` deferred runs. No double-close (nhooyr CloseNow is idempotent). OK. |
| Concurrent `register`/`unregister` under cap | Hub uses Lock on register/unregister, RLock on Count/pre-check. -race passes 3x. OK. |
| 20 goroutines into single Conn.handleFrame | `enqueue` is non-blocking via select-default, no shared mutable state outside `c.send` (channel-safe). -race passes. OK. |

---

## Lint regressions

Did not run `golangci-lint` (would need network). Spot-checked for obvious issues — none found. Trust the agent's "0 new" claim pending CI run.

---

## Things I Would *Not* Block On But Worth a Followup Ticket

1. M1 (Env detection narrowness) — file before next deploy.
2. M2 (Hub.MaxConns mutability) — fold into Phase 03 nhooyr→coder migration since Hub touched anyway.
3. G1 (dispatch-error test) — 10-line addition, nice-to-have.
4. L3 (TrustedProxies semantics doc) — fix comment now, real CIDR enforcement when load-balancing arrives.

---

## Unresolved Questions

- **Q1:** Should `enqueue` log the dropped envelope's `RequestId` for diagnosability? Currently logs only "buffer full". Diagnosing slow clients is hard without it. Trade-off: log volume / log injection. Defer to the per-conn rate-limit phase.
- **Q2:** With `nhooyr` slated for replacement in Phase 03 (→ coder), is it worth investing in M2 / N3 cleanups now or letting the migration sweep them up?
- **Q3:** Spec says "remove `wasm-unsafe-eval` from CSP in Phase 06". Confirm Phase 06 spec carries that follow-up — quick grep suggests the WASM bundle eviction is planned but the CSP tightening should be an explicit checkbox there.

---

**Status:** DONE_WITH_CONCERNS

**Summary:** Implementation is correct and ship-ready. All Phase 02 TODOs done. 2 medium issues are config/code-hygiene (env-name narrowness, public mutable Hub field) — not correctness blockers. 4 small test-coverage gaps worth filing followups. No critical/high-severity findings; race tests green at -count=3.

**Concerns/Blockers:** None blocking. M1 should be addressed before the first prod deploy to avoid silent CSWSH risk under env-name typos.
