---
phase: 2
title: "Strip MySQL store + revise config"
status: completed
priority: P1
effort: "0.5d"
dependencies: [1]
---

# Phase 2: Strip MySQL store + revise config

## Context Links

- Plan: [plan.md](plan.md)
- Prior code being removed: `server/internal/store/{store,migrate,users,store_test}.go`, `server/internal/store/migrations/0001_init.sql`

## Overview

Remove the MySQL HeatWave scaffolding committed in `7374f80`. **Downgrade Go toolchain from 1.26 to 1.25.5** in `go.work` and `server/go.mod`. Rewrite `internal/config` to expect Couchbase + Redis + Firebase env vars. Update `cmd/api/main.go` boot to nil-tolerant placeholder (filled by Phases 3+4).

## Key Insights

- `internal/config/config.go` currently has a `DatabaseURL` field for MySQL DSN — replace with three new field groups.
- `cmd/api/main.go` calls `store.New(...)` and `store.Migrate(...)` — both go away. Health-DB-ping in router signature `NewRouter(..., st *store.Store)` becomes `NewRouter(..., s store.Store)` (the new Phase 4 interface).
- Drop `github.com/go-sql-driver/mysql` and `github.com/google/uuid` from go.mod (uuid was only for MySQL PK; Firebase UID supplies IDs now).

## Requirements

- Functional: project compiles after this phase even though the Couchbase + Redis impls aren't wired yet (use temporary nil-tolerant health handler).
- Non-functional: zero MySQL imports remain anywhere; no migration files.

## Architecture

Boot sequence after this phase:
```
main() → config.Load() → (nothing wired) → http.NewRouter(webRoot, hub, wsOpts, nil, nil) → ListenAndServe
```
Phases 3+4 fill the nil. Health endpoint reports plain "ok" until store is wired.

## Related Code Files

- Delete:
  - `server/internal/store/store.go`
  - `server/internal/store/migrate.go`
  - `server/internal/store/users.go`
  - `server/internal/store/store_test.go`
  - `server/internal/store/migrations/0001_init.sql`
  - `server/internal/store/migrations/` (empty dir)
- Modify:
  - `go.work` — `go 1.26` → `go 1.25.5`
  - `server/go.mod` — `go 1.26` → `go 1.25.5`; drop `go-sql-driver/mysql`, `google/uuid`, `filippo.io/edwards25519`
  - `client/go.mod` and `shared/go.mod` (if pinned to 1.26) — same downgrade
  - `server/internal/config/config.go` — drop `DatabaseURL`; add `FirebaseCredentialsJSON`, `FirebaseProjectID`, `CouchbaseConnString`, `CouchbaseUsername`, `CouchbasePassword`, `CouchbaseBucket`, `RedisAddr`, `RedisPassword`
  - `server/cmd/api/main.go` — remove `store.New`, `store.Migrate`, `store.Close` calls; pass `nil` placeholder to `NewRouter` for `store.Store`
  - `server/internal/http/router.go` — signature accepts `store.Store` placeholder; health degrades to "ok" when nil
  - `server/internal/http/health.go` — nil-tolerant

## Implementation Steps

1. `git rm` the MySQL store files + migrations dir.
2. **Downgrade Go to 1.25.5** in all module files (`go.work`, `server/go.mod`, `client/go.mod`, `shared/go.mod`); run `go mod tidy` per module. Verify `go build ./server/... ./shared/...` green on Go 1.25.5.
3. Rewrite `internal/config/config.go`:
   - Required: `FirebaseCredentialsJSON`, `FirebaseProjectID`, `CouchbaseConnString`, `CouchbaseUsername`, `CouchbasePassword`, `CouchbaseBucket`, `RedisAddr`, `RedisPassword`
   - Keep: `Addr`, `WebRoot`, `AllowedOrigins`
4. Update `cmd/api/main.go`:
   - Remove store imports + boot context for migrations
   - `srvhttp.NewRouter(cfg.WebRoot, hub, wsOpts, nil)` (placeholder for store.Store)
4. Update `internal/http/router.go` signature + nil-tolerant health.
5. `cd server && go mod tidy` — drops MySQL deps.
6. `go build ./server/... ./shared/...` must pass.
7. `go test ./server/... ./shared/...` must pass (router_test passes nil/nil).

## Todo List

- [x] Delete MySQL store files (entire `server/internal/store/` removed)
- [x] Go toolchain downgraded to 1.25.5 across `go.work` + all `go.mod` files
- [x] Config rewrite to new env schema (Firebase + Couchbase + Redis groups, JSON validation, sentinel errors per missing key)
- [x] main.go wiring updated — store import removed; router gets 3-arg signature
- [x] Router signature updated — `store.Store` parameter dropped (Phase 3 reintroduces as interface)
- [x] Health simplified — returns plain "ok"; storage-aware health returns in Phase 3
- [x] go.mod dependencies cleaned — `go-sql-driver/mysql`, `google/uuid`, `filippo.io/edwards25519` all gone
- [x] `go build ./server/...` green on Go 1.25.5
- [x] `go test ./server/...` green (config + http + ws all pass; new `config_test.go` covers happy path + 8 missing-key cases + malformed JSON)

## Success Criteria

- [ ] No `mysql`, `uuid`, `database/sql` imports under `server/`
- [ ] `go.mod` only references go-redis (Phase 4 adds), firebase-admin (Phase 5 adds), and existing deps
- [ ] Server boots with new env vars set, listens on `:8080`, `/health` returns 200 "ok"

## Risk Assessment

- **Forgotten MySQL import** — caught by `go build`. Run `grep -r "go-sql-driver\|database/sql" server/` after to confirm zero matches.
- **Test breakage** — `router_test.go` passed `nil` already; signature widening should not break it but verify.

## Security Considerations

- New env var names must not collide with existing OS-level vars. Use `DLEAGUE_` prefix where ambiguous.
- Service-account JSON env var: validate JSON shape on `Load()` to fail-fast on bad config.

## Next Steps

Phase 3 (Couchbase) + Phase 4 (Redis) + Phase 5 (Firebase Admin) can now proceed in parallel.

## Unresolved Questions

- Confirm: are any currently-running deployments relying on MySQL connection? (No — nothing deployed to prod yet per export.)
