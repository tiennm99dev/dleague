---
type: research
title: "NoSQL candidates for dleague — does any beat CockroachDB Basic?"
date: 2026-05-05
session: 260505-1207
sources_consulted: 5 web searches 2026-05-05
project: dleague
---

# NoSQL candidates for dleague

> Companion to the SQL DB research thread. Question: would MongoDB Atlas / DynamoDB / Firestore / PocketBase beat CockroachDB Basic for dleague? Short answer: **no.**

## TL;DR — verdict

**Stick with CockroachDB Basic.** The closest NoSQL contender is **AWS DynamoDB Always-Free**, and even it loses on schema-fit ergonomic cost. dleague's data is structurally relational (users → sessions → matches → attempts → leaderboards by aggregation). NoSQL flexibility is a feature we wouldn't use; storage caps and access-pattern design overhead are taxes we'd pay for nothing.

If you want a **single dissenting case** for switching: **PocketBase co-hosted on the OCI VM** — eliminates network latency entirely, replaces our auth design, never depends on a free tier. It is genuinely interesting, but only if we'd kept Phase 3 auth as a Phase-3 deliverable instead of having already committed to our own protobuf-over-WS protocol. As of today (Phase 1 shipped), PocketBase is a 2-week pivot for unclear gain.

## Executive summary

| Candidate | Free storage | Ops/quota | Backups free | Killer issue for dleague |
|---|---|---|---|---|
| **CockroachDB Basic (baseline)** | **10 GiB** | $15/mo credit ≈ 50M RUs | ✓ Daily | None worth switching from |
| MongoDB Atlas M0 | **512 MB** ⚠ | 100 ops/sec throttle | ❌ | 20× less storage, no backups |
| AWS DynamoDB Always-Free | **25 GB** | 25 RCU/WCU (~2.1M reads + 2.1M writes/day) | ❌ (paid PITR) | Access-pattern design tax for relational data |
| Firestore Spark | 1 GiB | **50K reads/day, 20K writes/day hard cut** ⚠ | ❌ | Daily cap will hard-stop the game once popular |
| PocketBase (self-host) | "as much as VM" | bounded by OCI VM | self-managed | Pre-1.0, mismatches our own auth protocol |

