---
type: research
title: "MySQL HeatWave Always-Free deep dive — operational answers for dleague Phase 3"
date: 2026-05-05
session: 260505-1308
sources_consulted: 5 web searches 2026-05-05
project: dleague
related: researcher-260505-1253-oci-databases.md
---

# MySQL HeatWave Always-Free — operational deep dive

> Goal: answer 10 specific questions before committing dleague Phase 3 to MySQL HeatWave Always-Free. Verdict at bottom: **GO**, with two unverified items flagged for empirical test.

## Q1. How many logical databases / schemas can I create?

**Answer:** As many as you want. Oracle does not cap the number of MySQL schemas (`CREATE DATABASE`), users, tables, or routines on Always-Free. The single binding constraint is **storage** (50 GB shared across everything).

- Tenancy-level: 1 Always-Free MySQL HeatWave DB system per tenancy
- Inside that DB system: unlimited schemas, MySQL's native limits apply (~4 B tables per schema, ~10 K columns per table — irrelevant for us)
- Practical use: you can host dleague + future side projects on the same instance with separate schemas (`dleague`, `another_app`, ...) sharing the 50 GB pool

Source: no documented schema cap in [HeatWave Always-Free creation docs](https://docs.public.content.oci.oraclecloud.com/en-us/iaas/mysql-database/doc/creating-always-free-db-system.html).

## Q2. Storage allocation model

**Primary storage — 50 GB total, shared across all schemas.** Not per-schema. Not per-database. Single InnoDB pool. Includes data files + binary/redo logs.

**Backup storage — 50 GB separate volume**, Oracle-managed. You don't see it as a filesystem. It backs the automatic backup window (Q6).

**Exceeding 50 GB:** writes start failing once InnoDB hits the cap. Not a "soft overage that bills you" — Always-Free has no overage path. You either:
- Free up data (DELETE rows + `OPTIMIZE TABLE` to reclaim)
- Upgrade to a paid shape
- Move to a paid Always-Free is not available (the shape is fixed, see Q3)

For dleague's <500 MB year-1 budget, the 50 GB pool is effectively infinite.

Source: [HeatWave Always-Free release note](https://docs.public.content.oci.oraclecloud.com/iaas/releasenotes/mysql-database/heatwave-always-free.htm), [Introducing HeatWave Always Free](https://blogs.oracle.com/mysql/introducing-heatwave-always-free).

## Q3. Always-Free DB system shape — exact spec

| Property | Value |
|---|---|
| Shape name | `MySQL.Free` |
| Resources | **Single 16 GB RAM node** (vCPU count not separately published; ECPU-based) |
| HeatWave OLAP cluster | **Optional add-on**, also Always-Free, single 16 GB node, **cannot be expanded** |
| MySQL major version | 8.x — Oracle defaults to current LTS (8.4 as of 2026-05) |
| OCPU shapes | **Deprecated, removed entirely after 2026-03-13.** ECPU is the standard now. Always-Free already uses the new model — non-issue for new deployments. |
| Read replicas on free shape | Not available (paid shapes only) |

**HeatWave OLAP cluster:** that's the in-memory analytics accelerator. dleague is OLTP — we don't need it. Skip provisioning the cluster.

Source: [HeatWave Always-Free landing](https://blogs.oracle.com/mysql/introducing-heatwave-always-free), [MySQL HeatWave Service Guide — supported shapes](https://docs.oracle.com/en-us/iaas/mysql-database/doc/supported-shapes.html), [Oracle blog: choosing the correct shape](https://blogs.oracle.com/mysql/choosing-the-correct-shape-when-moving-from-onpremise-to-heatwave-in-oci).

## Q4. Connections + pooling

| Property | Value | Notes |
|---|---|---|
| Default `max_connections` | **Unverified** — likely MySQL's 151 default | Empirical test: `SHOW VARIABLES LIKE 'max_connections'` after provisioning |
| Connection pooling included | No, app-side only | pgxpool-equivalent is `database/sql` connection pool — built into Go's stdlib |
| External pooler (ProxySQL/etc) | Not bundled, would need separate VM | YAGNI for dleague's 10 concurrent WS |
| TLS | Required by default; can be relaxed but Always-Free defaults to TLS-required | Use `tls=true` in DSN |

For dleague's <25-conn pgxpool-equivalent (`db.SetMaxOpenConns(25)`), we sit comfortably under MySQL's default 151. No pooler needed.

## Q5. Idle / inactivity / reclamation policy ⚠ UNVERIFIED

**No documented idle-stop or reclamation policy** for MySQL HeatWave Always-Free was found in 5 searches. This is in contrast to OCI **Autonomous Database** which has a well-documented 7-day auto-stop + 90-day reclamation policy.

**Cautious reading:** absence of evidence ≠ evidence of absence. Two possibilities:

1. MySQL HeatWave Always-Free truly has no idle-reclaim — instances run forever as long as you have a tenancy. (Most likely based on docs not mentioning it.)
2. Reclaim policy exists but is buried in tenancy-level Always-Free terms.

**Recommended empirical test before Phase 3 cutover:**
- Provision a `MySQL.Free` instance
- Don't connect for 14 days
- Check if it's still RUNNING

If it does idle-stop, mitigate with a 5-min `SELECT 1` cron from the Coolify VM (same playbook we'd have used for Aiven).

## Q6. Backup mechanics

| Property | Always-Free value | Notes |
|---|---|---|
| Automatic backups | **Enabled, fixed on, can't disable** | Oracle-managed snapshots |
| Backup retention | **1 day** ⚠ | Tight — restore window is single-day only |
| Final backup retention (after delete) | 7 days max | If you delete the DB system, backup persists 7 days |
| Backup window | Oracle-determined, not configurable | Brief I/O slowdown during snapshot |
| Point-in-time recovery (PITR) | **DISABLED, cannot enable** ⚠ | PITR is paid-only |
| Restore time | Minutes — restores create new DB system from snapshot | Not in-place |

**Implication:** the 1-day automatic-backup window is fragile for a project that wants to recover from "I dropped the wrong table on Sunday morning" on Monday afternoon. **Plan a complementary `mysqldump` cron** to OCI Object Storage (10 GB Always-Free), weekly retention 4–6 weeks. Total ops cost: ~30 min to write the script.

```bash
#!/usr/bin/env bash
# weekly cron — runs on Coolify VM
mysqldump --single-transaction --routines --triggers \
  -h "$HEATWAVE_HOST" -u "$DB_USER" -p"$DB_PASS" dleague \
  | gzip > "/tmp/dleague-$(date +%F).sql.gz"
oci os object put -bn dleague-backups \
  --file "/tmp/dleague-$(date +%F).sql.gz" \
  --name "weekly/dleague-$(date +%F).sql.gz"
```

Source: [Introducing PITR for MySQL HeatWave](https://blogs.oracle.com/mysql/introducing-point-in-time-recovery-for-mysql-heatwave-database-systems), [HeatWave backup events](https://docs.public.content.oci.oraclecloud.com/en-us/iaas/mysql-database/doc/backup-events.html), [HeatWave Always-Free creation docs](https://docs.public.content.oci.oraclecloud.com/en-us/iaas/mysql-database/doc/creating-always-free-db-system.html).

## Q7. Scale-up path

**Pricing detail unverified** — Oracle's pricing page is interactive, search results couldn't pull concrete $/mo figures. Use the [HeatWave cost estimator](https://www.oracle.com/mysql/cost-estimator/) when you're ready.

**What I can confirm:**
- Smallest paid shapes are ECPU-based: `MySQL.HeatWave.E3.Standalone.*`, `E4`, `E5`, `X9`, `A1`
- ECPU and Memory are billed as separate SKUs (~$0.0xx per ECPU-hour, separate $/GB-RAM-hour)
- Storage scales independently — incremental $ per GB-month
- **Read replicas: paid-only** (you cannot keep the writer on `MySQL.Free` and add a paid replica)
- **HA (3-node) deployment: paid-only**
- Shape change from `MySQL.Free` → paid: requires a **restart** (brief downtime, minutes)
- Once you leave `MySQL.Free`, you cannot return to it — that's a one-way door

**For dleague's likely outgrow trigger** (year 2, ~1k users, ~2 GB data):
- Probably the smallest standalone E3 shape: ~ $30–60 / month all-in for compute + 50 GB storage
- Cheaper than CockroachDB Standard's smallest tier, comparable to Aiven Startup

**Recommendation:** when you outgrow free, re-evaluate. Don't pre-pay for runway you won't need.

Source: [MySQL HeatWave Pricing](https://www.oracle.com/heatwave/pricing/), [MySQL HeatWave Cost Estimator](https://www.oracle.com/mysql/cost-estimator/).

## Q8. Networking model — answers the co-location question

**Yes, deployable in a private subnet of your own VCN.** This is what makes the recommendation worthwhile.

| Property | Value |
|---|---|
| Deployment subnet | Private subnet inside the same VCN as your Coolify VM |
| Endpoint type | Private IP only (no public endpoint by default) |
| Latency from same-VCN compute | **<1 ms** (intra-VCN, intra-AD) |
| Required ports | 3306 (MySQL classic), 33060 (X-Protocol) |
| Lock-down | Two layers — Security List (subnet-level) + Network Security Groups (VNIC-level, more granular) |

**Recommended setup:**
1. Deploy MySQL HeatWave in a **private subnet** (no internet gateway route)
2. Create an NSG: `dleague-db-access`, ingress rule `TCP 3306 from <coolify-vm-NSG>`
3. Attach the NSG to the HeatWave VNIC
4. App connects to `<heatwave-private-ip>:3306` — never exposed to internet

This is the textbook OCI pattern. No public endpoint, no IP allowlisting headache.

Source: [Enhancing security with NSGs](https://blogs.oracle.com/mysql/enhancing-security-in-oci-using-network-security-groups-heatwave-mysql), [HeatWave private subnet security list episode](https://dasini.net/blog/2021/09/07/discovering-mysql-database-service-episode-6-update-the-private-subnet-security-list/).

## Q9. Maintenance / downtime ⚠ UNVERIFIED

**No published SLA for `MySQL.Free` shape specifically.** Standard paid shapes have a documented Oracle maintenance policy with notification, but Always-Free isn't explicitly covered.

**What I can infer:**
- Single-node Always-Free → planned maintenance = brief restart (seconds to a minute or two)
- Oracle generally announces maintenance via the OCI console "notifications" panel and email
- For a hobby/indie game backend, expect 99.9%-ish observed uptime — short occasional brownouts during patch nights

**For dleague:** sync-PvP matches lasting ~5 min could be killed by a maintenance restart. Acceptable risk at <100 users; if it becomes painful, that's a signal to upgrade to HA shape.

## Q10. dleague-specific schema idioms (PG → MySQL)

### UUID storage — use **UUIDv7 + BINARY(16)**

```sql
CREATE TABLE users (
  id BINARY(16) PRIMARY KEY,                  -- UUIDv7 (time-ordered)
  email VARCHAR(254) NOT NULL,
  password_hash VARCHAR(120) NOT NULL,
  display_name VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_email_lower ((LOWER(email)))
);
```

**Why UUIDv7:** time-ordered, plays well with B-tree indexes, no insert-hotspot vs UUIDv4. **Industry-standard recommendation for new MySQL projects in 2025–2026** (Buildkite, Slack migrated to v7). Generate at the application layer in Go using e.g. [`github.com/google/uuid` v1.6+](https://pkg.go.dev/github.com/google/uuid) or `gofrs/uuid` with v7 support.

```go
import "github.com/google/uuid"

func newID() []byte {
    id, _ := uuid.NewV7()
    b, _ := id.MarshalBinary()
    return b
}
```

Source: [PlanetScale: UUID PK problem](https://planetscale.com/blog/the-problem-with-using-a-uuid-primary-key-in-mysql), [DEV.to: UUIDv4/v7 best practices](https://dev.to/snappy_tools/uuid-best-practices-v4-v7-and-when-you-should-use-each-2pie), [createuuid.com: UUID as PK v4 vs v7](https://createuuid.com/articles/uuid-as-database-primary-key).

### Case-insensitive email — functional unique index

MySQL 8.0.13+ supports functional unique indexes:

```sql
UNIQUE KEY uniq_email_lower ((LOWER(email)))
```

Verified syntax. Replaces PostgreSQL `email citext unique`. Slightly more verbose, semantically equivalent. The `LOWER(email)` expression is computed once at insert and stored in the index B-tree.

### MySQL JSON ≠ PostgreSQL JSONB (close enough)

| Operation | PostgreSQL | MySQL 8 |
|---|---|---|
| Extract value | `data->'key'` (JSONB) / `data->>'key'` (text) | `data->'$.key'` / `data->>'$.key'` |
| Path-based lookup | `data#>'{a,b}'` | `JSON_EXTRACT(data, '$.a.b')` |
| Contains | `data @> '{"k":"v"}'` | `JSON_CONTAINS(data, '"v"', '$.k')` |
| Functional index on JSON path | Generated column or expression index | Generated column + index (fully supported) |

For dleague we use JSON as opaque blob for game-state snapshots. We don't query inside it. The dialect difference doesn't affect us.

### Other PG → MySQL translations

| PG idiom | MySQL equivalent | Notes |
|---|---|---|
| `gen_random_uuid()` | App-side `uuid.NewV7().Bytes()` → `BINARY(16)` | Don't generate in DB |
| `TIMESTAMPTZ` | `TIMESTAMP` (UTC) or `DATETIME` (no TZ) | Use `TIMESTAMP` + `?loc=UTC` in DSN |
| `INSERT ... ON CONFLICT (k) DO UPDATE` | `INSERT ... ON DUPLICATE KEY UPDATE` | Different syntax, same semantics |
| `SELECT FOR UPDATE` | `SELECT FOR UPDATE` | Identical syntax, well-defined under InnoDB REPEATABLE READ |
| `RETURNING` clause | Not supported | Use `LAST_INSERT_ID()` for auto-increment, separate `SELECT` for UUIDs |
| Recursive CTEs | `WITH RECURSIVE` | Same syntax, supported MySQL 8+ |

## Side table — limits at a glance

| Item | Always-Free value |
|---|---|
| DB systems per tenancy | 1 |
| Schemas per DB system | unlimited (storage-bound) |
| Tables per schema | MySQL native limit (~4 B) |
| Primary storage | 50 GB (shared) |
| Backup storage | 50 GB (Oracle-managed) |
| Automatic backup retention | 1 day |
| Final backup retention (post-delete) | 7 days |
| PITR | disabled, can't enable |
| Read replicas | not available |
| HA mode | not available |
| HeatWave OLAP cluster | optional, single 16 GB node, can't expand |
| Connections | MySQL default 151 (unverified Always-Free override) |
| Networking | private subnet of your VCN |
| Encryption in transit | TLS-required by default |
| Encryption at rest | enabled by Oracle |
| MySQL version | 8.x (LTS current) |
| Cost | $0/mo permanent |

## Final recommendation: **GO**

Commit dleague Phase 3 to MySQL HeatWave Always-Free. Three caveats to act on, none of them blockers:

### Caveats

1. **Test idle-reclaim policy empirically.** Provision the instance, leave idle for 14 days, confirm it's still RUNNING. If it idle-stops, add a 5-min `SELECT 1` cron from Coolify VM. (~30 min if needed.)
2. **Add weekly `mysqldump` to OCI Object Storage** because Always-Free's 1-day backup retention is too tight to recover from a Sunday-morning oops on Monday afternoon. (~30 min upfront.)
3. **Don't provision the HeatWave OLAP cluster.** We're OLTP. The cluster is optional and would burn no extra money but also gives no value for our workload.

### Watch triggers

- Storage approaches 40 GB → trim or upgrade
- Idle-reclaim observed in test → add keepalive
- Sync-PvP downtime during maintenance windows becomes painful → upgrade to HA paid tier
- Any change to "Always-Free" guarantees → migrate to CockroachDB Basic (the documented backup plan)

## Phase 3 setup checklist (final)

```bash
# 1. Provision (OCI console)
#    - MySQL HeatWave DB System → Always Free
#    - Same VCN as Coolify VM
#    - Private subnet
#    - NSG: dleague-db, allows TCP 3306 from coolify-vm NSG only
#    - MySQL 8 (default)
#    - Skip "HeatWave Cluster" toggle

# 2. App-side deps
go get github.com/go-sql-driver/mysql
go get github.com/google/uuid           # for v7 generation

# 3. DSN (Coolify env var)
DATABASE_URL="dleague_app:${DB_PASS}@tcp(<heatwave-private-ip>:3306)/dleague?tls=true&parseTime=true&loc=UTC"

# 4. Connection pool size
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(30 * time.Minute)

# 5. Schema bootstrap (single migration file)
CREATE DATABASE IF NOT EXISTS dleague CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
CREATE USER 'dleague_app'@'%' IDENTIFIED BY '...';
GRANT ALL ON dleague.* TO 'dleague_app'@'%';

# 6. Backup cron (Coolify VM, weekly)
#    See Q6 script

# 7. Document MySQL idioms + UUIDv7 in docs/code-standards.md
```

## Unresolved questions

1. **Idle-reclaim policy on `MySQL.Free`** — needs 14-day empirical test before Phase 3 cutover. Highest priority unverified item.
2. **Default `max_connections` on Always-Free** — likely 151 (MySQL default) but Oracle could override. `SHOW VARIABLES LIKE 'max_connections'` after provisioning to confirm.
3. **Maintenance window SLA for `MySQL.Free`** — not published. Track via OCI notifications panel after deploy; record observed uptime over Phase 3–4.
4. **Specific paid shape pricing** — Oracle's pricing page is interactive; couldn't extract concrete $/mo for `E3.Standalone.X.Y`. Use the [cost estimator](https://www.oracle.com/mysql/cost-estimator/) when ready to upgrade.
5. **Single-tenancy limitation** — Always-Free is "1 DB system per tenancy". If we run multiple side projects on the same OCI account, they all share that one instance and 50 GB. Is that fine? (For dleague + 1–2 small projects: yes. For 5+ apps: revisit.)
6. **Cross-project schema isolation** — if multiple apps share the instance via separate schemas, MySQL `GRANT` per-schema isolation is fine, but accidental cross-schema queries by an over-permissioned user are a footgun. Use one MySQL user per app, scoped to one schema.

## Sources

- [HeatWave Always-Free release note](https://docs.public.content.oci.oraclecloud.com/iaas/releasenotes/mysql-database/heatwave-always-free.htm)
- [Creating an Always Free MySQL HeatWave DB System](https://docs.public.content.oci.oraclecloud.com/en-us/iaas/mysql-database/doc/creating-always-free-db-system.html)
- [Introducing HeatWave Always Free (Oracle blog)](https://blogs.oracle.com/mysql/introducing-heatwave-always-free)
- [Creating and Connecting to A HeatWave MySQL Always Free Instance](https://blogs.oracle.com/mysql/heatwave-mysql-always-free-tier)
- [MySQL HeatWave Service Guide — supported shapes](https://docs.oracle.com/en-us/iaas/mysql-database/doc/supported-shapes.html)
- [Choosing the correct shape (Oracle blog)](https://blogs.oracle.com/mysql/choosing-the-correct-shape-when-moving-from-onpremise-to-heatwave-in-oci)
- [MySQL HeatWave pricing](https://www.oracle.com/heatwave/pricing/)
- [MySQL HeatWave cost estimator](https://www.oracle.com/mysql/cost-estimator/)
- [Enhancing security with Network Security Groups](https://blogs.oracle.com/mysql/enhancing-security-in-oci-using-network-security-groups-heatwave-mysql)
- [Discovering MySQL Database Service — private subnet security list](https://dasini.net/blog/2021/09/07/discovering-mysql-database-service-episode-6-update-the-private-subnet-security-list/)
- [Introducing PITR for MySQL HeatWave](https://blogs.oracle.com/mysql/introducing-point-in-time-recovery-for-mysql-heatwave-database-systems)
- [Backing up and restoring MySQL HeatWave](https://blogs.oracle.com/mysql/backing-up-and-restoring-a-mysql-heatwave-instance-with-the-oci-cli)
- [PlanetScale: UUID PK problem](https://planetscale.com/blog/the-problem-with-using-a-uuid-primary-key-in-mysql)
- [DEV.to: UUIDv4/v7 best practices](https://dev.to/snappy_tools/uuid-best-practices-v4-v7-and-when-you-should-use-each-2pie)
- [createuuid.com: UUID as PK v4 vs v7](https://createuuid.com/articles/uuid-as-database-primary-key)
- [MySQL Storing UUID Values](https://dev.mysql.com/blog-archive/storing-uuid-values-in-mysql-tables/)
