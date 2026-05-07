---
phase: 6
title: "Data migration (export → mongoimport)"
status: skipped
priority: P2
effort: "0.5d"
dependencies: [5]
---

# Phase 6: Data migration (export → mongoimport)

## Context links

- Existing export tool: [`server/cmd/dleague-export/main.go`](../../server/cmd/dleague-export/main.go).
- Beta posture: data may reset; this phase is *optional* if no beta data is in flight on Couchbase.

## Overview

Move beta data from the running Couchbase deployment to Atlas. This phase is optional — under the documented beta posture, "VM disk failure or `docker compose down -v` is acceptable data loss". If no real beta users exist yet (beta hasn't launched), skip Phase 6 entirely and let Atlas start empty. If beta data does exist and we want to preserve it, run a one-shot script.

**Sequencing note (post Phase 5):** Phase 5 left `cmd/dleague-export` pointed at Couchbase and only switched the live API to Mongo. So at the start of Phase 6 we have: API running on Atlas (mostly empty), Couchbase still running on the same VM (now read-only to clients but data intact), and `dleague-export` still able to read Couchbase. Phase 6 runs the export against Couchbase and imports into Atlas.

## Requirements

**Functional:**
- `dleague-export` is *still* Couchbase-backed at this point (Phase 5 left it that way for exactly this purpose). Run it directly against the running Couchbase to produce JSONL.
- Output of `dleague-export` is JSONL: one line per doc, shape `{collection: "...", doc: {...}}`.
- A small Go transformer (`scripts/import-jsonl-to-mongo.go`) reads that JSONL and upserts each doc into the matching Mongo collection.
- **Importer must decode JSONL into the typed `store.User` / `Puzzle` / `Attempt` / `Match` structs** (which now have `bson` tags from Phase 3) before passing to the driver. Going through `bson.M` / `map[string]any` would round-trip `time.Time` fields as RFC3339 strings instead of BSON Date — breaking sort order, breaking any future TTL on those fields, and silently producing incorrect data. **Non-negotiable.**
- **Per-collection upsert filter table** (importer dispatches on `collection` field of each JSONL line):
  | Collection | Upsert filter | `_id` strategy |
  |---|---|---|
  | `users` | `{uid: doc.uid}` | Mongo-generated `_id`; uniqueness via `{uid:1}` unique index |
  | `puzzles` | `{_id: doc.date}` | Set `_id = doc.date` on insert |
  | `attempts` | `{uid: doc.uid, puzzleDate: doc.puzzleDate}` | Mongo-generated `_id`; uniqueness via `{uid:1, puzzleDate:1}` unique index |
  | `matches` | `{_id: doc.id}` | Set `_id = doc.id` on insert |
  Use `ReplaceOne(filter, doc, options.Replace().SetUpsert(true))` for each. `InsertOne` is forbidden — would create duplicate `attempts` on rerun.
- Redis state (leaderboards, presence, cache) is **not** migrated. It rebuilds from `attempts` on first request via the cache-miss path and live WS heartbeats.

**Non-functional:**
- Idempotent: rerunning the import script produces the same Mongo state (uses upsert on `_id`).
- One pass per collection in this order: `users`, `puzzles`, `attempts`, `matches`. Order doesn't matter for correctness but makes progress visible.

## Architecture

```
[Old Couchbase prod]
         │
         │ (run while old binary still deployed, before Phase 5 cutover)
         ▼
   dleague-export > snapshot.jsonl
         │
         ▼
   scripts/import-jsonl-to-mongo.go
         │
         ▼
   [Atlas dleague DB]
```

## Related code files

- **Create:** `scripts/import-jsonl-to-mongo.go` — reads JSONL from stdin, dispatches by `collection` field, upserts to Atlas.
- **Modify:** `docs/migration-readiness.md` — replace "future swap to Atlas" recipe with "this is how we did it"; the old recipe stays as the symmetric outbound recipe.

## Implementation steps

1. **Decide:** does beta data need preserving?
   - If no real users yet → skip Phase 6 entirely. Document the decision in plan.md.
   - If yes → continue with steps 2–7.
2. **Run the export against running Couchbase** (no binary pinning needed — Phase 5 left `dleague-export` Couchbase-backed):
   ```sh
   COUCHBASE_CONN=... go run ./server/cmd/dleague-export > snapshot-$(date +%F).jsonl
   ```
   Capture stderr to `snapshot.log`. Verify line counts (`wc -l` per collection grep).
3. **Write the importer** at `scripts/import-jsonl-to-mongo.go`. Decode each JSONL line into a typed struct based on the `collection` field, then `ReplaceOne` with the per-collection filter from the table above:
   ```go
   // Single file, ~150 lines. No abstractions. Reads stdin, writes Atlas.
   type line struct {
       Collection string          `json:"collection"`
       Doc        json.RawMessage `json:"doc"`
   }
   for scanner.Scan() {
       var l line
       _ = json.Unmarshal(scanner.Bytes(), &l)
       switch l.Collection {
       case "users":
           var u store.User; _ = json.Unmarshal(l.Doc, &u)
           db.Collection("users").ReplaceOne(ctx, bson.M{"uid": u.UID}, u, upsert)
       case "puzzles":
           var p store.Puzzle; _ = json.Unmarshal(l.Doc, &p)
           db.Collection("puzzles").ReplaceOne(ctx, bson.M{"_id": p.Date}, p, upsert)
       case "attempts":
           var a store.Attempt; _ = json.Unmarshal(l.Doc, &a)
           db.Collection("attempts").ReplaceOne(ctx,
               bson.M{"uid": a.UID, "puzzleDate": a.PuzzleDate}, a, upsert)
       case "matches":
           var m store.Match; _ = json.Unmarshal(l.Doc, &m)
           db.Collection("matches").ReplaceOne(ctx, bson.M{"_id": m.ID}, m, upsert)
       }
   }
   ```
   The typed-struct decode is the load-bearing detail — it makes `time.Time` round-trip as BSON Date, not string.
4. **Dry-run** on a *test* Atlas DB (`dleague_test_import`) first:
   ```sh
   MONGODB_URI=$ATLAS_TEST_URI go run ./scripts/import-jsonl-to-mongo.go < snapshot.jsonl
   ```
5. **Spot-check counts:** for each collection, run `db.users.countDocuments({})` etc. on Atlas; compare against `wc -l` of the JSONL slice for that collection.
6. **Spot-check types:** in Atlas, `db.users.findOne({}, {lastSeen:1})` should show `lastSeen` as an ISODate (`{"$date": "..."}`), not a string. If it's a string, the importer is going through `bson.M` somewhere — fix before proceeding.
7. **Cutover sequence** (already partially done in Phase 5; this is the data part):
   - At this point: API is running on Mongo (Phase 5 done). Atlas is mostly empty.
   - Put the API into a brief maintenance mode (or accept that any writes during the import window land only in Atlas, since the API no longer writes to Couchbase).
   - Run `dleague-export` on the still-running Couchbase → JSONL.
   - Run the importer against prod Atlas.
   - Lift maintenance.
   - Smoke-test (sign in, fetch today's puzzle, submit attempt).
   - Total user-visible downtime target: <5 minutes (or zero if "post-Phase-5 writes go straight to Atlas; only reads of historical data are stale until import lands").
8. **Delete the snapshot file** from local disk after Atlas has 24h of healthy production traffic. Or move to OCI Object Store as a one-time backup.

## Todo list

- [ ] Decision recorded: migrate beta data, or accept reset?
- [ ] (If migrate) `scripts/import-jsonl-to-mongo.go` written with typed-struct decoding (no `bson.M`)
- [ ] (If migrate) Per-collection upsert filters match the table above
- [ ] Dry-run on `dleague_test_import` succeeds; counts match
- [ ] Type spot-check: dates are ISODate in Atlas, not strings
- [ ] Cutover plan rehearsed (downtime window communicated to beta testers)
- [ ] Production cutover executed
- [ ] Atlas counts verified post-cutover
- [ ] Snapshot file archived to OCI Object Store (or deleted)

## Success criteria

- Atlas `dleague` DB contains the same number of docs per collection as the source JSONL slice.
- A spot-checked beta user can sign in post-cutover and see their pre-cutover attempt history.
- Leaderboards repopulate within minutes of users hitting the API (the cache-miss path runs `SubmitScore` based on existing `Attempt` docs).

## Risk assessment

- **Lost data during cutover.** Mitigation: snapshot, dry-run, downtime window rehearsed. Beta posture explicitly accepts loss; this is "best-effort preservation".
- **Forgetting that Phase 5 keeps `dleague-export` on Couchbase.** Mitigation: the rule is documented in plan.md and Phase 5; without it, this phase has no way to read Couchbase data. Phase 7 has explicit "rewire dleague-export" step as the closing move.
- **Importer goes through `bson.M` and serializes timestamps as strings.** Highest-leverage silent failure. Mitigation: typed-struct decoding is mandatory (step 3 of Implementation), spot-check in step 6 verifies BSON Date type post-import, dry-run catches it before prod.
- **Duplicate `attempts` from `InsertOne`.** Mitigation: the per-collection table above explicitly forbids `InsertOne` and pins the upsert filter for each collection. Unique compound index `{uid, puzzleDate}` is the second line of defense (rejects duplicates with `E11000`).
- **Importer race with live writes during cutover.** Mitigation: pause writes (or accept that Phase-5-onwards writes already go to Atlas — Couchbase is a frozen snapshot at that point, so no race on the source side).

## Security considerations

- The JSONL snapshot contains user emails + UIDs. Treat as PII: encrypt at rest if archived, delete after the validation window.
- `MONGODB_URI` for the importer is the same prod URI; never log.

## Next steps

Phase 7: delete dead code (couchbase/, redis/, docker services), update docs, archive the old plan.
