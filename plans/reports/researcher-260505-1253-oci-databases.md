---
type: research
title: "OCI Always-Free databases for dleague — supersedes CockroachDB recommendation"
date: 2026-05-05
session: 260505-1253
supersedes: researcher-260505-1207-cockroach-fitness-followup.md
sources_consulted: 3 web searches 2026-05-05
project: dleague
---

# OCI Always-Free databases for dleague

> Companion to the SQL/NoSQL DB research thread. Question: since Coolify lives on an OCI Always-Free VM, do OCI's own free databases beat CockroachDB Basic? **Yes — MySQL HeatWave Always-Free wins.**

## TL;DR — verdict flipped (again)

**Use OCI MySQL HeatWave Always-Free.** Decisive reason: **co-located with the Coolify VM** in the same OCI VCN. That eliminates 10–50 ms cross-cloud DB hops on every request — meaningful for sync-PvP, free for everything else. On top of that, it's 50 GB storage (5× CRDB), 50 GB free backup storage, vanilla MySQL Go driver (`go-sql-driver/mysql`, no retry wrapper), and all OCI regions available including **Tokyo**.

The standing CRDB Basic recommendation was second-best because we hadn't asked: "what does OCI itself give us free?" Answer: a real DB, on the same network, no autopause, no third-party vendor.

## OCI Always-Free DB inventory (verified 2026-05-05)

| Service | Free quota | Type | dleague fit |
|---|---|---|---|
| **MySQL HeatWave** | **50 GB storage + 50 GB backup, 1 standalone DB system, 1-node HeatWave cluster** | Real MySQL | **Best fit** |
| Autonomous Database | 2 instances × 20 GB | Oracle SQL (ATP/ADW) | ⚠ See "skip" below |
| NoSQL Database | 133 M reads/mo + 133 M writes/mo, 25 GB/table, 3 tables max | Document/KV | Skip — only 3 tables |
| Object Storage | 10 GB | Files | Adjacent — for backups |

