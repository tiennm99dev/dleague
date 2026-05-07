---
title: "Red-team review: MongoDB Atlas-only migration plan"
type: code-review
date: 2026-05-07
plan_under_review: plans/260507-1648-mongodb-atlas-only-migration/
reviewer_role: staff-engineer / adversarial
---

# Red-team review — MongoDB Atlas migration plan

Hunting false assumptions, sequencing breaks, contract drift before any code lands.

## Top issues (by severity)

### BLOCKING-1 — Phase 6 cannot export from Couchbase after Phase 5 lands

**Where:** `phase-05-wiring-swap.md` step 4 + `phase-06-data-migration.md` step 2.

**What's wrong:** `cmd/dleague-export/main.go` directly imports `internal/store/couchbase` (verified line 24 of the existing file). It does **not** go through `composed/`. Phase 5 rewires it to `mongodb.New(...)`. After Phase 5 commit lands, the binary built from HEAD can no longer talk to Couchbase. Phase 6 says "build it from the pre-Phase-5 commit (`go build -o /tmp/dleague-export-old ./server/cmd/dleague-export` on the predecessor commit)" — fine in theory, but Phase 5 also drops `couchbase/` import from `composed/` and Phase 7 deletes the package outright. The git checkout-old-commit-and-build flow works only if the developer remembers and does it before Phase 7 starts.

Worse: Phase 6 step 7 ("Cutover") expects to do the export *after* Phase 5 ships in prod, with the old running pod still up. But Coolify already redeployed the new (Mongo-backed) binary at end of Phase 5. The "still-running pre-cutover prod" door is shut.

**Fix the plan:** Re-sequence as `1 → 2 → (3,4) → 6 → 5 → 7`. Run the Couchbase→JSONL export *before* the wiring swap. Either:
- (a) move data export to a Phase 5a "snapshot before cutover" step, OR
- (b) add explicit gate: "Build & archive `dleague-export-old` binary from the current commit before merging Phase 5; verify checksum logged in Phase 6 todo." Plus: change Phase 5 step 4 from "rewire dleague-export to mongo" to "leave dleague-export pointed at couchbase until Phase 6 is done; rewire in Phase 7." This keeps Couchbase import alive in `cmd/dleague-export` while the rest of the server runs on Mongo.

Either approach must be picked explicitly. Today the plan describes both inconsistently.

### BLOCKING-2 — Phase 6 importer will create duplicate `attempts` (and matches via `_id` collision)

**Where:** `phase-06-data-migration.md` step 4 + comment "ReplaceOne with upsert on the natural _id ... or compound key (attempt: uid+puzzleDate, user: uid)".

**What's wrong:**
- Phase 3 step 4 chose: "use compound index, let Mongo generate `_id`" for `attempts`. So the Mongo `attempts` collection has Mongo-generated ObjectIDs as `_id`, and a unique compound index on `{uid, puzzleDate}`.
- Phase 6 importer description says "upserts ... on the natural _id" with attempt's compound key as a parenthetical. Vague.
- If the importer does `InsertOne` per JSONL line, every run creates fresh `_id` values; rerunning produces duplicates **until** the unique compound index rejects them with `E11000`. So "idempotent rerun" claim in non-functional reqs (Phase 6) is false unless the importer explicitly upserts on `{uid, puzzleDate}`.
- The original Couchbase JSONL has *no* `_id` field — Couchbase docs were keyed by `uid::date`. The importer must reconstruct the filter from the doc body.

**Fix the plan:** Add explicit per-collection upsert filter table in Phase 6, e.g.:
| Collection | Upsert filter |
|---|---|
| `users` | `{uid: doc.uid}` |
| `puzzles` | `{_id: doc.date}` (set `_id = doc.date` on insert) |
| `attempts` | `{uid: doc.uid, puzzleDate: doc.puzzleDate}` |
| `matches` | `{_id: doc.id}` (set `_id = doc.id` on insert) |

And replace `InsertOne / ReplaceOne` text with "ReplaceOne with the filter above + `upsert: true`."

