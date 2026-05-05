---
phase: 3
title: "Couchbase 8.0 Go integration (gocb v2, primary store)"
status: pending
priority: P1
effort: "2d"
dependencies: [2]
---

# Phase 3: Couchbase 8.0 Go integration

## Context Links

- Plan: [plan.md](plan.md)
- Couchbase Go SDK: https://docs.couchbase.com/go-sdk/current/hello-world/start-using-sdk.html
- gocb pkg: https://pkg.go.dev/github.com/couchbase/gocb/v2

## Overview

`internal/store/couchbase` package wrapping `gocb.Cluster` + `Bucket` + `Collection` handles. Implements the **primary-store half** of the `store.Store` interface (Phase 4 implements the cache half; `internal/store/composed/` glues them together for `main.go`). Connection target: docker-compose internal hostname `couchbase` over plain `couchbase://` (no TLS — internal network).

## Key Insights

- Self-hosted Couchbase 8.0 connection: `couchbase://couchbase` (service name + plain protocol). gocb auto-discovers the rest of the cluster topology.
- gocb v2 supports Couchbase Server 7.0+; v8.0 backward compatible. Use latest gocb release.
- TLS optional inside docker network — KISS, skip unless required externally.
- Document IDs: Firebase UID for `users`; UUIDv7 for `matches`; date string for `puzzles`; `<uid>::<date>` for `attempts`.
- Indexes created in Phase 1 (primary on each collection); add secondary indexes here only if N1QL queries need them.

## Requirements

- Functional: open cluster, ping, expose typed CRUD per collection. Implement `store.Store` methods: `UpsertUserOnFirstAuth`, `GetUser`, `TouchLastSeen`, `GetPuzzle`, `PutPuzzle`, `GetAttempt`, `UpsertAttempt`, `GetMatch`, `UpsertMatch`, `ListUserMatches`, plus `Export()` for persistent docs.
- Non-functional: graceful Close, 5s op timeout, panic-safe reconnect on container restart.

## Architecture

```
internal/store/
├── store.go              # interface + entity types (defined here in Phase 3 if not yet)
├── errors.go
├── couchbase/
│   ├── client.go         # Cluster open/Ping/Close, WaitUntilReady on boot
│   ├── users.go          # users collection — UpsertUserOnFirstAuth, GetUser, TouchLastSeen
│   ├── puzzles.go        # puzzles collection — GetPuzzle, PutPuzzle
│   ├── attempts.go       # attempts collection — GetAttempt, UpsertAttempt
│   ├── matches.go        # matches collection — GetMatch, UpsertMatch, ListUserMatches (N1QL)
│   ├── export.go         # Export — N1QL scan + JSONL stream of all 4 collections
│   └── couchbase_test.go # gated by COUCHBASE_TEST_CONN
```

Document shapes (flat; one entity = one doc):
- `users.<firebase_uid>` — `{uid, email, displayName, provider, isBetaTester, betaSignupAt, createdAt, lastSeen}`
- `puzzles.<YYYY-MM-DD>` — `{date, word, hint, difficulty}`
- `attempts.<uid>::<date>` — `{uid, puzzleDate, guesses, won, score, completedAt, inProgress}`
- `matches.<matchId>` — `{id, players, mode, puzzleDate, state, turns, winner, createdAt, endedAt}`

## Related Code Files

- Create:
  - `server/internal/store/store.go` (if not yet defined; this phase + Phase 4 share it)
  - `server/internal/store/errors.go`
  - `server/internal/store/couchbase/{client,users,puzzles,attempts,matches,export,couchbase_test}.go`
  - `server/internal/store/memstore/memstore.go` (in-memory full-interface impl for tests)
- Modify:
  - `server/cmd/api/main.go` — wire `couchbase.New(ctx, cfg)` (returns concrete; passed to `composed.New` in Phase 4)
  - `server/internal/http/health.go` — add Couchbase Ping path

