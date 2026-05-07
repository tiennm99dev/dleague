# Migration Readiness

Dleague's data plane is **MongoDB Atlas** (M0 free tier during beta). The
seam (`store.Store` Go interface) was designed for cheap backend swaps and
this doc captures both the current state and the recipe for the next swap
(if one is ever needed).

## Migration seam: `store.Store`

**File:** `server/internal/store/store.go`.

Every other server package — HTTP handlers, the WS hub, the auth middleware,
the export call — depends on the `Store` interface only. No package outside
`internal/store/mongodb/` may import `go.mongodb.org/mongo-driver/v2/...`.

Verifiable with `make grep-isolation`:

```sh
grep -rl '"go.mongodb.org/mongo-driver/v2' server/ \
  | grep -v 'internal/store/mongodb'
```

Should return zero hits.

## Concrete impls

```
internal/store/
├── store.go         # Store interface + entity types (json + bson tags)
├── errors.go        # Sentinel errors (ErrNotFound, ErrClosed)
├── mongodb/         # mongo-driver/v2 — collections users/puzzles/attempts/
│                    # matches/leaderboards/presence/cache
└── memstore/        # In-memory; tests + dev + proof of seam
```

Two impls (`mongodb` + `memstore`) is what proves the seam holds — if only
one ever existed it would be a leaky abstraction. The `memstore` runs the
same upper-layer test suite the `mongodb` impl does.

## Doc-shape constraints

- **MongoDB documents are flat JSON-shaped** (matching the `store.User` /
  `Puzzle` / `Attempt` / `Match` Go structs). No `$lookup`, no aggregation
  pipelines, no driver-side transforms beyond `$max` and TTL indexes.
- **Atomicity primitives we use:** `$max` for "score-only-on-higher"; TTL
  index + query-side `expireAt > now()` filter for accurate presence/cache
  liveness; `$setOnInsert` to freeze first-auth fields.
- **Indexes** (created at startup by `mongodb.ensureIndexes`):
  | Collection | Index |
  |---|---|
  | `users` | `{uid: 1}` unique |
  | `puzzles` | `_id` (date string) |
  | `attempts` | `{uid: 1, puzzleDate: 1}` unique |
  | `matches` | `_id` (match ID) + `{players: 1, createdAt: -1}` |
  | `leaderboards` | `{board: 1, score: -1}` + `{board: 1, uid: 1}` unique |
  | `presence` | TTL on `expireAt`, `expireAfterSeconds: 0` |
  | `cache` | TTL on `expireAt`, `expireAfterSeconds: 0` |

## Migration export

`(store.Store).Export(ctx, w)` streams every persistent doc as JSONL. One
line per doc: `{"collection": "<name>", "doc": {...}}`. The format is impl-
agnostic — the same line shape worked on the prior Couchbase backend and
works against MongoDB now.

There is no longer a dedicated `cmd/dleague-export` binary; for outbound
migration use Atlas's native `mongodump`:

```sh
mongodump --uri "$MONGODB_URI" --db dleague --out ./snapshot/
```

Restoring elsewhere is `mongorestore` against any MongoDB-compatible target
(self-hosted Mongo, FerretDB, DocumentDB, etc.). The `Export` interface
method remains in `store.Store` because it's the seam-friendly path that
any future non-Mongo impl will also implement.

## Outbound recipe (if/when we leave Atlas)

Hypothetical: migrating from Atlas to (e.g.) Couchbase Capella, FerretDB, or
self-hosted MongoDB.

1. **Implement** `internal/store/<target>/Client` to satisfy `store.Store`.
2. **Wire** in `cmd/api/main.go`: change one constructor call from
   `mongodb.New(...)` to `<target>.New(...)`.
3. **Migrate data:** `mongodump` from Atlas, `mongorestore` to target (or a
   small per-collection transformer if the target speaks a different wire
   format).
4. **Verify:** the same upper-layer test suite that runs against `memstore`
   already validates the surface; run it against the new impl with the
   integration env-var pattern (e.g. `FERRETDB_TEST_URI`).

The `make grep-isolation` check ensures no consumer accidentally imported
the new driver outside its package — if it did, the swap would fan out to
N files.

## Known migration costs

| Cost | Mitigation |
|------|-----------|
| Test fixtures specific to `MONGODB_TEST_URI` | Tests skip cleanly when env var is unset; memstore covers the seam at the upper layer |
| Atlas-only features (`$max` atomic upserts, TTL indexes) | All standard MongoDB features — portable to any MongoDB-compatible target. If targeting a non-Mongo store, document equivalents at port time |
| Index recreation on target backend | Index list above is the canonical reference; `ensureIndexes` does the work at startup |
| Coolify env var → managed-service connection string | Adjust `internal/config` schema; same `Store` interface stays |

## Beta posture & data loss

The `/health` endpoint pings Atlas; sustained failure surfaces in monitoring.
The beta posture explicitly accepts data loss — if the M0 cluster is wiped,
beta data is gone. `mongodump` is the escape hatch when we want a snapshot.

## License watchout

MongoDB Community Edition ships under the **Server Side Public License
(SSPL)**. We do **not** use Community on a self-hosted node — we use
**MongoDB Atlas (managed)**, governed by the Atlas commercial terms. SSPL
restrictions on offering-Mongo-as-a-service do not apply to dleague (we are
a game, not a Mongo-as-a-service vendor).

If we ever consider self-hosting Mongo in the future:
- Vanilla Mongo CE on our own infra → SSPL applies; review the "offering it
  as a service" clause carefully.
- FerretDB → Apache-2.0; drop-in wire-compatible alternative without SSPL.

For now: Atlas → no SSPL exposure.

## Predecessor (historical)

Before 2026-05-07 the data plane was self-hosted **Couchbase Community 8.0
+ Redis 8.4** behind the same `store.Store` seam. The migration to Atlas
exercised the seam exactly once. See
`plans/260507-1648-mongodb-atlas-only-migration/plan.md` for the consolidation
decision and `plans/260505-1604-firebase-couchbase-redis-pivot/` for the
predecessor stack's full design + license review.