### BLOCKING-3 — JSONL import: `time.Time` round-trips as string, not BSON Date → TTL won't apply, sort breaks

**Where:** `phase-06-data-migration.md` step 4 (`scripts/import-jsonl-to-mongo.go` description).

**What's wrong:** Couchbase Export uses `json.NewEncoder(w).Encode(...)` (verified `couchbase/export.go:39`). `time.Time` JSON-marshals to RFC3339 string. Importer reads JSONL with `json.Decoder` into `bson.M` / `map[string]any`, then writes via `ReplaceOne`. All `time.Time` fields become *strings* in BSON, not BSON Date. Consequences:
- `ListUserMatches` `.sort({createdAt: -1})` works on string lex order — RFC3339 happens to lex-sort correctly, **but only if all timestamps are UTC + same zone offset**. Mixed offsets break sort silently.
- More importantly: If beta data ever lands in `presence`/`cache`/`leaderboards` (it won't, per "Redis state is not migrated"), TTL would not fire. Fine for now since Phase 6 only imports persistent docs.
- `users.lastSeen`, `attempts.completedAt`, `matches.createdAt`, `matches.endedAt` all imported as strings. Any future code expecting `Date` BSON type breaks.

**Fix the plan:** Phase 6 importer must decode JSONL into the typed `store.User` / `Puzzle` / `Attempt` / `Match` structs (with bson tags from Phase 3) before passing to `ReplaceOne`. Add as explicit Phase 6 implementation note. Pseudocode change:
```
switch line.Collection {
case "users":  var u store.User; json.Unmarshal(line.Doc, &u); replace(&u)
case "puzzles": ... etc
}
```
Avoids `bson.M` round-trip entirely. ~150 lines instead of 100, worth it.

### HIGH-1 — `expireAfterSeconds: 0` is correct, but only if `expireAt` arrives as BSON Date

**Where:** `phase-02-mongodb-scaffold.md` line 91 + Phase 4 cache/presence ops.

**What's wrong:** TTL index on `expireAt` field with `expireAfterSeconds: 0` deletes a doc when `expireAt <= now()`. **Confirmed correct** (MongoDB docs verify). BUT: TTL works **only** when the indexed field's BSON type is Date. If a future caller writes `expireAt` as a string or int64-millis (e.g., from a different client driver, or a botched migration), the doc never expires — silently. The Go driver v2 encodes `time.Time` as Date by default, so the new code is fine. Worth adding a one-liner code comment in `presence.go` + `cache.go` near `MarkOnline`/`CacheSet`: "BSON Date required for TTL — do not pass Unix epoch ints or RFC3339 strings here."

**Fix the plan:** Add a sentence to Phase 2 step 3 `presence` + `cache` index entries: "The `expireAt` field MUST be inserted as a Go `time.Time` (driver encodes as BSON Date). Strings/ints are silently retained forever."

### HIGH-2 — `CacheSet(ttl=0)` semantic divergence vs Redis & memstore

**Where:** `phase-04-cache-port.md` step 3 (`cache.go` pseudocode) + `internal/store/redis/cache.go:30` (Redis comment: "zero TTL = no expiry") + `internal/store/memstore/memstore.go:362` (memstore: `if ttl > 0 { e.expiry = ... }`).

**What's wrong:** In Redis and memstore, `ttl=0` means "no expiry — store forever". The plan's mongo `CacheSet` does:
```go
"expireAt": time.Now().Add(ttl)
```
With `ttl=0`, this writes `expireAt = now()`. Combined with the query-side filter `expireAt: {$gt: now()}`, the doc is unreachable from `CacheGet` immediately, AND TTL purges it within 60s. Silent contract break. No callers today (verified: no `CacheSet` in `internal/api/` or `internal/wsws/`), but the contract drift will bite later.

**Fix the plan:** Phase 4 step 3 — special-case `ttl <= 0`: store with no `expireAt` field and document the divergence-or-parity choice. Add to non-functional reqs: "If `ttl <= 0`, no `expireAt` is written — doc persists indefinitely (parity with Redis SET / memstore)."

### HIGH-3 — `SubmitScore` writes `updatedAt` even when score unchanged → unbounded write churn

**Where:** `phase-04-cache-port.md` step 1 (leaderboards.go pseudocode).

**What's wrong:** Plan writes `$set: {updatedAt: time.Now()}` on every call, regardless of `$max` no-op. Redis ZADD GT does *not* write when score is lower. Implications:
- Every WS heartbeat-driven score submit writes a full doc to disk → index update on `{board, score:-1}` → index churn.
- M0 has 100 ops/sec sustained throttle. Plan's "<50 CCU" buys you ~2 ops/sec/user budget. If clients spam `SubmitScore` (e.g., on every guess attempt), you eat that quickly.
- `updatedAt` is never *read* anywhere in `store.Store` — it's a debugging field. Cost without benefit.

**Fix the plan:** Drop `$set: {updatedAt}` entirely. If you want a "last write" probe for ops, add a separate query-only field `firstSeenAt` via `$setOnInsert`. Update Phase 4 step 1 + the table at line 28.

### MEDIUM-1 — `$setOnInsert: {board, uid}` is redundant with filter

**Where:** `phase-04-cache-port.md` step 1.

**What's wrong:** When `updateOne(filter={board, uid}, update=..., upsert=true)` inserts a new doc, MongoDB **automatically** seeds the new doc with the equality fields from the filter (`board`, `uid`). The `$setOnInsert: {board: board, uid: uid}` is duplicative — not wrong, just noise. Confirmed by MongoDB docs (filter equality fields auto-set on insert during upsert).

**Fix the plan:** Remove `$setOnInsert` from `SubmitScore`. Reduce to:
```go
update := bson.M{"$max": bson.M{"score": score}}
```
Cleaner; same behavior.

### MEDIUM-2 — `MarkOnline` uses `replaceOne` not `updateOne`; documents are tiny but principle matters

**Where:** `phase-04-cache-port.md` step 2.

**What's wrong:** `replaceOne({_id:uid}, {expireAt})` replaces the entire doc. Today the doc has only `_id` + `expireAt` so it's harmless. If anyone later adds a field to presence (e.g. `lastIP`, `clientVersion` for ops debugging), this silently drops it on every heartbeat. Footgun.

**Fix the plan:** Change to `updateOne(filter={_id:uid}, update={$set: {expireAt: now+ttl}}, upsert=true)`. Same wire cost; future-proof.

### MEDIUM-3 — Concurrent presence updates: not tested

**Where:** `phase-04-cache-port.md` "Tests" list.

**What's wrong:** Two WS connections from the same UID (e.g., mobile + web) heartbeat concurrently. Both call `MarkOnline(uid, 30s)` ~within ms of each other. With `replaceOne` + `upsert`, Mongo serializes per-doc — last write wins. Behaviorally OK but no test covers it. Ditto `IsOnline` racing with the TTL purge thread.

**Fix the plan:** Add to Phase 4 tests:
- `TestMarkOnline_ConcurrentSameUID` — 10 goroutines call `MarkOnline` with mixed TTLs; assert final `expireAt` is within bounds and `IsOnline` returns true.

### MEDIUM-4 — No "Atlas unreachable" / network-failure test in any phase

**Where:** Phase 5 success criteria.

**What's wrong:** Acceptance criteria all assume Atlas works. None say what happens when Atlas TLS handshake fails mid-request. Mongo Go driver default `serverSelectionTimeout` is 30s — that's a 30-second hung HTTP request before the client gets anything. WS hub will queue heartbeats during the stall.

**Fix the plan:** Phase 5 success criteria add: "When Atlas is unreachable (test by setting `MONGODB_URI` to a black-hole IP), `/health` returns 503 within 5s, not 30s. Configure `options.Client().SetServerSelectionTimeout(5 * time.Second)`."

### LOW-1 — `0.0.0.0/0` allowlist trade-off acknowledgment is thin

**Where:** `phase-01-atlas-provisioning.md` step 5.

What's there is fine for beta with SCRAM. Worth one extra line: "Re-evaluate before any non-beta launch; static-IP NAT or VPC peering required for prod." The plan's risk-row (line 79) covers it briefly; phase file should match.

### LOW-2 — Effort estimates: Phase 3 (1.5d) is optimistic

**Where:** plan.md phases table.

10 methods + 5 tests + integration test setup + first-time mongo driver v2 wiring + bson tag debugging. Realistic 2-2.5d for solo dev. Not blocking; flag for visibility.

### LOW-3 — `Indexes().CreateMany` partial-failure semantics

**Where:** `phase-02-mongodb-scaffold.md` risk row.

Minor: docs note that on a malformed index spec, all *prior* indexes in the array are created and the bad one returns the error. So "return early on first error" is necessary but doesn't roll back already-created indexes. That's fine (re-running CreateMany no-ops on existing indexes), just call it out.

## Things the plan got right (worth confirming)

- `$max` on missing field correctly creates field with new value (verified MongoDB docs). Phase 4 SubmitScore atomicity claim holds.
- `mongo.Connect(options...)` with no context arg is correct for driver v2 (confirmed against current docs). Plan's Phase 2 pseudocode matches.
- BSON struct tags vs Couchbase: gocb uses JSON tags via its default JSON transcoder; **adding** `bson` tags (Phase 3 step 1) does not affect Couchbase serialization. JSON tags remain authoritative for gocb. No regression.
- `$gt: now()` query-side filter is the correct pattern for masking TTL purge lag. Documented in MongoDB community for years.
- Pre-Phase-5 additive landing (couchbase + redis + new mongodb all coexist) is correct and gives clean rollback.
- `var _ store.Store = (*Client)(nil)` compile-time assertion in Phase 5 is the right move.

## Things to verify in implementation (not the plan's fault)

- Atlas M0 connection limit is **500** total connections per cluster, **not per pool**. Mongo Go driver default pool is 100. Single instance is fine; if Coolify ever scales to 2+ replicas, watch `currentOp.connections`. Plan's risk row notes this but acceptance criteria don't include a pool-size check.
- `time.Time` zero-value: zero `time.Time` BSON-encodes to `0001-01-01`. If any `Match.EndedAt` defaults to zero (active match), Mongo stores it as a real Date. Sort order on `endedAt` for ended-match queries works fine; for active matches it's the year 1. Behavior parity with Couchbase confirmed (Couchbase JSON also stores zero time).

## Unresolved questions

1. Phase 6 sequencing — pick one of (a) re-sequence to `→6→5→7`, or (b) keep order but defer the `cmd/dleague-export` rewire to Phase 7. Which? (Affects BLOCKING-1.)
2. Should we add an `internal/store/mongodb/` integration-test helper that drops all `dleague_test*` collections at startup, to keep CI flake-free across test reruns? Phase 3 step 8 says "defer Drop" per test — good, but a TestMain cleanup is more robust.
3. M0 `serverSelectionTimeout` default of 30s — is 5s the right target, or do we want 2s? Beta UX vs Atlas-cold-start trade.
4. `topCap` removal in Phase 4 — confirm no offline analytics depends on bounded `leaderboards` doc count. Today there's no such consumer; if `cmd/dleague-export` ever exports leaderboards, an unbounded collection grows linearly.
5. `MONGODB_TEST_URI` lifecycle — is the test cluster the same M0 as prod (different DB name `dleague_test`), or a separate M0? Phase 1/2 imply same cluster. Acceptable for beta but worth being explicit.

Sources:
- [$setOnInsert MongoDB docs](https://www.mongodb.com/docs/manual/reference/operator/update/setoninsert/)
- [$max MongoDB docs](https://www.mongodb.com/docs/manual/reference/operator/update/max/)
- [TTL indexes — must be Date type](https://www.mongodb.com/docs/manual/core/index-ttl/)
- [Driver v2 Connect signature](https://www.mongodb.com/docs/drivers/go/current/quick-start/)
