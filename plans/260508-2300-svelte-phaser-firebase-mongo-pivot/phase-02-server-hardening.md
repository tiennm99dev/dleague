---
phase: 2
title: "Server hardening (concurrency + security)"
status: completed
completed_on: 2026-05-09
priority: P1
effort: 1w
dependencies: [1]
---

# Phase 02 — Server hardening

## Context Links
- `plans/reports/code-review-260508-2300-phase1-foundation.md` (H1, H2, H3, H5, M1, M2, M3, M9; pre-Phase-2 checklist)
- `plans/reports/security-review-260508-2300-phase1-foundation.md` (H2, H3, M3, M4, M5, L2, L3)
- `server/internal/ws/conn.go` (readLoop, idle timeout, write path)
- `server/internal/ws/hub.go` (`hub.conns` unbounded, dispatch)
- `server/internal/http/router.go` (RealIP, FileServer, no security headers)
- `server/internal/http/health.go` (degraded text leak)
- `proto/dleague/v1/envelope.proto` (no `MESSAGE_TYPE_ERROR`)

## Overview
Fold all Phase 1 audit findings (code-review H1/H2/H3/H5 + security-review H2/H3/M3/M4/M5/L2/L3) into hardening commits before any new feature work. Restructure `Conn` to bounded `send` channel + writer goroutine. Add WS connection cap, origin enforcement, request_id length cap, error envelope return path, security headers on static, `go test -race` in CI.

## Key Insights
- **H1+H2 (code-review):** `nhooyr.io/websocket` `Write` is NOT safe for concurrent calls. Phase 1 is safe today only because no broadcast exists. Adding the writer goroutine NOW while surface is small avoids surgery later.
- **H3 (code-review):** 60s read idle timeout cuts mid-thought users. Replace with WS-level `Ping` ticker (30s) + 90s missed-pong cut.
- **H4 (code-review):** SQL splitter — moot once Phase 04 deletes `migrate.go`. Don't touch here.
- **H5:** `pending` counter dead — log applied count. Cheap.
- **Security H2:** `hub.conns` unbounded → unauth DoS. Cap at env-driven default 1000.
- **Security H3:** `middleware.RealIP` is spoofable without proxy trust. Gate with `DLEAGUE_TRUSTED_PROXIES` allowlist.
- **Security M3:** `request_id` reflected to logs unbounded. Cap 128 bytes.
- **Security M5:** Proto unmarshal error → connection close. Should log + return `MESSAGE_TYPE_ERROR` envelope, continue.
- **Security L2/L3:** `/health` info leak; static FileServer no CSP.

## Requirements
**Functional:**
- Per-conn bounded `send chan []byte` (cap 64) + writer goroutine; closes conn on overflow.
- WS-level keepalive: 30s ping interval, 90s pong timeout; replaces 60s read idle.
- Hub connection cap from `DLEAGUE_MAX_CONNS` env (default 1000); reject upgrades over cap with 429.
- WS origin allowlist enforced: `DLEAGUE_WS_ORIGINS` non-empty asserted in non-dev.
- `request_id` >128 bytes → reject with `MESSAGE_TYPE_ERROR` envelope; do not log raw.
- Proto unmarshal failure → emit `MESSAGE_TYPE_ERROR` to client, do not close conn.
- New proto message `MESSAGE_TYPE_ERROR` + `Error{code, message}` payload.
- `middleware.RealIP` only registered when `DLEAGUE_TRUSTED_PROXIES` non-empty (CIDR list). Otherwise skip.
- Static-file middleware adds: `Content-Security-Policy: default-src 'self'`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`.
- `/health` returns 503 with empty body when DB unreachable; logs full error server-side. Removes "degraded: db unreachable" body.
- `go test -race ./...` in Makefile + CI.

**Non-functional:**
- No new external deps.
- Each touched file remains <200 LOC.
- Tests pass with `-race` flag.

## Architecture
```
HTTP upgrade
  └─ origin check (env allowlist) ─ rejects 403
  └─ conn-cap check ─ rejects 429
  └─ websocket.Accept
       └─ Conn{ws, hub, send chan []byte (cap 64)}
            ├─ readLoop (proto.Unmarshal → dispatch → enqueue resp)
            │   └─ on Unmarshal error: enqueue MESSAGE_TYPE_ERROR, continue
            ├─ writeLoop (drain send, ws.Write, ping ticker 30s)
            │   └─ on send-chan-closed or pong miss: cancel readLoop ctx
            └─ on close: hub.unregister
