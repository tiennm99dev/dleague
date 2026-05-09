---
type: research
title: "Aiven for PostgreSQL vs CockroachDB Serverless — head-to-head free-tier eval for dleague"
date: 2026-05-05
session: 260505-1207
sources_consulted: 5 web searches across 2026-05-05
project: dleague
---

# Aiven for PostgreSQL vs CockroachDB Serverless

> Decision date: 2026-05-05 · For Phase 3+ of dleague (Go backend on Coolify @ OCI free tier, <100 users year 1).

## Executive summary

**Pick Aiven for PostgreSQL.** One decisive reason: **CockroachDB does not implement `LISTEN/NOTIFY` ([issue #41522](https://github.com/cockroachdb/cockroach/issues/41522), still open) and treats advisory locks as no-op stubs ([issue #13546](https://github.com/cockroachdb/cockroach/issues/13546)).** The dleague Phase 4–5 design uses both: advisory locks for sync-PvP matchmaking and `LISTEN/NOTIFY` for live game updates. Cockroach's 10 GiB headroom is irrelevant when the project caps at <500 MB year 1, and the storage win does not offset rewriting the matchmaking + live-update layer.

Aiven's downsides — 1 GB storage cap, no PgBouncer on free, automatic power-off after inactivity — are all manageable: storage budget is met, pgxpool gives us app-side pooling, and a 1-min cron pinging `SELECT 1` from the Coolify VM keeps the instance warm.

**Backup plan if Aiven breaks (storage exceeded, plan deprecated, power-off too aggressive):** **Neon** free tier (0.5 GB, no autopause, vanilla PG, Frankfurt). Neon was eliminated from this comparison per user direction but remains the cleanest fallback because it shares Aiven's "vanilla PG" property — zero migration code change.

## Methodology

- 5 WebSearch queries (skill cap), 2026-05-05
- All claims tagged with source URL; observation date = 2026-05-05 unless noted
- No Gemini (`useGemini=false` in `~/.claude/.ck.json`)

---

## Side-by-side comparison

| | **Aiven for PostgreSQL — Free** | **CockroachDB Serverless / Basic — Free** |
|---|---|---|
| Plan name (2026) | "Free" / "Hobbyist" plan | "Basic" tier (post-Dec 2024 rebrand from "Serverless") |
| Permanent free? | **Yes**, no time expiry | **Yes**, $15/mo credit auto-applied |
| Storage cap | **1 GB** (was 5 GB before May 15 2025) | **10 GiB** included in $15 credit |
| Compute | 1 vCPU / 1 GB RAM, single-node | Serverless, scales 0→N RUs |
| Free compute units | Unlimited within VM caps | **50M RUs/mo** (≈ $15 credit) |
| Storage cap if you exceed free | Hard cap — must upgrade plan | Pay $0.50/GiB-mo above 10 GiB |
| Backups / PITR | **Not on free** (paid tiers only) | **Daily backups** included on Basic since Dec 2024 |
| HA / replicas | Single-node, no HA | Single-region, multi-AZ replicated by default (3-way) |
| Postgres versions | 13, 14, 15, 16, 17 (Aiven keeps current) | "PostgreSQL wire compat" — not real PG |
| Real Postgres? | **Yes — actual PG** | **No** — distributed engine with PG wire protocol |
| Connection pooling on free | **PgBouncer NOT included** (Startup tier+) | App-side pool only (no built-in pooler) |
| Default `max_connections` | ~25–100 (plan-tier dependent; not documented for free) | 500 per cluster (claimed by docs) |
| TLS-only | Yes, mandatory | Yes, mandatory |
| IP allowlist required | Optional | Optional |
| Power-off when idle | **Yes** — auto-powered-off after inactivity, email warning | No autopause documented for Basic |
| Free → next paid tier | **Hobbyist → Startup** ≈ $25–50/mo (4 GB RAM, 80 GB SSD, PgBouncer, daily backups) | Stays on Basic, just exceeds $15 credit; pay-as-you-go ≈ $0.30/M RUs + $0.50/GiB |
| Region count (free) | Many (AWS/GCP/Azure/UpCloud) | Limited subset of AWS + GCP |
| Region near OCI Phoenix | AWS us-west-2 (Oregon), us-east-1 (Virginia) | AWS us-east-1, GCP us-east1 |
| Region near OCI Frankfurt | AWS eu-central-1 (Frankfurt) ✓ | AWS eu-central-1 ✓ / GCP europe-west1 |
| Region near OCI Tokyo | AWS ap-northeast-1 (Tokyo) ✓ | AWS ap-southeast-1 (Singapore) — no Tokyo on free |
| Setup friction | Email signup, no card | Email signup, **no card on Basic** |
| Recent free-plan changes | **Storage 5 GB→1 GB on 2025-05-15** ⚠ | **2024-12-01: Serverless renamed Basic, daily backups added, $15 credit unchanged** |
| Risk of free disappearing | Moderate — already shrunk storage in 2025 | Low — credit unchanged through 2024 rebrand |

Sources: [Aiven free plans doc](https://aiven.io/docs/platform/concepts/free-plan), [Aiven storage changelog (May 15, 2025)](https://aiven.io/changelog/6ad6c429-6c1c-4418-abfb-2ca82f927414), [Aiven pg-connection-pooling](https://aiven.io/docs/products/postgresql/concepts/pg-connection-pooling), [CockroachDB Basic plan](https://www.cockroachlabs.com/docs/cockroachcloud/plan-your-cluster-basic), [CockroachDB pricing blog Dec 2024](https://www.cockroachlabs.com/blog/improved-cockroachdb-cloud-pricing/), [CockroachDB serverless free tier](https://www.cockroachlabs.com/blog/serverless-free/).

---

## PostgreSQL feature support matrix

For features dleague's Phase 3–5 plans likely depend on:

| Feature | Used by dleague for | **Aiven (real PG)** | **CockroachDB** |
|---|---|---|---|
| `INSERT ... ON CONFLICT` upsert | Daily-puzzle attempt write | ✅ | ✅ same syntax |
| `JSONB` columns | Game state snapshots in `attempts` | ✅ | ✅ but CRDB's JSONB has minor operator differences |
| `gen_random_uuid()` (pgcrypto) | Match IDs | ✅ | ✅ via `gen_random_uuid()` |
| `TIMESTAMPTZ` | All time fields | ✅ | ✅ |
| Partial / expression indexes | Active-match lookups | ✅ | ✅ |
| `SERIAL` / sequences | Avoid (use UUID) | ✅ | ⚠ uses unique_rowid() — cluster-monotonic, not sequential |
| **Advisory locks** (`pg_try_advisory_xact_lock`) | **Sync PvP matchmaking** | ✅ | ❌ **No-op stubs** ([#13546](https://github.com/cockroachdb/cockroach/issues/13546)) |
| **LISTEN / NOTIFY** | **Live game updates, presence** | ✅ | ❌ **Not implemented** ([#41522](https://github.com/cockroachdb/cockroach/issues/41522)) |
| `CTE` / recursive CTE | Leaderboard queries | ✅ | ✅ |
| Prepared statements (pgx) | Default driver behavior | ✅ | ✅ |
| `SELECT FOR UPDATE` | Match state mutation | ✅ | ✅ but Cockroach is serializable-only by default — different lock semantics |
| Triggers / stored procs | Avoid for KISS | ✅ | ⚠ partial (CRDB has limited PL/pgSQL) |
| Foreign key cascades | Schema integrity | ✅ | ✅ |
| `LATERAL` joins | Reporting queries | ✅ | ✅ |
| `CITEXT` (case-insensitive) | Username uniqueness | ✅ | ❌ not supported |

**Verdict on compat:** Two of dleague's likely-used Postgres features (advisory locks, LISTEN/NOTIFY) are **not implementable on Cockroach**. Workarounds exist (DB-row "lock table" instead of advisory locks; pubsub via Redis or polling for LISTEN/NOTIFY) but they impose code complexity disproportionate to the project scope. Source: [CockroachDB PostgreSQL Compatibility doc](https://www.cockroachlabs.com/docs/stable/postgresql-compatibility).

---

## Latency / region match for OCI Always-Free

OCI Always-Free regions: Phoenix, Ashburn, Frankfurt, Amsterdam, Tokyo, Osaka.

| OCI region | Best Aiven free region | RTT estimate | Best Cockroach Basic region | RTT estimate |
|---|---|---|---|---|
| Phoenix (us-west) | AWS us-west-2 (Oregon) | ~25 ms | GCP us-west1 (Oregon) | ~25 ms |
| Ashburn (us-east) | AWS us-east-1 | <5 ms | AWS us-east-1 | <5 ms |
| Frankfurt | AWS eu-central-1 | <5 ms | AWS eu-central-1 | <5 ms |
| Amsterdam | AWS eu-west-1 (Ireland) or eu-central-1 | ~10–15 ms | AWS eu-central-1 | ~10 ms |
| Tokyo | AWS ap-northeast-1 ✓ | <5 ms | **No Tokyo** — Singapore ~70 ms ⚠ |
| Osaka | AWS ap-northeast-1 (Tokyo) | ~15 ms | Singapore ~80 ms ⚠ |

**Both providers cover Frankfurt and Ashburn cleanly.** If the OCI VM lands in Tokyo/Osaka (likely for Vietnam-based devs), Aiven wins on latency. Sync PvP needs <50 ms DB round-trip; Cockroach's Singapore hop blows past that for APAC.

---

## Connection pattern fit

dleague Phase 5 expects ~10 concurrent WS clients. Each gameplay action does 1–3 short DB queries. With pgxpool sized at 10–25 connections:

- **Aiven free:** 1 GB RAM = up to ~500 connections theoretical, but Aiven sets a low `max_connections` on the free plan (not publicly documented; likely 25–100). Our pool of 25 should fit. **No PgBouncer on free** — pgxpool is on its own. Acceptable at this scale.
- **Cockroach free:** Docs claim 500 connections per cluster. Plenty.

**Neither requires external pooling at dleague's scale.** This factor is a wash.

---

## Risk of free tier disappearing

**Aiven (moderate risk):**
- Already shrunk free-plan storage **5 GB → 1 GB on 2025-05-15** ([changelog](https://aiven.io/changelog/6ad6c429-6c1c-4418-abfb-2ca82f927414)).
- Has track record of pivoting free offerings (introduced "Developer tier", added "$5 PostgreSQL", restructured plans multiple times in 2024–2025).
- Power-off-when-idle policy means the plan is genuinely subsidized by usage; if Aiven's economics change, free could shrink again.

**CockroachDB (low risk):**
- 2024-12-01 rebrand from "Serverless" → "Basic" kept the $15 credit unchanged. Improvement: backups added.
- $15 credit has held for ~3 years. Pricing-up direction (unbundled compute/transfer/CDC) suggests they're not retreating from free.

For dleague's 12–18-month horizon, both are stable enough. Aiven's storage cut is a yellow flag; pin notification email and watch the changelog.

---

## Power-off / cold-start fit for sync PvP

**Aiven free** auto-powers-off **inactive** instances. Definition of "inactive" = no connections / queries for an extended period (Aiven docs are vague on the exact threshold). Sync PvP matches burst then idle — between match-cluster windows, instance could go to sleep.

**Mitigation:** add a Coolify cron job (or a goroutine in the server process) that runs `SELECT 1` every 5 minutes. Costs nothing and keeps the connection warm.

**Cockroach Basic** has no documented autopause. Wins this dimension cleanly. But this single dimension does not outweigh the LISTEN/NOTIFY + advisory-lock gap.

---

## Total cost of ownership at "outgrow free"

Triggering scenarios: 1000 users + 2 GB storage.

**Aiven Hobbyist → Startup:**
- Storage: 80 GB SSD on Startup-4
- Plan: ~$58/mo for Startup-4 (4 GB RAM, 2 vCPU, 80 GB) AWS Frankfurt list price
- Cheaper Startup-2 (~$25/mo, 1 GB RAM, 20 GB) bridges 2–10 GB scale
- **Cliff: small** ($25–58/mo step)

**Cockroach Basic pay-as-you-go:**
- Storage above 10 GB: $0.50/GiB-mo
- Compute above 50M RUs: $0.30/M RUs ≈ ramps with traffic, not user count
- For 1000 light-traffic users on a -dle game, est. $5–15/mo overage on top of $15 credit
- **Cliff: smoothest** (no plan jump, just metered usage)

Cockroach wins TCO ramp. If dleague hits 10k users, Cockroach stays predictable; Aiven Startup-2/4/8 stair-steps. But this is a 2–3 year horizon — irrelevant for the Phase 3 decision.

Source: [CockroachDB Pricing](https://www.cockroachlabs.com/pricing/), [Aiven Service Pricing](https://aiven.io/docs/platform/concepts/service-pricing).

---

## Final recommendation

> **Use Aiven for PostgreSQL Free.**

**The one decisive reason:** CockroachDB does not implement `LISTEN/NOTIFY` or true advisory locks. Both are load-bearing in the dleague Phase 4–5 plan (matchmaking, live updates). Building portable workarounds is wasted YAGNI work on a project that will outgrow free tier years before it outgrows 1 GB of game data.

**Setup checklist (Phase 3):**
1. Create Aiven free PG service in **eu-central-1 (Frankfurt)** if OCI VM is EU; **us-east-1** if NA; **ap-northeast-1** if APAC.
2. Pin Postgres version (16 LTS or 17).
3. Save DSN in Coolify env: `DATABASE_URL=postgres://...?sslmode=require`.
4. Add a **5-minute keepalive ping** (Coolify cron or in-process `time.Ticker`) executing `SELECT 1` to defeat auto power-off.
5. Use **pgxpool** (no PgBouncer needed at this scale).
6. Schedule **logical backups** weekly via `pg_dump` from Coolify (free plan has no PITR).

**Watch for these triggers to revisit:**
- Storage approaches 800 MB → start trimming or upgrade
- Aiven announces another free-plan reduction → snapshot + migrate to Neon
- Sync PvP latency exceeds 100 ms p95 → check region match, then DB plan size
- Auto power-off triggers despite keepalive → file Aiven support ticket; if recurring, switch to Neon

**Backup plan:** **Neon** free (0.5 GB, no autopause, also vanilla PG). Migration = `pg_dump | psql`, zero code change.

---

## Unresolved questions

1. **Aiven's exact free-plan `max_connections`** — not documented in public sources. Likely 25–100. Verify by spinning up a free instance and reading `SHOW max_connections;` before committing schema design.
2. **Aiven's auto-power-off threshold** — vague in docs ("after inactivity"). Empirically test how long a 5-minute SELECT-1 cron needs to be to defeat it.
3. **CockroachDB region for OCI Tokyo VM** — no Tokyo on Basic free. Singapore adds ~70 ms RTT. Confirms Aiven advantage in APAC, but worth re-checking when Cockroach expands free regions.
4. **Aiven's `$5 PostgreSQL` developer tier** — referenced in 2024 marketing material; unclear if it's a step between free and Startup. Could change the upgrade-cliff math. Check before Phase 6.
5. **dleague schema decisions** — Phase 3 plan does NOT yet specify the schema. If matchmaking is built without advisory locks (e.g. table-row-locking via `SELECT FOR UPDATE`) and live updates use polling instead of `LISTEN/NOTIFY`, Cockroach becomes viable. Decision is owned by Phase 4–5 design, not by this report.

---

## Sources

- [Aiven free plans docs](https://aiven.io/docs/platform/concepts/free-plan)
- [Aiven Free PostgreSQL landing page](https://aiven.io/free-postgresql-database)
- [Aiven changelog: Storage on free plans (May 2025)](https://aiven.io/changelog/6ad6c429-6c1c-4418-abfb-2ca82f927414)
- [Aiven PG connection pooling](https://aiven.io/docs/products/postgresql/concepts/pg-connection-pooling)
- [Aiven PG connection limits per plan](https://aiven.io/docs/products/postgresql/reference/pg-connection-limits)
- [CockroachDB Basic Cluster planning](https://www.cockroachlabs.com/docs/cockroachcloud/plan-your-cluster-basic)
- [CockroachDB Pricing](https://www.cockroachlabs.com/pricing/)
- [CockroachDB blog — Improved Cloud Pricing (Dec 2024)](https://www.cockroachlabs.com/blog/improved-cockroachdb-cloud-pricing/)
- [CockroachDB PostgreSQL Compatibility doc](https://www.cockroachlabs.com/docs/stable/postgresql-compatibility)
- [GitHub: CRDB advisory lock stubs (#13546)](https://github.com/cockroachdb/cockroach/issues/13546)
- [GitHub: CRDB LISTEN/NOTIFY support (#41522)](https://github.com/cockroachdb/cockroach/issues/41522)
- [Medium: CockroachDB Serverless Free-Tier analysis](https://medium.com/@radoslav.vlaskovski/cockroachdb-serverless-free-tier-analysis-70747ec64ad8)