Sources: [OCI Always Free Resources](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm), [OCI MySQL HeatWave Always-Free DB system](https://docs.public.content.oci.oraclecloud.com/en-us/iaas/mysql-database/doc/creating-always-free-db-system.html), [HeatWave Always-Free release notes](https://docs.public.content.oci.oraclecloud.com/iaas/releasenotes/mysql-database/heatwave-always-free.htm), [Introducing HeatWave Always Free (Oracle blog)](https://blogs.oracle.com/mysql/introducing-heatwave-always-free).

## Why Autonomous Database is a trap for us

Don't pick Autonomous Database, despite the 40 GB total quota:

- **7-day auto-stop** on inactivity. Game backend during a quiet week → DB stops. Cold-start when first user hits the server in the morning. Source: [Always Free Autonomous Database docs](https://docs.oracle.com/en/cloud/paas/autonomous-database/adbsa/autonomous-always-free.html).
- **90-day reclamation** if a stopped instance stays stopped 90 days cumulative → **data permanently deleted**. Foot-gun.
- Mitigation requires a SQL*Net or HTTPS connection at least once a week. Doable but adds a babysitting cron.
- **Oracle SQL dialect** — not portable. Migration off Oracle is famously painful.
- Go driver story: **`godror`** (needs Oracle Instant Client + CGO) or **`go-ora`** (pure Go, faster, simpler). Both work, but neither has the maturity of `pgx` or `go-sql-driver/mysql`. Source: [Oracle godror install docs](https://docs.oracle.com/en/cloud/paas/autonomous-database/serverless/adbsb/cdw-connect-go-install-go-and-godror.html).

This is engineering effort spent on Oracle-isms instead of dleague.

## Why MySQL HeatWave Always-Free wins

### 1. Co-location is the killer feature

Coolify on OCI VM and MySQL HeatWave in the same VCN → DB round-trip is **<1 ms**. CockroachDB Basic from OCI to AWS (cross-cloud) → 10–50 ms typical. Across a sync-PvP match with 5–15 DB hits per minute, that's hundreds of milliseconds of avoidable latency per match.

### 2. Real MySQL with vanilla Go driver

`github.com/go-sql-driver/mysql` is one of Go's best-maintained `database/sql` drivers. Zero retry wrapper needed (MySQL's `REPEATABLE READ + SELECT FOR UPDATE` is forgiving). Every Go developer knows MySQL.

Compare to CRDB's required `crdbpgx.ExecuteTx` for serializable retries. Saves us ~30 LOC and a dependency.

### 3. Storage room

- 50 GB primary + 50 GB backup = **100 GB total** included
- vs CRDB Basic 10 GiB total
- vs Aiven free 1 GB
- Year-1 budget <500 MB → essentially infinite headroom

### 4. All OCI regions including Tokyo

Eliminates the one outstanding CRDB disqualifier. Wherever the Coolify VM sits, MySQL HeatWave is right there.

### 5. One vendor, one bill, one support channel

The previous CRDB recommendation meant: OCI + CockroachDB + Coolify + GitHub + (eventually) Cloudflare. MySQL HeatWave collapses one of those. When the bill arrives or something breaks, "it's all Oracle" simplifies the post-mortem.

## What we trade by leaving CRDB

| Capability | CRDB Basic free | MySQL HeatWave free | Impact for dleague |
|---|---|---|---|
| Multi-AZ replication | 3-way replicated | **Single-node** | Sync-PvP downtime if node restarts. ~99.9% availability acceptable for indie game. |
| Daily managed backups | Included | **Manual via `mysqldump`** | Must script weekly backup → R2/B2. ~1 hour to set up. |
| `CITEXT` native | ✓ | ❌ | Replace `email citext` with `email VARCHAR(254) + UNIQUE INDEX (LOWER(email))`. 5-line schema change. |
| `JSONB` PG semantics | ✓ | MySQL `JSON` — different operators | We use as opaque blob. Doesn't matter. |
| Distributed serializable | ✓ | `REPEATABLE READ` default | MySQL semantics are well-understood by every dev. Match-join via `SELECT FOR UPDATE` works exactly as in PG. |
| Connection pooling | App-side (pgx) | App-side (go-sql-driver/mysql) | Wash. |
| Free tier permanence | $15/mo credit, ~3 yr track record | OCI Always Free guarantee, written into ToS | OCI is arguably the safer commitment — Oracle will not unilaterally reduce Always-Free without massive blowback. |

## Migration plan from current setup

Phase 1 already shipped — DB doesn't yet exist. For Phase 3:

1. **Provision** OCI MySQL HeatWave Always-Free DB system in same region as Coolify VM (Phoenix / Ashburn / Frankfurt / Tokyo / etc.)
2. **Networking:** put DB in a private subnet of the same VCN. Security list: allow port 3306 from Coolify VM only.
3. **Schema:** translate `phase-03-backend-auth.md:38` from PG-isms to MySQL:
   ```sql
   -- was: users (id uuid pk, email citext unique, ...)
   CREATE TABLE users (
     id BINARY(16) PRIMARY KEY,                   -- UUID stored as BINARY(16)
     email VARCHAR(254) NOT NULL,
     password_hash VARCHAR(120) NOT NULL,
     display_name VARCHAR(64) NOT NULL,
     created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
     UNIQUE KEY uniq_email_lower ((LOWER(email)))
   );
   ```
4. **Go deps:**
   ```bash
   go get github.com/go-sql-driver/mysql
   # plus optional: github.com/jmoiron/sqlx for tagged scans
   ```
5. **Connection string:** `DATABASE_URL=user:pass@tcp(<heatwave-host>:3306)/dleague?tls=true&parseTime=true&loc=UTC`
6. **Backups:** Coolify cron, weekly:
   ```bash
   mysqldump --single-transaction --routines --triggers \
     -h $HOST -u $USER -p$PASS dleague \
     | gzip > /backups/dleague-$(date +%F).sql.gz
   # ship to OCI Object Storage (10 GB free)
   ```

## Side-by-side vs all prior candidates

| | CRDB Basic | **OCI MySQL HeatWave** | Aiven Free PG | MongoDB M0 | DynamoDB Always-Free |
|---|---|---|---|---|---|
| Storage | 10 GiB | **50 GB** ✓ | 1 GB | 512 MB | 25 GB |
| Backups free | ✓ daily | manual `mysqldump` (50 GB free space) | ❌ | ❌ | ❌ |
| Co-located with OCI VM | ❌ cross-cloud | **✓ same VCN, <1 ms** ✓ | ❌ | ❌ | ❌ |
| Tokyo region | ❌ | **✓ all OCI regions** ✓ | ✓ | ✓ | ✓ |
| Autopause | None ✓ | None ✓ | yes ⚠ | None | None |
| Real SQL | PG wire | **Real MySQL** ✓ | PG | NoSQL | NoSQL |
| Go driver | pgx + crdbpgx wrapper | **go-sql-driver/mysql** ✓ | pgx | mongo-driver | aws-sdk-go-v2 |
| Free 3-way replication | ✓ | ❌ single-node | ❌ | replica set | multi-AZ |
| Free tier change risk | Low | **Lowest** (OCI Always Free contractual) | Moderate (cut once) | Low | Lowest |
| Vendor sprawl | OCI + Cockroach Labs | **OCI only** ✓ | OCI + Aiven | OCI + MongoDB | OCI + AWS |

## Final recommendation

> **Switch to OCI MySQL HeatWave Always-Free.**

**One decisive reason:** the database lives in the same OCI VCN as the Coolify VM, eliminating cross-cloud network hops on every request. For a real-time PvP game, that single fact matters more than the marginal CRDB advantages (3-way replication + daily managed backups).

**Pick region** = same as Coolify VM. All OCI regions support MySQL HeatWave Always-Free.

**Setup checklist (Phase 3):**

1. Provision MySQL HeatWave DB system, Always-Free, same region as Coolify VM
2. Place in same VCN, security list allows 3306 from Coolify VM CIDR only
3. Schema = MySQL flavor (no CITEXT, use `LOWER(email)` unique; UUIDs as `BINARY(16)`)
4. `go get github.com/go-sql-driver/mysql`
5. DSN via Coolify env var (sslmode=verify-full, TLS-only)
6. Weekly `mysqldump` cron → OCI Object Storage 10 GB free bucket
7. Document MySQL idioms in `docs/code-standards.md`

**Watch triggers to revisit:**

- Sync-PvP needs cross-region failover → upgrade to MySQL HeatWave HA (paid)
- Storage exceeds 40 GB → trim or upgrade
- Always-Free policy ever changes (Oracle would need to formally amend ToS — high blast-radius signal)

**Backup plan:** if MySQL HeatWave Always-Free proves operationally bad in dev (capacity-out errors, region unavailability), fall back to **CockroachDB Basic** — the migration is `mysqldump → load via `crdb-import` after dialect tweaks. Estimate ~half a day.

## What I'd do differently looking back

The original DB-research thread should have started with: **"is there a free DB on the same cloud as the app server?"** before surveying third-party SaaS. Co-location is usually the first-order factor for a small project; vendor sprawl is the silent tax.

Lesson recorded for similar decisions: when the app host is committed (OCI here), check the host's own DB free tier before evaluating external candidates.

## Unresolved questions

1. **MySQL HeatWave Always-Free idle behavior** — searches did NOT find a documented idle-reclaim policy for HeatWave (unlike Autonomous Database's 7-day stop). Verify by leaving an instance idle for 2+ weeks before Phase 3 production cutover. If reclaim exists, add a keepalive cron similar to Aiven.
2. **HeatWave standalone vs HeatWave cluster** — Always-Free includes "1 standalone DB system + 1 single-node HeatWave cluster". For OLTP (our use case), the standalone DB is what we want; the HeatWave cluster is for OLAP analytics. Confirm we're provisioning the standalone, not the cluster, to avoid waste.
3. **Networking from Coolify** — does Coolify's bundled Docker proxy + OCI's VCN cooperate cleanly for service-to-service TLS? Validate with a smoke test before committing.
4. **Restore drill** — `mysqldump | mysql` restore on a fresh instance: practice once before Phase 4 ships, before any data exists that we'd care about losing.
5. **Always-Free quota across services** — Always-Free is per-tenancy, not per-resource. Confirm that running Coolify VM + MySQL HeatWave + Object Storage backups doesn't exceed any aggregate cap.

## Sources

- [OCI Always Free Resources](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm)
- [Oracle Cloud Free Tier landing](https://www.oracle.com/cloud/free/)
- [Creating Always Free MySQL HeatWave DB System](https://docs.public.content.oci.oraclecloud.com/en-us/iaas/mysql-database/doc/creating-always-free-db-system.html)
- [HeatWave Always-Free release note](https://docs.public.content.oci.oraclecloud.com/iaas/releasenotes/mysql-database/heatwave-always-free.htm)
- [Introducing HeatWave Always Free (Oracle blog)](https://blogs.oracle.com/mysql/introducing-heatwave-always-free)
- [Always Free Autonomous Database (7-day stop, 90-day reclaim)](https://docs.oracle.com/en/cloud/paas/autonomous-database/adbsa/autonomous-always-free.html)
- [Always Free Autonomous AI Database](https://docs.oracle.com/en/cloud/paas/autonomous-database/serverless/adbsb/autonomous-always-free.html)
- [godror Go driver](https://godror.github.io/godror/doc/connection.html)
- [godror pkg.go.dev](https://pkg.go.dev/github.com/godror/godror)
- [Oracle docs: Install Go and godror](https://docs.oracle.com/en/cloud/paas/autonomous-database/serverless/adbsb/cdw-connect-go-install-go-and-godror.html)
- [Working in Go with Oracle DB and Autonomous DB](https://www.oracle.com/developer/working-in-go-applications-with-oracle-database-and-oracle-cloud-autonomous-database/)