## Implementation Steps

1. `cd server && go get github.com/couchbase/gocb/v2@latest`
2. Define `store.Store` interface + `store.User/Puzzle/Attempt/Match/Rank/AuthClaims` types + sentinel errors.
3. `couchbase.Client`:
   - `gocb.Connect("couchbase://couchbase", ClusterOptions{Authenticator: PasswordAuthenticator{...}})`
   - `cluster.WaitUntilReady(ctx, 30s, &WaitUntilReadyOptions{ServiceTypes: []ServiceType{ServiceTypeKeyValue, ServiceTypeQuery}})`
   - Get `Bucket("dleague")` and the four `Collection` handles.
4. Per-collection files: `Upsert`/`Get`/`Remove` ops on `Collection`.
5. `users.go`: `UpsertUserOnFirstAuth` uses `Upsert` with subdoc `INSERT` for `isBetaTester`/`betaSignupAt` so they're stamped only on doc creation; `TouchLastSeen` uses subdoc `Replace` for `lastSeen` only.
6. `matches.go`: `ListUserMatches(uid, n)` runs N1QL: `SELECT META().id, * FROM \`dleague\`.\`_default\`.\`matches\` WHERE ANY p IN players SATISFIES p = $uid END ORDER BY createdAt DESC LIMIT $n`. Requires secondary index — create in this phase via SDK or manually.
7. `export.go`: `SELECT META().id, * FROM ...` per collection, stream to `io.Writer` as JSONL.
8. `memstore.go`: trivial map-backed full-interface impl.
9. Tests: gated integration on `COUCHBASE_TEST_CONN`; round-trip per entity type.

## Todo List

- [ ] gocb v2 added to go.mod
- [ ] `store.Store` interface + entity types + sentinel errors
- [ ] `couchbase.Client` New/Ping/Close + WaitUntilReady
- [ ] User upsert with first-write-only beta fields (subdoc INSERT)
- [ ] Puzzle Get/Put
- [ ] Attempt Get/Upsert
- [ ] Match Get/Upsert
- [ ] N1QL `ListUserMatches` + supporting index
- [ ] `Export` JSONL streamer
- [ ] `memstore` full impl
- [ ] Health endpoint Pings Couchbase
- [ ] Integration test passes with live Couchbase

## Success Criteria

- [ ] `gocb` import only inside `internal/store/couchbase/` (verify with grep)
- [ ] All persistent CRUD methods green against live container
- [ ] `isBetaTester` set on first upsert; never overwritten on subsequent calls
- [ ] `Export` produces valid JSONL for ≥100 docs in <2s
- [ ] `go test ./server/internal/store/...` green using `memstore` for upper-layer tests

## Risk Assessment

- **gocb compat with Couchbase 8.0** — verify on first connect; fallback to 7.6 image if SDK trips.
- **Subdoc INSERT semantics** — confirm "insert only if absent" works for stamping beta fields atomically. Otherwise use CAS-based read-modify-write.
- **N1QL latency on small dataset** — should be <50ms locally; primary indexes from Phase 1 cover most queries.
- **Container restart loses in-progress connections** — gocb retries; ensure 5s op timeout doesn't cascade.

## Security Considerations

- App user `dleague_app` has only `Application Access` role on `dleague` bucket — no admin.
- No TLS inside docker network is acceptable; flag for future review if Couchbase ever moves off-VM.
- N1QL queries parameterized — no string concat — to avoid injection.

## Next Steps

Phase 4 implements the cache/leaderboard half of the interface. Phase 5 calls `UpsertUserOnFirstAuth` from token verifier middleware.

## Unresolved Questions

- Couchbase secondary index for `matches.players` — create automatically on first N1QL query, or pre-create in Phase 1? Pre-create is safer.
- `Export` format — pure JSONL or wrapped with metadata header (collection name, count)? KISS = pure JSONL with collection prefix in `META().id`.
