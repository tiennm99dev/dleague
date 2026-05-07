---
phase: 3
title: "Persistent half port"
status: completed
priority: P1
effort: "1.5d"
dependencies: [2]
---

# Phase 3: Persistent half port (users, puzzles, attempts, matches, Export)

## Context links

- `Store` interface methods to implement: [`server/internal/store/store.go`](../../server/internal/store/store.go) lines 18–35.
- Couchbase reference impls (port from these): `server/internal/store/couchbase/{users,puzzles,attempts,matches,export}.go`.
- Memstore reference shape: `server/internal/store/memstore/` (cleanest semantics — read first, then translate to Mongo).

## Overview

Port the persistent half of `store.Store` (10 methods) to MongoDB. Shapes are flat JSON dictionaries — same as Couchbase — so the port is straightforward. The hardest method is `UpsertUserOnFirstAuth` (idempotent first-auth ledger semantics). Each method gets a unit-style integration test that runs against Atlas with the `MONGODB_TEST_URI` gate.

## Requirements

**Functional — implement against `*mongodb.Client`:**

| Store method | Mongo collection | Mongo op |
|---|---|---|
| `UpsertUserOnFirstAuth(claims) (User, error)` | `users` | `findOneAndUpdate({uid}, {$setOnInsert: betaFields, $set: lastSeen}, upsert+returnAfter)` |
| `GetUser(uid) (User, error)` | `users` | `findOne({uid})` |
| `TouchLastSeen(uid, at) error` | `users` | `updateOne({uid}, {$set:{lastSeen:at}})` |
| `GetPuzzle(date) (Puzzle, error)` | `puzzles` | `findOne({_id: date})` |
| `PutPuzzle(p Puzzle) error` | `puzzles` | `replaceOne({_id: p.Date}, p, upsert)` |
| `GetAttempt(uid, date) (Attempt, error)` | `attempts` | `findOne({uid, puzzleDate: date})` |
| `UpsertAttempt(a Attempt) error` | `attempts` | `replaceOne({uid: a.UID, puzzleDate: a.PuzzleDate}, a, upsert)` |
| `GetMatch(matchID) (Match, error)` | `matches` | `findOne({_id: matchID})` |
| `UpsertMatch(m Match) error` | `matches` | `replaceOne({_id: m.ID}, m, upsert)` |
| `ListUserMatches(uid, n) ([]Match, error)` | `matches` | `find({players: uid}).sort({createdAt: -1}).limit(n)` |
| `Export(ctx, w) error` | all 4 | iterate every collection's docs; emit JSONL `{collection, doc}` |

**Non-functional:**
- `ErrNotFound` returned when a doc is missing (sentinel from `internal/store/errors.go`).
- All methods accept `ctx`; respect cancellation.
- BSON struct tags on `User`/`Puzzle`/`Attempt`/`Match`. Reuse the existing JSON tags via `bson:",inline"` where safe; otherwise add explicit `bson:"..."` tags. **Decision: add explicit `bson` tags** — Mongo defaults to lowercased field names, which won't match JSON tags exactly (e.g. `IsBetaTester` → bson default `isbetatester`, but JSON tag is `isBetaTester`).

## Architecture

```
internal/store/mongodb/
├── client.go              (from Phase 2)
├── indexes.go             (from Phase 2)
├── users.go               ← Phase 3
├── puzzles.go             ← Phase 3
├── attempts.go            ← Phase 3
├── matches.go             ← Phase 3
├── export.go              ← Phase 3
└── mongodb_test.go        (extended in Phase 3)
```

## Related code files

