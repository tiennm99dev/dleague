# Security Review — Phase 1 Foundation

**Date:** 2026-05-08  
**Scope:** `server/`, `proto/`, `docker-compose.yml`, `.github/workflows/ci.yml`  
**Method:** Static analysis — no code execution  
**Reviewer:** debugger agent

---

## Summary

Phase 1 is a small, clean foundation. No active vulnerabilities that would cause immediate compromise in dev. However, three structural gaps need to close **before Phase 3 lands auth**: missing WS connection cap, unbounded `request_id` field, and CI actions pinned to mutable tags. The docker-compose DB mismatch (postgres image vs mysql driver) is a correctness bomb that will break any dev spinning up locally.

---

## Findings

### CRITICAL

None at Phase 1 scope. Auth stubs are intentionally unimplemented — rated under High/Missing Scaffolding.

---

### HIGH

#### H1 — Docker-compose / server driver mismatch (`docker-compose.yml:3` + `store/store.go:14`)

**Issue:** `docker-compose.yml` spins up `postgres:16` and exposes port 5432. The server uses `go-sql-driver/mysql` and `sql.Open("mysql", dsn)`. These are incompatible. A developer running `docker compose up` then pointing `DATABASE_URL` at it will get confusing failures; worse, they might add a Postgres DSN flag that silently bypasses TLS on MySQL HeatWave (wrong driver, wrong defaults).

**Evidence:**
- `docker-compose.yml:3`: `image: postgres:16`
- `store.go:14`: `_ "github.com/go-sql-driver/mysql"`
- `store.go:37`: `sql.Open("mysql", dsn)`

**Recommendation:** Replace docker-compose service with `mysql:8.4` (or `mysql:8.0`) image. Update env vars to `MYSQL_ROOT_PASSWORD`, `MYSQL_DATABASE`, etc. Provide a `DATABASE_URL` example in a `.env.example` file matching the MySQL DSN format documented in `config.go`.

---

#### H2 — No WebSocket connection cap (`ws/hub.go`)

**Issue:** `hub.conns` is an unbounded map. Any unauthenticated client can open unlimited WS connections, exhausting file descriptors and goroutines. `Count()` exists but nothing enforces a ceiling.

**Evidence:**
- `hub.go:25-28`: `register()` adds without limit check
- `conn.go:37-57`: `UpgradeHandler` registers before any auth

**Recommendation:** Before Phase 3 auth lands, add a `maxConns int` field to `Hub` and reject upgrades when `Count() >= maxConns` (return `429`). Suggested default: 5000. Wire from config env `DLEAGUE_WS_MAX_CONNS`.

---

#### H3 — `middleware.RealIP` without proxy trust config (`http/router.go:46`)

**Issue:** `chi/middleware.RealIP` blindly rewrites `r.RemoteAddr` from `X-Forwarded-For` or `X-Real-IP`. If the server is ever directly internet-exposed (no reverse proxy), clients can spoof any IP. Future rate-limiting or IP-based bans will be bypassable.

**Evidence:** `router.go:46`: `r.Use(middleware.RealIP)` — no upstream proxy IP allowlist.

**Recommendation:** Either (a) remove `RealIP` now and re-add behind a proxy-IP allowlist when deploying behind Coolify/nginx, or (b) document that this middleware is only safe behind a trusted proxy and add an assertion that verifies `DLEAGUE_TRUSTED_PROXY` is set in non-dev environments.

---

### MEDIUM

#### M1 — CI actions pinned to mutable version tags, not SHAs (`.github/workflows/ci.yml:12,14,40,46,52,62`)

**Issue:** All `uses:` references use tag pins (`@v4`, `@v5`, `@v6`), not immutable commit SHAs. A compromised action publisher can push malicious code to the same tag. This is a supply-chain risk.

**Evidence:**
```
actions/checkout@v4
actions/setup-go@v5
golangci/golangci-lint-action@v6   (×3)
actions/upload-artifact@v4
```

**Recommendation:** Pin each action to a full commit SHA. Example:
```yaml
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2
```
Use a tool like `pin-github-actions` or Dependabot `update-type: digest` to maintain these.

---

#### M2 — Hardcoded credentials in `docker-compose.yml` (`docker-compose.yml:6-8`)

**Issue:** `POSTGRES_USER: dleague` / `POSTGRES_PASSWORD: dleague` are committed. Even though these are dev-only, they (a) establish a bad pattern that gets copy-pasted, and (b) may cause confusion if someone accidentally points a staging env at a docker-compose stack.

