---
title: "MongoDB Atlas only — drop Couchbase + Redis"
description: "Consolidate the data plane onto MongoDB Atlas (M0 free tier for beta). Replaces self-hosted Couchbase Community 8.0 + Redis 8.4 with a single managed backend. Documents (users/puzzles/attempts/matches) move to Mongo collections; leaderboards via `$max` + compound index; presence + cache via TTL indexes with query-side `expireAt > now()` accuracy filter. Migration uses the existing `store.Store` seam — one new `internal/store/mongodb/` impl, delete `couchbase/` + `redis/` + `composed/`. Beta posture preserved."
status: completed
priority: P1
effort: 5-6d
branch: main
tags: [mongodb, mongodb-atlas, managed-services, store-swap, post-pivot, consolidation, beta]
created: 2026-05-07
parent_plan: 260505-1604-firebase-couchbase-redis-pivot/plan.md
supersedes: []
research:
  - reports/researcher-260507-1648-mongodb-atlas-tiers-and-limits.md
  - reports/researcher-260507-1650-mongodb-replaces-redis.md
  - reports/brainstorm-260507-1648-mongodb-only-tradeoffs.md
blockedBy: [260505-1604-firebase-couchbase-redis-pivot]
---

# MongoDB Atlas only — drop Couchbase + Redis

## Goal

Replace **both** self-hosted Couchbase Community 8.0 (primary store) **and** Redis 8.4 (cache + leaderboards + presence) with a **single MongoDB Atlas** cluster. Use the M0 free tier during beta (peak <50 concurrent WS clients, well under 100-conn cap). Preserve the `store.Store` interface seam — only one new impl ships; old impls deleted.

## Why now (decision context)

- User pick after reviewing brainstormer's status-quo recommendation (75% confidence).
- Operational pull: prefer one managed backend over two self-hosted containers on the OCI VM.
- Technical fit confirmed by research: `$max` provides atomic ZADD-GT equivalent; TTL indexes with query-side `expireAt > now()` filter give accurate presence/cache liveness; M0 free tier covers beta scale.
- License panic on Couchbase CE was already resolved 2026-05-06 — this migration is **not** driven by license fear; the seam was always meant to be exercised.

## Decisions locked

- **Backend:** MongoDB Atlas M0 (free tier). Region: AWS Singapore (`ap-southeast-1`) to co-locate with OCI Singapore. SCRAM-SHA-256 auth via `MONGODB_URI` env var.
- **Driver:** `go.mongodb.org/mongo-driver/v2` v2.6.x.
- **Scale assumption:** beta peak <50 concurrent WS clients. M0 (500 conn cap, 100 ops/sec sustained) holds. M10 ($57/mo) is a known upgrade path post-beta if traffic warrants.
- **Collections:** `users`, `puzzles`, `attempts`, `matches`, `leaderboards`, `presence`, `cache`. Single database `dleague`.
- **Atomicity contracts:**
  - Leaderboard score-only-on-higher: `updateOne({board, uid}, {$max: {score}}, {upsert:true})` — atomic at single-doc level.
  - Presence/cache TTL: TTL index on `expireAt` field (`expireAfterSeconds: 0`) + every read includes `expireAt: {$gt: now()}` to mask the up-to-60s background-purge lag.
- **No leaderboard trim.** Storage cost is negligible at our scale; index keeps top-N reads fast.
- **Delete `composed/`.** Single backend → no need to fan persistent vs cache halves through a wrapper. `*mongodb.Store` implements `store.Store` directly. KISS.
- **Drop `couchbase/` + `redis/` packages and their integration test gates.** Replace with `mongodb_test.go` gated by `MONGODB_TEST_URI`.
- **Data migration:** existing `cmd/dleague-export` → JSONL → small Go transformer (or `mongoimport` per collection) → Atlas cluster. One-shot script in `scripts/`.
- **Beta posture preserved.** "Beta — data may reset" banner stays; `isBetaTester` ledger semantics unchanged.

## What stays the same