```
Data: `Hub.dispatch` already returns `*Envelope`; new path enqueues to `conn.send` instead of returning bytes back through `readLoop`. `readLoop` no longer writes directly.

## Related Code Files
**Create:**
- `server/internal/ws/error.go` — helper `errorEnvelope(reqID string, code int32, msg string) *dleaguev1.Envelope`.
- `server/internal/http/security_headers.go` — chi middleware adding CSP/XFO/XCTO/Referrer.

**Modify:**
- `server/internal/ws/conn.go` — add `send chan []byte`, `writeLoop`, ping ticker, request_id cap, error envelope on Unmarshal error.
- `server/internal/ws/hub.go` — `MaxConns` field, `register` returns error if over cap.
- `server/internal/ws/ping.go` — unaffected (app-level Ping/Pong stays for clock-sync).
- `server/internal/config/config.go` — add `MaxConns int`, `TrustedProxies []string`.
- `server/internal/http/router.go` — gate `middleware.RealIP`; mount security headers on `/*` static handler.
- `server/internal/http/health.go` — drop response body on 503; log error.
- `server/cmd/server/main.go` — pass `MaxConns` to hub; assert WS origins non-empty in prod.
- `proto/dleague/v1/envelope.proto` — add `MESSAGE_TYPE_ERROR = 4` + `message Error { int32 code = 1; string message = 2; }`.
- `Makefile` — add `-race` to `test` target.
- `.github/workflows/ci.yml` — `-race` already implied if Makefile drives it; verify.

**Delete:** none.

## Implementation Steps
1. **Proto:** add `MESSAGE_TYPE_ERROR` enum value + `Error` message in `envelope.proto`. Run `make proto-gen`.
2. **Hub cap:** add `Hub.MaxConns int`; `register` returns `ErrAtCapacity` if `len(h.conns) >= MaxConns`. `UpgradeHandler` returns 429 on that error.
3. **Per-conn writer goroutine:** add `send chan []byte` (cap 64) to `Conn`. Spawn `writeLoop(ctx)` from `UpgradeHandler` after accept; goroutine drains `send`, calls `ws.Write`, ticks ping every 30s.
4. **Ping/pong policy:** pong handler resets a per-conn deadline timer (90s); if missed → cancel readLoop context. Drop the per-iteration 60s `WithTimeout` in `readLoop`.
5. **Read path:** `readLoop` calls `dispatch`; if dispatch returns response envelope, marshal and `select { case c.send <- bytes: default: close(c) }` (drop-on-overflow).
6. **Unmarshal error handling:** `proto.Unmarshal` failure → build `MESSAGE_TYPE_ERROR` envelope (code=400, msg=concise reason), enqueue to `send`, `continue` not `return`. Log the error server-side.
7. **request_id cap:** in `conn.handle`, after Unmarshal, if `len(env.GetRequestId()) > 128` → enqueue `MESSAGE_TYPE_ERROR` (code=400, msg="request_id too long"), continue. Never log the raw bad id.
8. **Origin allowlist:** in `main.go`, if `cfg.Env == "production" && len(cfg.WSOrigins) == 0` → fail boot with clear error. (Closes security review Q1.)
9. **TrustedProxies gate:** in `router.go`, `r.Use(middleware.RealIP)` only when `len(cfg.TrustedProxies) > 0`. Document semantics in `config.go` comment.
10. **Security headers middleware:** new `security_headers.go` middleware applied to the static FileServer route only (not `/ws` or `/health`). CSP allows `'self'` + `'wasm-unsafe-eval'` (kept for now; tightened in Phase 06 once we ditch WASM).
11. **`/health` body:** on DB ping failure, `log.Printf("health: db ping: %v", err)` then `w.WriteHeader(503); return` (no body).
12. **Migration log line (H5):** add `log.Printf("migrate: applied %d migrations", pending)` AFTER the loop. Note: this file dies in Phase 04 — minimal effort here, easier to keep diff clean.
13. **Tests:**
    - `conn_test.go`: add table-driven cases for unmarshal error → error envelope; oversized request_id; concurrent write via send channel under `-race`.
    - `hub_test.go`: cap enforcement, register-then-unregister concurrency under `-race`.
14. **Makefile:** `test:` target → `go test -race ./...`. CI inherits via `make test`.
15. **Manual verify:** `wscat -c ws://localhost:8080/ws -H 'Origin: http://evil.com'` → 403.

## Todo List
- [x] Proto: `MESSAGE_TYPE_ERROR` + `Error` message
- [x] Hub `MaxConns` cap + 429
- [x] Per-conn `send` channel + writer goroutine
- [x] Ping ticker + pong deadline (replace 60s idle)
- [x] Unmarshal error → error envelope, no close
- [x] request_id length cap (128B)
- [x] WS origin allowlist asserted in prod
- [x] `RealIP` gated by `TrustedProxies`
- [x] Static security-headers middleware
- [x] `/health` body removed on 503
- [x] Migration applied-count log line
- [x] `go test -race` in Makefile
- [x] Tests for new failure paths
- [x] CSP allows wasm-unsafe-eval (drop in phase 06)

## Success Criteria
- [ ] `go test -race ./...` green
- [ ] Sending malformed proto → client receives `MESSAGE_TYPE_ERROR`, conn stays open
- [ ] Opening 1001 conns when `DLEAGUE_MAX_CONNS=1000` → 1001st rejected 429
- [ ] WS upgrade with disallowed Origin → 403
- [ ] Production boot with empty `DLEAGUE_WS_ORIGINS` → fail-fast
- [ ] `curl /health` with DB down → 503 + empty body
- [ ] Browser DevTools shows CSP/XFO/XCTO headers on `/index.html`
- [ ] Idle conn for 5 min → still alive (ping/pong working)

## Risk Assessment
| Risk                                          | Likelihood | Impact | Mitigation                                                |
|-----------------------------------------------|------------|--------|-----------------------------------------------------------|
| `send` channel overflow on slow client        | Medium     | Medium | Drop-on-overflow with conn close; document policy.        |
| Ping ticker leaks on disorderly close         | Low        | Low    | `defer ticker.Stop()` in writeLoop.                       |
| CSP `wasm-unsafe-eval` lingers post Phase 06  | High       | Low    | Phase 06 todo: remove `wasm-unsafe-eval` from CSP.        |
| Test flake from `-race` exposing existing bug | Medium     | High   | Phase blocker if found; fix before merging.               |
| Unmarshal failure floods logs                 | Low        | Low    | Already capped via dropped raw `request_id`; rate-limit if observed. |

## Security Considerations
- **CSWSH** (cross-site WS hijacking): origin allowlist + boot-time non-empty assertion closes this.
- **DoS via unbounded conns:** cap closes it.
- **Log injection / log volume DoS via request_id:** length cap closes it.
- **IP spoofing via XFF:** `RealIP` gating closes it.
- **Information leak via /health:** body removal closes it.
- **Clickjacking / MIME sniffing on static:** headers close them.
- Defer **per-conn rate limit** to Phase 09 (sync PvP) — not needed pre-auth.

## Next Steps
- Phase 03 — WS library migration nhooyr → coder. Independent of behavior changes here; only import paths.
