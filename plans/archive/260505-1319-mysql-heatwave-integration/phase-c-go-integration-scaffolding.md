---
phase: C
title: "Go integration scaffolding"
status: pending
priority: P1
effort: 1d
dependencies: [A]
---

# Phase C: Go integration scaffolding

## Context Links

- Phase 1 plan (mentions `server/internal/store/`, `server/internal/config/` as future stubs): [`plans/260505-0947-dleague-pvp-game/phase-01-foundation-monorepo.md`](../260505-0947-dleague-pvp-game/phase-01-foundation-monorepo.md)
- Existing `server/cmd/api/main.go`, `server/internal/http/router.go`, `server/internal/http/health.go`
- Driver: [`github.com/go-sql-driver/mysql`](https://pkg.go.dev/github.com/go-sql-driver/mysql)
- UUID v7: [`github.com/google/uuid`](https://pkg.go.dev/github.com/google/uuid) (v1.6+)

## Overview

Scaffold the Go data layer in `server/internal/store/` against the live MySQL HeatWave instance. Phase C delivers compile-clean code + structure; concrete table-touching methods (CreateUser, etc.) are stubs to be implemented in Phase 3 of the parent plan.

## Requirements

**Functional**
- `Store` struct wraps `*sql.DB` with lifecycle methods (`New`, `Close`, `Ping`)
- Forward-only SQL migrator using `embed.FS`, runs at startup
- `dleague_app` connects with `tls=true&parseTime=true&loc=UTC`
- `/health` reports DB status (200 = DB pingable, 503 = degraded)
- Config layer reads `DATABASE_URL` from env (Coolify-injected)

**Non-functional**
- Connection pool: `MaxOpenConns=25`, `MaxIdleConns=5`, `ConnMaxLifetime=30m`
- Each new Go file <200 LOC
- Migrator is dumb-simple: read `migrations/*.sql` in lexical order, track applied set in a `_migrations` table
- No external migration framework (`goose`, `golang-migrate`, etc.) — keep deps minimal

## Architecture

```
server/
├── cmd/api/main.go              (modified: load config, init Store, run migrator)
├── internal/
│   ├── config/
│   │   └── config.go            (NEW — env loading, validation)
│   ├── http/
│   │   ├── router.go            (modified: pass Store to /health handler)
│   │   └── health.go            (modified: ping DB, return 200|503)
│   └── store/
│       ├── store.go             (NEW — Store struct, New, Close, Ping)
│       ├── migrate.go           (NEW — embed.FS-based forward-only runner)
│       ├── users.go             (NEW — CreateUser, GetUserByEmailLower stubs; signatures only)
│       └── migrations/
│           └── 0001_init.sql    (NEW — initial schema; concrete content from Phase D)
```

## Related Code Files

**Create:**
- `server/internal/store/store.go`
- `server/internal/store/migrate.go`
- `server/internal/store/users.go`
- `server/internal/store/migrations/0001_init.sql` (content owned by Phase D)
- `server/internal/config/config.go`

**Modify:**
- `server/go.mod` — add `github.com/go-sql-driver/mysql`, `github.com/google/uuid`
- `server/cmd/api/main.go` — initialize config, Store, run migrator before starting HTTP server
- `server/internal/http/router.go` — accept `*store.Store`, pass into health handler
- `server/internal/http/health.go` — add `db.PingContext` check, return 503 if it fails

**Delete:** none

## Implementation Steps

1. **Add deps:**
   ```bash
   cd server && go get github.com/go-sql-driver/mysql@latest && go get github.com/google/uuid@latest
   ```
2. **`internal/config/config.go`** — load env, validate `DATABASE_URL` is set, expose `Config{Addr, WebRoot, AllowedOrigins, DatabaseURL}`. Return error if required keys missing.
3. **`internal/store/store.go`** — `Store struct{ db *sql.DB }`, `New(ctx, dsn) (*Store, error)`. Set pool sizes after `sql.Open`. Run `db.PingContext(ctx)` to fail fast.
4. **`internal/store/migrate.go`** — `Migrate(ctx, *sql.DB) error`. Embeds `migrations/*.sql`. Creates `_migrations(id INT PRIMARY KEY, applied_at TIMESTAMP)` if not exists. For each `*.sql` not in the table, BEGIN → exec → INSERT into `_migrations` → COMMIT. Statement separator: `;\n` at file scope (KISS — multi-statement files supported by `multiStatements=true` DSN flag, but safer to one-statement-per-file at this stage).
5. **`internal/store/users.go`** — function signatures only, body returns `errors.New("not implemented")`. This is the contract that Phase 3 of parent plan will fill in.
6. **`internal/store/migrations/0001_init.sql`** — populated by Phase D. For now, leave empty file or single comment line so the file path exists.
7. **Wire `cmd/api/main.go`:**
   - Load config
   - `store.New(ctx, cfg.DatabaseURL)`
   - `store.Migrate(ctx, ...)` — fail-fast on error
   - Pass `*Store` into `srvhttp.NewRouter`
   - `defer store.Close()`
8. **Update `internal/http/router.go`** signature: `NewRouter(webRoot string, hub *ws.Hub, wsOpts ws.UpgradeOptions, st *store.Store) (http.Handler, error)`.
9. **Update `internal/http/health.go`:** accept Store via closure (use the existing `health` handler shape; create a `healthHandler(st *store.Store) http.HandlerFunc` factory).
10. **Add a unit test** for `Store.Ping` against a `mysql:8` Docker container (skip in CI if no Docker). Pattern: `testing.T` skips if `os.Getenv("MYSQL_TEST_DSN") == ""`.
11. **Build verification:**
    ```bash
    cd server && go build -tags debug ./... && go build ./... && go test ./...
    ```
12. **Smoke test against live DB:**
    - Set `DATABASE_URL` env from OCI Vault values
    - Run server, hit `/health`, expect 200 with body indicating DB OK

## Todo List

- [ ] Add MySQL + UUID deps; `go mod tidy`
- [ ] Implement `internal/config/config.go`
- [ ] Implement `internal/store/store.go` with pool defaults
- [ ] Implement `internal/store/migrate.go` with `embed.FS`
- [ ] Stub `internal/store/users.go` signatures
- [ ] Create empty `internal/store/migrations/0001_init.sql` (filled in Phase D)
- [ ] Wire Store into `cmd/api/main.go`
- [ ] Update router signature + health handler factory
- [ ] Build clean (prod + `-tags debug`)
- [ ] Unit test `Store.Ping` against local Docker `mysql:8`
- [ ] Smoke test `/health` against live OCI MySQL HeatWave

## Success Criteria

- [ ] `go build ./...` clean (server module, prod + debug)
- [ ] `go test ./...` clean (server module)
- [ ] `/health` returns 200 with DB pingable, 503 when DB unreachable
- [ ] Server startup log shows "migrations applied" or "no new migrations"
- [ ] All new Go files <200 LOC
- [ ] `users.go` exports the function signatures that Phase 3 of parent plan will use

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| MySQL TLS cert verification fails from Go | Med | Confirm `tls=true` (driver's "preferred verify-full" mode) works with Oracle's cert chain — if not, set `tls=skip-verify` (insecure) in dev only and document |
| Migrator races on concurrent server starts | Low | Use `INSERT IGNORE` semantics + transaction wrapping; not a concern at single-instance scale |
| `embed.FS` requires Go 1.16+ — already on 1.26, fine | None | n/a |
| `wait_timeout` (default 28800s) closes idle conns; pool churn | Low | `ConnMaxLifetime=30m` < `wait_timeout` keeps pool ahead of server-side close |
| Pool size 25 exceeds Always-Free `max_connections` | Low | Phase B records actual value; reduce here if needed |

## Security Considerations

- `DATABASE_URL` contains the `dleague_app` password — must come from Coolify env var sourced from OCI Vault, never committed
- Add `.env*` to `.gitignore` (already there per Phase 1 setup) — confirm
- TLS-required (`tls=true` driver flag) — refuse plaintext fallback
- No SQL string concatenation in `users.go` stubs — use `sql.DB.QueryContext` with `?` placeholders only
- Don't log full DSN; redact password before any error output

## Next Steps

After Phase C success criteria are met:
- Phase D writes the actual content of `0001_init.sql` (concrete schema)
- Phase 3 of parent plan implements `users.go` bodies (CreateUser, hash password, etc.)