- `store.Store` interface signature — unchanged. No HTTP handler, WS hub, or auth code touches.
- `memstore` impl — kept as test double + proof of seam.
- Firebase Auth — untouched (this plan does not touch auth).
- Client (Svelte 5 + Phaser 4 + Capacitor) — untouched.
- Wire format / WebSocket hub — untouched.

## Phases

| # | Phase | File | Effort | Status |
|---|-------|------|--------|--------|
| 1 | Atlas provisioning + env wiring | [phase-01-atlas-provisioning.md](phase-01-atlas-provisioning.md) | 0.5d | completed |
| 2 | `internal/store/mongodb/` scaffold (client, indexes, smoke test) | [phase-02-mongodb-scaffold.md](phase-02-mongodb-scaffold.md) | 1d | completed |
| 3 | Persistent half port (users, puzzles, attempts, matches, Export) | [phase-03-persistent-port.md](phase-03-persistent-port.md) | 2-2.5d | completed |
| 4 | Cache half port (leaderboards via `$max`, presence + cache via TTL) | [phase-04-cache-port.md](phase-04-cache-port.md) | 1d | completed |
| 5 | Wiring swap in `cmd/api/main.go`; delete `composed/` | [phase-05-wiring-swap.md](phase-05-wiring-swap.md) | 0.5d | completed |
| 6 | Data migration script (export → mongoimport) | [phase-06-data-migration.md](phase-06-data-migration.md) | 0.5d | **skipped** — beta has not deployed; Atlas starts empty (user decision 2026-05-07) |
| 7 | Cleanup + docs + supersession | [phase-07-cleanup-and-docs.md](phase-07-cleanup-and-docs.md) | 1d | completed |

## Sequencing

```
1 → 2 → (3, 4) parallel → 5 → 6 → 7
```

Phase 1 unblocks everything (need a real cluster URI for any integration test). Phase 2 lays the package shape. Phases 3 + 4 are independent ports of the persistent and cache halves (different files, no shared state). Phase 5 flips the wiring once both halves pass tests. Phase 6 migrates beta data (or skips if "beta data may reset" is exercised). Phase 7 deletes dead code and refreshes docs.

**Phase 5 sequencing nuance:** Phase 5 rewires `cmd/api/main.go` to Mongo, but **leaves `cmd/dleague-export/main.go` pointed at Couchbase** until Phase 7. This keeps the Couchbase→JSONL export door open during the Phase 6 cutover (the live API is on Mongo; the export CLI still reads the soon-to-be-decommissioned Couchbase). Phase 7 rewires (or retires in favor of `mongodump`) the export CLI as the last act before deletion.

**Phase 6 skip:** Beta has not been deployed yet — no live data exists to migrate. Atlas starts empty. This collapses sequencing to `1 → 2 → (3,4) → 5 → 7`. The Phase 5 rule (leave `cmd/dleague-export` Couchbase-backed) becomes optional — we can either retire the export CLI in Phase 7 or rewire it directly. Plan recommendation: retire in favor of `mongodump`. Phase 7's "24h prod soak" gate also collapses since there are no users to protect; deletion can run immediately after Phase 5's smoke test passes.

## Red-team review applied

This plan was reviewed by `code-reviewer` agent on 2026-05-07; report at [`plans/reports/code-reviewer-260507-1648-mongodb-plan-redteam.md`](../reports/code-reviewer-260507-1648-mongodb-plan-redteam.md). All BLOCKING + HIGH issues are addressed in the per-phase files:

- **BLOCKING-1 (Phase 6 export sequencing):** Phase 5 leaves `cmd/dleague-export` Couchbase-pointed; Phase 7 rewires it.
- **BLOCKING-2 (importer upsert filters):** Phase 6 has an explicit per-collection upsert-filter table.
- **BLOCKING-3 (`time.Time` round-trips as string):** Phase 6 importer decodes JSONL into typed entity structs before write.
- **HIGH-1 (TTL needs BSON Date):** Phase 2 + Phase 4 explicitly require `time.Time` (not strings/ints) for `expireAt`.
- **HIGH-2 (`CacheSet(ttl=0)`):** Phase 4 special-cases `ttl <= 0` to write no `expireAt` (parity with Redis/memstore).
- **HIGH-3 (`SubmitScore` write churn):** Phase 4 drops `$set: {updatedAt}` from `SubmitScore`.
- **MEDIUM (Mongo idiom cleanups):** `MarkOnline` uses `updateOne` not `replaceOne`; `$setOnInsert` removed from leaderboard upserts; concurrent-presence test added; Atlas-unreachable test + `SetServerSelectionTimeout(5s)` added in Phase 5.