Sources: [MongoDB Atlas Free Cluster Limits](https://www.mongodb.com/docs/atlas/reference/free-shared-limitations/), [DynamoDB pricing — Always-Free](https://aws.amazon.com/dynamodb/pricing/provisioned/), [Firestore Spark pricing](https://cloud.google.com/firestore/pricing), [PocketBase FAQ](https://pocketbase.io/faq/).

## Schema-shape fit for dleague

Phase 3–5 plan defines:

```
users(id uuid pk, email citext unique, password_hash, display_name, created_at)
sessions(token, user_id fk → users, expires_at)
puzzles(date pk, seed, answer_hash)
matches(id uuid pk, kind enum[async,sync], creator_id, joiner_id, puzzle_date, status, created_at)
attempts(id uuid pk, match_id fk → matches, user_id fk → users, guess, attempts_used, duration_ms, won)
leaderboards = SELECT user_id, count(won) FROM attempts WHERE puzzle_date = ? GROUP BY user_id ORDER BY ...
```

This is **textbook relational**. Foreign keys, joins, aggregations. NoSQL "wins" come from one of three places:

1. **Schema flexibility** — adding fields without migration. We won't do this for years; auth schema is locked.
2. **Horizontal scale** — sharding for high-write workloads. We'll have <100 users for year 1; this is a non-issue.
3. **Single-document atomic ops** — true win for things like counters. dleague has counters (wins per user) but they're computed from `attempts`, not stored.

For #3, NoSQL does NOT actually beat Cockroach: SELECT-FOR-UPDATE + UPDATE in a transaction is one round-trip in pgx and exactly as fast as a Mongo `findOneAndUpdate`.

**Brutal honesty:** swapping to NoSQL for dleague is recreational architecture, not engineering.

## Match-join race condition — per-candidate

Phase 4's central concurrency case: two players hit "join match $TOKEN" simultaneously, exactly one must win.

| Candidate | Pattern | LOC | Verdict |
|---|---|---|---|
| **CockroachDB** | `BEGIN; SELECT ... FOR UPDATE; UPDATE ... WHERE joiner_id IS NULL; COMMIT;` wrapped in `crdbpgx.ExecuteTx` | ~15 | Clean |
| MongoDB | `db.matches.findOneAndUpdate({_id, joiner_id: null}, {$set: {joiner_id}})` — atomic single-document, no txn needed | ~5 | **Cleanest** |
| DynamoDB | `UpdateItem` with `ConditionExpression: "attribute_not_exists(joiner_id)"` | ~10 | Clean, native pattern |
| Firestore | `RunTransaction` with optimistic retry | ~20 | Verbose |
| PocketBase | SQLite `BEGIN IMMEDIATE; UPDATE matches SET joiner = ? WHERE id = ? AND joiner IS NULL; COMMIT;` | ~10 | Clean |

MongoDB's `findOneAndUpdate` is genuinely elegant for this single case. But this case is one of dozens; the rest of the codebase eats Mongo's storage cap.

## Leaderboard fit at dleague scale

Top 100 by wins desc, then attempts asc:

| Candidate | Pattern | Cost at <100 users |
|---|---|---|
| Cockroach | Compound index on `(puzzle_date, wins desc, attempts asc)` + LIMIT 100 | trivial |
| MongoDB | Compound index, `find().sort().limit(100)` | trivial |
| DynamoDB | Pre-aggregated table with GSI on (puzzle_date, wins desc) — must design upfront | **Real engineering time** |
| Firestore | Compound index, `.orderBy().limit(100)` — counts each result against 50K daily reads | trivial today, risky later |
| PocketBase | SQLite SELECT, same as Cockroach | trivial |

DynamoDB's leaderboard needs upfront GSI design + a separate aggregate table. Time cost: ~half a day. Not catastrophic, but real.

## Free-tier risk profile (2024–2026 signal)

- **CockroachDB:** Dec 2024 rebrand kept $15 credit unchanged. Direction = paid tiers, not free retraction. **Low risk.**
- **MongoDB Atlas M0:** Storage at 512 MB has been steady for years. M0 is core acquisition funnel for them; unlikely to vanish. **Low risk.** But you can only have 1 M0 cluster per project.
- **DynamoDB Always-Free:** AWS Always-Free explicitly never expires. Most stable free guarantee in this list. **Lowest risk.**
- **Firestore Spark:** Stable for years. Daily-quota model means failure mode is "stop accepting traffic," not "delete cluster." **Low risk on existence, high risk on operability.**
- **PocketBase:** Pre-1.0, breaking changes between versions. Single maintainer. **Medium-high project-risk.** Free hosting risk is whatever your VM costs.

## Region coverage vs OCI Always-Free

OCI free regions: Phoenix, Ashburn, Frankfurt, Amsterdam, Tokyo, Osaka.

| Candidate | OCI Phoenix | OCI Ashburn | OCI Frankfurt | OCI Tokyo |
|---|---|---|---|---|
| Cockroach Basic | ✓ AWS us-west-2 | ✓ us-east-1 | ✓ eu-central-1 | ❌ Singapore only |
| MongoDB M0 | ✓ AWS us-west-2 | ✓ us-east-1 | ✓ eu-central-1 | ✓ ap-northeast-1 |
| DynamoDB | All AWS regions | All | All | ✓ ap-northeast-1 |
| Firestore | ✓ multi-region us | ✓ | ✓ eur3 | ✓ asia-northeast1 |
| PocketBase | wherever VM is | wherever | wherever | wherever |

**MongoDB, DynamoDB, Firestore, and PocketBase all cover Tokyo.** Cockroach does not. If the OCI VM lands in Tokyo and you can't move it, this is the only legitimate Cockroach disqualifier — and it shifts the recommendation toward DynamoDB.

## Go driver maturity (2026)

| Driver | Status | Quirks |
|---|---|---|
| `go.mongodb.org/mongo-driver/v2` | Stable, official | BSON struct tags everywhere; context-aware |
| `cloud.google.com/go/firestore` | Stable, Google-maintained | Implicit schemaless; relies on struct tags |
| `github.com/aws/aws-sdk-go-v2/service/dynamodb` | Stable, AWS-maintained | Verbose by design; expression builders help |
| pgx + crdbpgx (Cockroach) | Stable, Cockroach-maintained wrapper | One retry wrapper, otherwise vanilla pgx |
| PocketBase | Embedded Go SDK (`pocketbase.io/docs/go-overview`) OR HTTP client | If embedded, must run server in same process — design choice |

All four are production-grade. Boilerplate ranking: DynamoDB > Firestore > Mongo > pgx.

## Co-host on OCI Always-Free VM — feasibility

**OCI Always-Free** = 1 OCPU + 1 GB RAM + 200 GB storage on the VM-Std-E2.1.Micro shape. Coolify ~150–250 MB. Go server ~50 MB at idle. Budget remaining: ~600 MB.

### MongoDB self-hosted

- Idle MongoDB 7.x community edition: ~200–400 MB RAM
- Plus storage engine cache configurable, but minimum ~256 MB for stability
- **Tight but workable** on 600 MB
- Backups: `mongodump` to R2/B2 cron — straightforward
- **Operational cost:** real. You become the MongoDB SRE.
- **Verdict:** technically possible, not recommended for a solo project

### PocketBase self-hosted

- Idle: ~20 MB RAM, single 15 MB binary
- SQLite means zero RAM for "DB engine" — file system is the engine
- Backups: cron `sqlite3 .backup`, ship to object storage. 1-line cron job.
- **Easily fits** in remaining 600 MB with massive headroom
- Replaces: auth, admin UI, file storage, realtime subscriptions
- **Trade-off:** PocketBase wants to OWN your data layer. We have a custom protobuf protocol over WS already. Wiring our Go server to use PocketBase's HTTP API breaks our wire architecture.
- **Verdict:** would be the right pick **if we'd chosen it before designing our own protocol.** Pivoting now = ~2 weeks rework on a project where the foundation is fresh but the auth design is committed in `phase-03-backend-auth.md`.

### Cockroach self-hosted

- Single-node: ~500 MB RAM minimum
- **Will not fit** alongside Coolify + game server. Skip.

### Postgres self-hosted (not asked, but for completeness)

- ~150 MB RAM idle with conservative config
- Fits, but you become the DBA. No autopause but also no managed backups.

## When NoSQL would actually win for dleague

Three scenarios:

1. **OCI VM is in Tokyo and immovable.** Cockroach has no Tokyo region on free; DynamoDB does. → switch to DynamoDB.
2. **Schema is genuinely undefined and changing weekly.** Not us; auth schema is locked.
3. **Project hits >10 GiB data on a permanent free tier.** Cockroach caps at 10 GiB; DynamoDB at 25 GB. We'd probably be off free tier entirely by then for other reasons (compute > $15/mo credit).

Outside those scenarios: SQL wins.

## Final recommendation

> **Stay on CockroachDB Basic.**

**One decisive reason:** dleague's data is relational, the schema is locked in Phase 3 plan files, and SQL with `crdbpgx.ExecuteTx` solves every concurrency case the plan calls out (`SELECT FOR UPDATE`) in ~15 lines. NoSQL flexibility solves a problem we don't have at a cost (storage cap, access-pattern design, daily quotas, or operational burden) we do feel.

**The one valid reason to switch:** OCI VM lands in Tokyo and can't relocate. In that single case, switch to **DynamoDB Always-Free** (best Tokyo region, never-expires storage, native conditional-write atomicity for match-join). Plan ~1 day extra for GSI design + leaderboard aggregate table.

**Do NOT pick:** MongoDB M0 (storage too small), Firestore (daily hard caps), PocketBase (pivot cost too high given current architecture).

## Setup checklist confirmation (unchanged from prior report)

```bash
# Phase 3 — DB integration
go get github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx
go get github.com/jackc/pgx/v5

# Region match: Cockroach AWS eu-central-1 ↔ OCI Frankfurt
# DSN: postgres://user:pass@host:26257/dleague?sslmode=require
# All multi-statement txns: crdbpgx.ExecuteTx
# Single ops: pgxpool direct
```

## Unresolved questions

1. **OCI VM region** — locked decision. If Tokyo, switch to DynamoDB. Need user to confirm region.
2. **MongoDB M0 multi-document transactions on free tier** — searches did not definitively confirm. Atlas runs M0 as a 3-node replica set, so transactions theoretically work. Verify before any future re-evaluation.
3. **DynamoDB cost ramp at >25 RCU/WCU** — provisioned overage is $0.00065/RCU-hr, $0.00065/WCU-hr in us-east-1 = ~$0.50/mo per extra unit. Would need to estimate sustained load to compare against Cockroach's $15 credit ceiling.
4. **PocketBase as adjunct (not primary)** — could PB host admin UI + content management while CRDB owns transactional state? Probably overkill, but worth reconsidering at Phase 6 if we want a "report a bug" or "submit puzzle idea" form without writing UI code.
5. **DynamoDB local for dev** — `amazon/dynamodb-local` Docker image works; might or might not fit in OCI VM if we co-host for testing.

## Sources

- [MongoDB Atlas Free Cluster Limits](https://www.mongodb.com/docs/atlas/reference/free-shared-limitations/)
- [MongoDB Atlas Service Limits](https://www.mongodb.com/docs/atlas/reference/atlas-limits/)
- [MongoDB Pricing](https://www.mongodb.com/pricing)
- [AWS DynamoDB Pricing — Provisioned](https://aws.amazon.com/dynamodb/pricing/provisioned/)
- [Dynobase: DynamoDB Free Tier](https://dynobase.dev/dynamodb-free-tier/)
- [Firestore pricing](https://cloud.google.com/firestore/pricing)
- [Firebase Usage and Limits](https://firebase.google.com/docs/firestore/quotas)
- [PocketBase GitHub](https://github.com/pocketbase/pocketbase)
- [PocketBase FAQ](https://pocketbase.io/faq/)
- [PocketBase production discussion #4032](https://github.com/pocketbase/pocketbase/discussions/4032)