**Recommendation:** Move to a `.env` file (already in `.gitignore`) and reference via `${POSTGRES_PASSWORD}` in compose. Add `.env.example` with placeholder values.

---

#### M3 — Unbounded `request_id` string reflected in logs (`ws/hub.go:51` + `proto/envelope.proto:18`)

**Issue:** `request_id` is a free-form protobuf string with no length constraint. It is reflected verbatim in `log.Printf("ws dispatch: unhandled type=%v request_id=%q", ...)` and echoed back in Pong response. An attacker can:
1. Send a 1 MB `request_id` inside a valid Envelope (still under `readLimit`).
2. Flood logs with attacker-controlled content (log injection if `\n` appears in `%q` output — Go's `%q` escapes newlines so direct log injection is mitigated, but volume-based log DoS is not).
3. Future code that stores `request_id` in a DB column without length check will truncate or error.

**Evidence:**
- `envelope.proto:18`: `string request_id = 2;` — no `(validate.rules).string.max_len`
- `hub.go:51`: `log.Printf(... env.GetRequestId())` 
- `ping.go:24`: `RequestId: env.GetRequestId()` — echoed back

**Recommendation:** Add a validation pass in `conn.handle()` before dispatch: reject any envelope where `len(env.GetRequestId()) > 128`. Return a protocol error rather than logging the raw value.

---

#### M4 — Database user has DDL rights (migration concern)

**Issue:** The DSN comment in `config.go` uses a single DB user for both migrations (DDL: `CREATE TABLE`, `ALTER TABLE`) and runtime queries (DML: `SELECT`, `INSERT`). Running migrations with the app's runtime credentials violates least-privilege. If the runtime user is compromised via SQL injection, an attacker can run schema mutations.

**Evidence:** `config.go:28`: single `DatabaseURL` used for both `store.New()` (runtime) and `store.Migrate()` (DDL). No separate `MIGRATION_DATABASE_URL`.

**Recommendation:** Before Phase 3, introduce a `MIGRATION_DATABASE_URL` env var. Runtime user has `SELECT, INSERT, UPDATE, DELETE` only. Migration user (used only at startup) has `ALTER, CREATE, DROP`. Wire in `main.go`: migration uses its own DSN, then closes that connection before the runtime pool opens.

---

#### M5 — `proto.Unmarshal` error closes connection (`ws/conn.go:83-85`)

**Issue:** A malformed protobuf frame returns an error from `handle()`, which propagates to `readLoop()` and causes connection termination (`return`). An attacker who can send one bad frame silently disconnects a legitimate user sharing a connection slot — or trivially disconnects their own connection in a rapid reconnect loop to create churn in the hub's lock.

More critically: once Phase 3 adds authentication state to `Conn`, a parse error before auth completes would silently drop the connection without sending an error response, confusing clients.

**Recommendation:** In `readLoop`, treat parse errors as non-fatal: log, send a binary error envelope back (`MESSAGE_TYPE_ERROR` — add to proto), and `continue` rather than `return`. Fatal only on write errors or context cancellation.

---

### LOW

#### L1 — `nhooyr.io/websocket` is an archived library

**Issue:** `nhooyr.io/websocket v1.8.17` was the last release (Aug 2024) before the repo was archived. The project moved to `github.com/coder/websocket`. The archived repo (`nhooyr/websocket-old`) will not receive security patches.

**Evidence:**
- `server/go.mod:10`: `nhooyr.io/websocket v1.8.17`
- Go proxy confirms origin URL: `https://github.com/nhooyr/websocket-old`
- `github.com/coder/websocket` latest: `v1.8.14` (2025-09-05)

**No known CVE at time of review**, but the library is unsupported. Migration is a one-line import path change plus module replace.

**Recommendation:** Migrate to `github.com/coder/websocket` (API-compatible, same maintainer). `go get github.com/coder/websocket@latest` + update import paths.

---

#### L2 — `/health` leaks DB reachability to unauthenticated callers (`http/health.go:30-33`)

**Issue:** `GET /health` returns `503 degraded: db unreachable` publicly. This confirms DB dependency existence to external scanners. Not severe at Phase 1, but becomes a fingerprinting vector.

**Recommendation:** Before public launch, add a `DLEAGUE_HEALTH_TOKEN` env check or restrict `/health` to internal network / load-balancer IP range. Alternatively, return a generic `503 service unavailable` without the "db unreachable" body.

---

#### L3 — No `Content-Security-Policy` or security headers on static file server (`http/router.go:51-52`)

**Issue:** `http.FileServer` serves the WASM app with no security headers. XSS, clickjacking, MIME sniffing protections are absent.

**Recommendation:** Wrap the file server with a middleware that adds:
- `Content-Security-Policy: default-src 'self'; script-src 'self' 'wasm-unsafe-eval'`
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`

---

#### L4 — `go install ... @latest` in CI is non-deterministic (`.github/workflows/ci.yml:20-23`)

**Issue:** `go install github.com/bufbuild/buf/cmd/buf@latest` and `protoc-gen-go@latest` resolve at CI run time. Two runs on different days may use different versions, silently breaking protobuf generation verification.

**Recommendation:** Pin both to explicit versions:
```yaml
go install github.com/bufbuild/buf/cmd/buf@v1.50.0
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
```

---

#### L5 — No `user:` in docker-compose container (`docker-compose.yml`)

**Issue:** Postgres container runs as root by default. Low severity for a local dev tool, but establishes a bad pattern.

**Recommendation:** Add `user: "999:999"` (postgres uid in the official image) to match the image's internal user. More importantly, fix H1 (wrong image) first.

---

## Missing Auth Scaffolding (Pre-Phase 3 Checklist)

These are **not current bugs** — stubs exist — but must land before Phase 3 ships to production:

| # | Item | File | Risk if Skipped |
|---|------|------|----------------|
| A1 | Session token generation (crypto/rand 32B) | `store/users.go` | Predictable tokens |
| A2 | `sessions` table expiry enforcement (SELECT WHERE expires_at > NOW()) | `store/users.go` | Sessions never expire |
| A3 | WS auth gate: attach `userID` to `Conn` before allowing game messages | `ws/conn.go` | Unauthenticated game actions |
| A4 | Password hashing — bcrypt cost ≥12 or argon2id | `store/users.go` | Weak password storage |
| A5 | `HttpOnly; Secure; SameSite=Strict` on session cookie | future auth handler | Session hijack / CSRF |
| A6 | `DLEAGUE_WS_ORIGINS` must be non-empty in production | `config/config.go` | CSWSH (currently safe — nhooyr defaults to same-origin when empty, but a misconfigured deploy with empty string is silently open) |

---

## Dependency Versions (No Known CVEs at Review Date)

| Package | Version | Status |
|---------|---------|--------|
| `nhooyr.io/websocket` | v1.8.17 | ARCHIVED — migrate to `github.com/coder/websocket` |
| `go-sql-driver/mysql` | v1.10.0 | No CVEs; latest is v1.10.0 |
| `google.golang.org/protobuf` | v1.36.11 | No CVEs; current |
| `github.com/go-chi/chi/v5` | v5.1.0 | No CVEs |
| `github.com/google/uuid` | v1.6.0 | No CVEs |
| `filippo.io/edwards25519` | v1.2.0 | No CVEs |

---

## Priority Order for Next Sprint

1. **H1** — Fix docker-compose to mysql:8.4 (blocks all local dev)
2. **H2** — Add WS connection cap before Phase 3 opens auth endpoints
3. **M1** — Pin CI action SHAs (supply chain, low effort)
4. **M3** — Add `request_id` length validation in `conn.handle`
5. **M4** — Split migration vs runtime DB user before Phase 3 stores passwords
6. **L1** — Migrate `nhooyr.io/websocket` → `github.com/coder/websocket`
7. **H3** — Document / gate `RealIP` middleware before rate-limiting lands
8. **A1–A6** — Auth scaffolding checklist (Phase 3 prerequisite)

---

**Unresolved Questions:**

1. Is `DLEAGUE_WS_ORIGINS` validated to be non-empty in production deploy config (Coolify)? The code allows empty (same-origin safe), but a misconfigured env with `DLEAGUE_WS_ORIGINS=""` is silently permissive.
2. Will the migration user / runtime user split (M4) be addressed in Phase 3 or earlier? HeatWave OCI IAM user provisioning needs to be done before the DB goes live.
3. `splitStatements()` in `migrate.go` strips `--` line comments but does not handle block comments (`/* */`) or string literals containing semicolons. Is the migration SQL guaranteed never to have these? If yes, add a comment; if no, the splitter needs to be more robust.

---

**Status:** DONE_WITH_CONCERNS  
**Summary:** Phase 1 has no exploitable vulnerabilities today. Concerns are H1 (wrong DB image blocks dev), H2 (no WS connection cap), and the auth scaffolding checklist that must close before Phase 3 ships.  
**Concerns:** H1 is a correctness break for any developer running the project locally. H2 becomes a DoS vector the moment Phase 3 opens registration. Prioritize both before the next phase begins.
