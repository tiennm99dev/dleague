# Migration Readiness

Dleague is on a deliberately short-term self-hosted stack (Couchbase Community
8.0 + Redis 8.4 on a single VM). This doc captures the seams that keep the
swap to managed services cheap (~1 week, not a rewrite).

## Migration seam: `store.Store`

**File:** `server/internal/store/store.go`.

Every other server package — HTTP handlers, the WS hub, the auth middleware,
the export CLI — depends on the `Store` interface only. No package outside
`internal/store/` may import `gocb` or `go-redis`.

Verifiable with grep:

```sh
# Couchbase isolation
grep -r '"github.com/couchbase/gocb"' server/ | grep -v internal/store/couchbase

# Redis isolation
grep -r '"github.com/redis/go-redis"' server/ | grep -v internal/store/redis
```

Both queries should return zero hits.

## Concrete impls

```
internal/store/
├── store.go         # Store interface
├── errors.go        # Sentinel errors (ErrNotFound, ErrClosed)
├── couchbase/       # gocb v2 — bucket "dleague", collections users/puzzles/matches/attempts
├── redis/           # go-redis v9 — leaderboards (ZSETs), presence (SET+TTL), generic cache
├── composed/        # Wires couchbase + redis behind Store
└── memstore/        # In-memory; tests + dev + proof of seam
```

Three impls = a real seam. If only one ever existed, it would be a leaky
abstraction. The `memstore` runs the same upper-layer test suite the
composed impl does.

## Doc-shape constraints

- **Couchbase docs are flat JSON**. No deep nesting, no Couchbase-specific
  features (no SDK-side aggregations, no stored procedures, no XATTRs).
- **Redis usage is plain commands only** — `ZADD`, `ZRANGEBYSCORE`,
  `SET … EX`, `EXISTS`. No Lua scripts.
- **Keys are predictable**:
  - Users: `<uid>` in `users` collection.
  - Puzzles: `<YYYY-MM-DD>` in `puzzles` collection.
  - Attempts: `<uid>::<YYYY-MM-DD>` in `attempts` collection.
  - Matches: `<match-id>` in `matches` collection.
  - Leaderboards (Redis): `lb:daily:<date>`, `lb:global:alltime`.
  - Presence (Redis): `presence:<uid>`.

## Migration export CLI

**File:** `server/cmd/dleague-export/main.go`.

Wraps `store.Export(ctx, w)` from the active store impl. Emits one JSON object
per line; each line carries `{collection, doc}`. Runs with the same env vars
as the server.

```sh
go run ./cmd/dleague-export > snapshot-$(date +%F).jsonl
```

Redis state is **not** exported — leaderboards rebuild from `attempts`
post-import.

## Target-backend swap recipe

Hypothetical: migrating from Couchbase + Redis to (e.g.) MongoDB Atlas.

1. **Implement** `internal/store/mongodb/Client` to satisfy the persistent
   half of `Store` (UpsertUser, GetPuzzle, …, Export).
2. **Implement** `internal/store/mongodb-cache/` (or reuse existing Redis if
   keeping Redis) for the cache half (SubmitScore, TopN, MarkOnline, etc.).
3. **Compose** the new pair in `internal/store/composed/` (or replace the
   composed wiring entirely; trivial change).
4. **Wire** in `cmd/api/main.go`: change one constructor call.
5. **Migrate data**: run `dleague-export | mongoimport` (or a small
   per-line transformer). Redis leaderboards rebuild from imported
   attempts at first request via cache-miss path.
6. **Verify**: `memstore`'s test suite already validates the surface; run
   it against the new composed impl.

The grep test in step 0 ensures no consumer accidentally imported
`gocb`/`go-redis` — if it did, the swap would fan out to N files.

## Known migration costs

| Cost | Mitigation |
|------|-----------|
| Test fixtures specific to Couchbase N1QL | Keep N1QL queries inside `couchbase/`; tests use memstore at upper layer |
| Redis ZADD GT semantics → not all backends have | Document SubmitScore semantics ("higher score wins"); equivalent in MongoDB via $max |
| Index recreation on target backend | Document expected indexes per collection (currently: primary index only on each Couchbase collection) |
| Coolify env-var → managed-service connection string | Adjust `internal/config` schema; same `Store` interface stays |

## Beta posture & data loss

The migration export is the **escape hatch** for the data-loss-acceptable
beta posture. If the VM disk fails before a managed-service migration is
ready, beta data is gone — but as long as the export CLI ran on a healthy
VM, the export file is the canonical seed for a future store.

## License watchout

Couchbase Community Edition is governed by the [Couchbase CE License
Agreement](https://www.couchbase.com/community-license-agreement/). Per
primary-source review (full report:
`plans/reports/researcher-260506-1029-couchbase-ce-license-verbatim.md`),
the license **explicitly permits** building commercial products on top
of CE:

> *"Couchbase grants you a non-exclusive, non-transferable, …license to
> install and use the Community Software at no charge for your internal
> business purposes and to develop or commercialize products that
> interact with the Community Software, subject to the restrictions in
> Section 5..."*

There is **no production-use restriction** and **no "Productive Use"
auto-conversion** clause in the CE License. (That language exists in
Couchbase's *Enterprise* subscription agreements — a separate document.)

### Hard restrictions (Section 5)

| Limit | Effect on dleague |
|---|---|
| **≤ 5 nodes** per Couchbase cluster | ✓ We run 1 node on the Coolify VM. Headroom is large. |
| **≤ 4 cores per node** | ✓ Single-VM deploy comfortably fits. |
| **No XDCR** (cross-datacenter replication) | ✓ Single-region beta does not need XDCR. |
| **No use to "support a workload also supported by any commercial Couchbase offering"** | ✓ dleague is a game, not a Couchbase competitor. Practical reading: don't build DBaaS on CE. |

If we ever exceed any of those, the move is to **Couchbase Capella**
(managed, paid) or buy an Enterprise license — same `gocb` SDK, ~1-day
cutover.

### Capella migration (operational, not legal)

Capella is no longer required for license reasons. It remains an
**operational** option if we want managed backups, multi-AZ, or vendor
support. Free tier: 1 cluster, ~8 GB storage, auto-pauses on inactivity.

### What we are NOT going to do

- We are **not** migrating to MongoDB / FerretDB / Postgres for license
  reasons. The CE license fits the project; alternative DBs would only
  make sense if we hit the 5-node / 4-core cap or wanted a different
  data model — neither is in scope.

### Review log

| Date | Reviewer | Outcome |
|------|----------|---------|
| 2026-05-06 (a.m.) | researcher subagent #1 (`researcher-260506-0939-couchbase-ce-license-fit.md`) | **Superseded.** Conflated BSL source-license terms with CE binary license; introduced an over-cautious reward-shape table that was wrong. |
| 2026-05-06 (mid-day) | researcher subagent #2 (`researcher-260506-1029-couchbase-ce-license-verbatim.md`) | **In force.** CE License grants commercial use directly. Hard caps are 5 nodes / 4 cores per node / no XDCR / no DBaaS-style competitive use. dleague is fully compliant for beta and for monetized production within those caps. |