- **Create:** `server/internal/store/mongodb/users.go`, `puzzles.go`, `attempts.go`, `matches.go`, `export.go`.
- **Modify:** `server/internal/store/mongodb/mongodb_test.go` — add round-trip tests per method.
- **Modify:** `server/internal/store/store.go` — add `bson` struct tags to `User`/`Puzzle`/`Attempt`/`Match`. (Won't break Couchbase impl; Couchbase serializes via JSON tags which still work.)
- **No changes to:** `internal/store/errors.go`, `composed/` (yet), `couchbase/`, `redis/`, `memstore/`.

## Implementation steps

1. **Add `bson` struct tags** to entity types in `store.go`. Keep JSON tags. Example:
   ```go
   type User struct {
       UID         string    `json:"uid"          bson:"uid"`
       Email       string    `json:"email,omitempty" bson:"email,omitempty"`
       // ... etc
   }
   ```
2. **`users.go`:** implement `UpsertUserOnFirstAuth` with `$setOnInsert` for beta-tester fields (only set on first insert) and `$set` for `lastSeen` + provider fields. Use `findOneAndUpdate` with `ReturnDocument: After` to get the post-update doc atomically. Implement `GetUser` and `TouchLastSeen`.
3. **`puzzles.go`:** `_id` is the date string. `GetPuzzle` and `PutPuzzle` straightforward.
4. **`attempts.go`:** compound key `{uid, puzzleDate}`. Use `_id` set to a derived string `uid + "::" + date` to keep parity with the Couchbase impl, OR use the unique compound index from Phase 2 and don't set `_id`. **Pick: use compound index, let Mongo generate `_id`.** Cleaner.
5. **`matches.go`:** `_id = match.ID`. `ListUserMatches` filters by `players: uid` (Mongo array query auto-matches), sorts by `createdAt: -1`, limits to `n`.
6. **`export.go`:** iterate `users`, `puzzles`, `attempts`, `matches` in that order. For each, open a cursor with empty filter, decode each doc into a generic `bson.M` (or `map[string]any`), wrap in `{collection: "...", doc: ...}`, JSON-encode, write line + newline. Stop on first cursor error.
7. **Tests** (add to `mongodb_test.go`):
   - `TestUpsertUserOnFirstAuth_Idempotent` — call twice with same claims, assert `betaSignupAt` unchanged on 2nd call but `lastSeen` updated.
   - `TestGetPuzzle_NotFound` — assert `errors.Is(err, store.ErrNotFound)`.
   - `TestUpsertAttempt_Replace` — write, mutate, write; assert read returns the mutated value.
   - `TestListUserMatches_OrderAndLimit` — write 5 matches, assert order by `createdAt` desc.
   - `TestExport_Streams4Collections` — write one of each, run Export, parse the JSONL output, assert exactly 4 lines + correct collection labels.
8. **Each test cleans up its own collections** with `defer db.Collection(...).Drop(ctx)` to keep the test DB tidy.
9. **Run integration tests:** `MONGODB_TEST_URI=... go test -tags=integration ./server/internal/store/mongodb/...`

## Todo list

- [ ] `bson` tags added to `User`/`Puzzle`/`Attempt`/`Match` in `store.go`
- [ ] `users.go` (3 methods) + tests
- [ ] `puzzles.go` (2 methods) + tests
- [ ] `attempts.go` (2 methods) + tests
- [ ] `matches.go` (3 methods) + tests
- [ ] `export.go` (1 method) + tests
- [ ] All integration tests pass against Atlas M0
- [ ] `go test ./...` green (memstore + couchbase + redis untouched)
- [ ] `golangci-lint run ./server/internal/store/mongodb/...` clean

## Success criteria

- Every persistent-half method has a green integration test against Atlas.
- The same memstore-targeted upper-layer test suite (in `internal/api/...`) passes when wired against `mongodb` — verifying no semantic drift.
- `Export` produces line-for-line equivalent output (modulo doc ordering) when run against memstore vs mongodb with the same seed data.

## Risk assessment

- **`UpsertUserOnFirstAuth` race:** two concurrent first-auth requests for the same uid. With `findOneAndUpdate` + `$setOnInsert` + `upsert: true`, Mongo serializes; one wins the insert, the other sees the existing doc. Both return the same final state. ✓
- **`bson` tag default mismatch:** if you forget a `bson` tag, Mongo lowercases the Go field name silently. Add a struct-level lint check or rely on the integration tests catching it.
- **Cursor leak in `Export`:** must `defer cursor.Close(ctx)` per cursor. Easy to forget.
- **Time-zone drift:** `time.Time` round-trips through BSON as UTC. Confirm encoding/decoding preserves millisecond precision (Mongo dates are millisecond-resolution).

## Security considerations

- No secret writes happen in this layer; SCRAM auth happens at the connection level (Phase 2).
- `Export` is invoked from `cmd/dleague-export` only; HTTP layer has no path that triggers it.

## Next steps

Independent of Phase 4 (different files, different concerns). Both must complete before Phase 5 (wiring swap).