## Top risks

- **M0 connection cap (500) under WS heartbeat load.** Mitigation: confirmed peak <50 CCU with user; Mongo Go driver pools connections (default 100). Alert if `mongoSession.NumberSessionsInProgress` approaches 80% of pool.
- **TTL purge lag (up to ~60s).** Mitigation: every presence/cache read includes `expireAt: {$gt: now()}` filter — accurate regardless of physical purge timing. Documented in code comments + `migration-readiness.md`.
- **Static IP allowlist friction with Coolify.** Atlas requires IP allowlist; OCI egress IP from Coolify may rotate. Mitigation: use `0.0.0.0/0` during beta (auth still SCRAM-secured) or pin Coolify outbound. Decide in Phase 1.
- **Atlas M0 auto-pause after 30 days inactivity.** Beta is active so this won't trigger; `/health` endpoint is hit by uptime monitors. If we ever idle 30d, manual resume + 30-60s cold start.
- **Leaderboard sort latency vs Redis.** ~30× slower (15ms vs 0.5ms) per top-N query. Invisible at beta scale; revisit at >1k DAU.
- **Lost Redis-class atomic primitives elsewhere.** None found in codebase audit (only ZADD GT, EX/SET — all reproducible). If a future workload needs real ZSET semantics, layer in managed Redis (Upstash) — the seam still permits it.
- **Single-VM SPOF replaced by Atlas SPOF.** Worse single-AZ availability than 3-AZ replica set; M0 is 3-node anyway (no config). Net: better than current single-VM.

## Rollback plan

1. Phase 1–4 are additive (new package alongside existing `couchbase/` + `redis/`). No prod impact until Phase 5 wiring swap.
2. Phase 5 flips one constructor call in `cmd/api/main.go`. Rollback = revert that commit.
3. Phase 6 skipped — N/A.
4. Phase 7 deletion is irreversible only via git. Without a deployed beta, the "24h soak" gate is a no-op; rollback path is `git revert` if anything regresses post-cleanup.

## Post-migration acceptance

- [ ] All current API tests pass against `mongodb` impl (running `MONGODB_TEST_URI=...`).
- [ ] `go test ./...` green.
- [ ] Leaderboard p95 read < 50ms over public Atlas TLS from OCI Singapore.
- [ ] Presence-write latency on WS heartbeat does not visibly stall WS hub (measure).
- [ ] `dleague-export | mongoimport` round-trip preserves all 4 persistent collections.
- [ ] `migration-readiness.md` rewritten — Atlas now the active backend; "future swap" section pivots to "if we need to leave Atlas".
- [ ] `system-architecture.md` diagram updated.
- [ ] `project-roadmap.md` reflects new phase status.
- [ ] Old plan `260505-1604-firebase-couchbase-redis-pivot/plan.md` gets `superseded_by: 260507-1648-mongodb-atlas-only-migration/plan.md` in frontmatter (only for the *post-pivot* state — its Phase 11/12 deploy-on-Coolify-with-Couchbase-Redis is the predecessor that this plan supersedes via consolidation).

## Unresolved questions

1. Coolify outbound IP stability — static or rotating? (Determines whether `0.0.0.0/0` or pinned allowlist; Phase 1.)
2. Backup cadence for beta — daily `mongodump` to OCI Object Store, or accept "data may reset" and skip?
3. Should `cmd/dleague-export` stay (now Mongo-flavored), or be retired in favor of `mongodump`? (Phase 7 decision.)
4. Phase 11 deploy of the *current* stack — does it ship before this plan starts, or do we skip Phase 11 entirely and deploy directly with Mongo? (Recommend: skip Phase 11; this plan delivers the deployable target.)
