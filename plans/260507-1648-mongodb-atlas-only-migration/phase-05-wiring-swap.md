---
phase: 5
title: "Wiring swap in cmd/api/main.go; delete composed/"
status: completed
priority: P1
effort: "0.5d"
dependencies: [3, 4]
---

# Phase 5: Wiring swap (`cmd/api/main.go`); delete `composed/`

## Context links

- Current wiring: [`server/cmd/api/main.go`](../../server/cmd/api/main.go).
- Composed wrapper to delete: [`server/internal/store/composed/composed.go`](../../server/internal/store/composed/composed.go).
- Config schema: [`server/internal/config/config.go`](../../server/internal/config/config.go).

## Overview

Replace the two-backend composed wiring with a single `*mongodb.Client` that satisfies `store.Store` directly. Delete the `composed/` package — there is nothing left to compose. Update `internal/config/` to read `MONGODB_URI` and stop reading Couchbase + Redis env vars (those env vars stay defined in `.env.example` until Phase 7 sweep).

**Important sequencing rule:** This phase rewires only `cmd/api/main.go`. **`cmd/dleague-export/main.go` stays pointed at Couchbase** until Phase 7. Reason: Phase 6 needs to run the Couchbase→JSONL export *after* the API has cut over to Mongo (so beta data isn't lost between cutover and export). Keeping the export CLI on Couchbase preserves that escape hatch through Phase 6. The Couchbase Go package (`internal/store/couchbase/`) therefore continues to exist + compile through Phase 6; only its API consumer is removed in Phase 5.

## Requirements

**Functional:**
- `cmd/api/main.go` constructs one store: `mongodb.New(ctx, mongodb.Config{URI: cfg.MongoURI, Database: "dleague"})`. That value is passed everywhere the old composed store was used.
- HTTP handlers, WS hub, auth middleware, scoring — all unchanged. They depend on `store.Store`, which `*mongodb.Client` now satisfies.
- `cmd/dleague-export/main.go` is **not** rewired in this phase. It continues to use `couchbase.New(...)` directly so Phase 6 can snapshot Couchbase. Phase 7 rewires (or retires) it.
- `*mongodb.Client.New` configures `SetServerSelectionTimeout(5 * time.Second)` so requests fail fast when Atlas is unreachable — preventing the Mongo Go driver's 30s default from hanging the WS hub.

**Non-functional:**
- `*mongodb.Client` must satisfy a compile-time `var _ store.Store = (*Client)(nil)` assertion in `client.go` after Phases 3 + 4 land all methods.
- No changes to public types; pure wiring.

## Architecture

```
Before:
  main.go
    ├── couchbase.New(...) → Persistent
    ├── redis.New(...)      → Cache
    └── composed.New(p, c)  → store.Store

After:
  main.go
    └── mongodb.New(...)    → store.Store (directly)
```

`composed/composed.go` deleted. Its 130-line passthrough wrapper is no longer needed.

## Related code files

- **Modify:** `server/cmd/api/main.go` — replace 3 constructor calls with 1.
- **Do NOT modify:** `server/cmd/dleague-export/main.go` in this phase. (Phase 7 rewires it.)
- **Modify:** `server/internal/config/config.go` — add `MongoURI string` field; keep Couchbase + Redis fields for now (export CLI still reads `CouchbaseConn`; deleted in Phase 7).
- **Delete:** `server/internal/store/composed/composed.go`. Delete the entire `composed/` directory.
- **Add:** `var _ store.Store = (*Client)(nil)` to `server/internal/store/mongodb/client.go`.

## Implementation steps

1. **Add the compile-time assertion** to `client.go`:
   ```go
   var _ store.Store = (*Client)(nil)
   ```
   This will fail to compile if any `Store` method is missing — surfaces gaps before runtime.
2. **Edit `internal/config/config.go`:** add a `MongoURI string` field and load it from the `MONGODB_URI` env var. Leave Couchbase + Redis fields alone (Phase 7 deletes them).
3. **Edit `cmd/api/main.go`:**
   ```go
   // before:
   cb, _ := couchbase.New(ctx, ...)
   r, _ := redis.New(ctx, ...)
   st, _ := composed.New(cb, r)
   // after:
   st, err := mongodb.New(ctx, mongodb.Config{URI: cfg.MongoURI, Database: "dleague"})
   if err != nil { log.Fatal(err) }
   defer st.Close()
   ```
4. **Leave `cmd/dleague-export/main.go` alone** — it stays pointed at Couchbase through Phase 6. Phase 7 rewires it.
5. **Delete `internal/store/composed/composed.go`** and the directory: `git rm -r server/internal/store/composed/`. (Confirm via `git grep -l '"github.com/tiennm99/dleague/server/internal/store/composed"'` — only `cmd/api/main.go` should appear, and that's the file we just rewrote.)
6. **Build:** `go build ./server/...` — must compile clean.
7. **Run unit tests:** `go test ./server/...`.
8. **Run integration tests:** `MONGODB_TEST_URI=... go test -tags=integration ./server/...`.
9. **Smoke-test the API locally:**
   ```sh
   MONGODB_URI=... FIREBASE_CREDENTIALS_JSON=... go run ./server/cmd/api
   curl localhost:8080/health  # expect "ok"
   ```

## Todo list

- [ ] Compile-time `var _ store.Store = (*Client)(nil)` assertion added
- [ ] `MongoURI` field in `config.go`
- [ ] `cmd/api/main.go` rewired
- [ ] `cmd/dleague-export/main.go` rewired
- [ ] `internal/store/composed/` deleted
- [ ] `go build ./server/...` clean
- [ ] `go test ./server/...` green (unit)
- [ ] Integration tests still green
- [ ] Local API smoke test: `/health` returns ok, sign-in flow works against Mongo

## Success criteria

- The compile-time assertion passes (means all `Store` methods are implemented).
- A locally-run API serves at least one happy-path request (sign in → /api/v1/puzzles/me/today → POST /api/v1/attempts) end-to-end against Atlas.
- **Atlas-unreachable smoke test:** Set `MONGODB_URI` to an unroutable address (e.g. `mongodb://10.255.255.1:27017/dleague?serverSelectionTimeoutMS=5000`) and run the server. `/health` (or any handler that touches the store) returns within ~5s, not 30s. Proves the `SetServerSelectionTimeout(5s)` is wired correctly and the WS hub is not hanging on dead Atlas connections.
- No `gocb` or `go-redis` import outside their respective packages — but those packages still exist and compile (deleted in Phase 7).

## Risk assessment

- **Missing `Store` method surfaced at compile time, late.** The assertion catches it; the fix is to go back to Phase 3/4 and add the method. Low risk if Phases 3+4 acceptance criteria are met.
- **`composed/` deletion breaks an unexpected import.** Mitigation: `git grep -l '"github.com/tiennm99/dleague/server/internal/store/composed"'` should return only `cmd/api/main.go` and `cmd/dleague-export/main.go`. If anything else shows up, that's a leak in the seam — fix that file too.
- **Different error wrapping shape.** `composed.New(...)` returned `error`; `mongodb.New(...)` returns the same shape. Behavioral parity expected.

## Security considerations

- `MONGODB_URI` (with embedded password) must not appear in any log line. Audit `main.go` startup logging.
- The deleted Coolify-injected Couchbase/Redis env vars become dead config — purge in Phase 7 to avoid confusion.

## Next steps

Phase 6: migrate beta data from running Couchbase to Atlas (one-shot). Phase 7: delete `couchbase/` + `redis/` packages, drop the docker services, update docs.
