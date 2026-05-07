---
title: MongoDB Atlas Evaluation for dleague Beta
author: researcher
date: 2026-05-07
---

## TL;DR Recommendation

1. **M0 is viable for beta** (100s-1000s DAU). 512 MB storage, 500 max connections, 100 ops/sec limit. Zero cost. Will auto-pause after 30 days inactivity (resume anytime).

2. **Change streams: blocker.** M0 does not support change streams (requires M10+). If Redis ZSET→MongoDB leaderboard migration uses change streams, upgrade to M10 ($57/mo). If you can poll/batch instead, stay on M0.

3. **First upgrade: M10 @ $57/mo** (dedicated cluster, full feature set, point-in-time recovery, 10-120 GB storage).

4. **Go driver:** Official `go.mongodb.org/mongo-driver/v2` (v2.6.0 GA as of 2026). Mature, supports change streams natively. Drop-in replacement for Couchbase gocb v2.

5. **Network + auth:** IP allowlist required (no PrivateLink on M0). SCRAM connection string injected as env var (`MONGODB_URI`). Public internet only—no latency penalty vs Couchbase. TTL indexes fully supported on M0.

---

## 1. M0 Free Tier Specifications

### Storage & Throughput
- **Storage cap:** 512 MB (hard limit)
- **Databases:** Max 100
- **Collections:** Max 500 total across all databases
- **Connections:** 500 max (concurrent clients)
- **Operations/sec:** 100 ops/sec sustained (throttled on breach)
- **Sort memory:** 32 MB per query
- **Data transfer (7-day rolling):** 10 GB in + 10 GB out

**Assessment:** For daily puzzles + attempts (1 per user/day) + match records in <1k DAU: easily within limits. 4 collections (users, puzzles, attempts, matches) well under 500.

### Auto-Pause Behavior
- **Trigger:** 30 days zero connections → cluster pauses
- **Recovery:** Manual resume (no data loss) or API call
- **Impact on beta:** If you run continuous health checks/cron jobs, cluster stays active. No risk for active development.

### Replication & Availability
- **Nodes:** 3-node replica set (fixed, non-configurable)
- **Backup:** None. Use `mongodump`/`mongorestore` manually
- **Point-in-time recovery:** Not available
- **Encryption at rest:** No customer key management

---

## 2. Change Streams: Hard Blocker on M0

**Source:** [Atlas Free Cluster Limits](https://www.mongodb.com/docs/atlas/reference/free-shared-limitations/)

Change streams **NOT supported on M0**. Change streams require replica sets with oplog access—M0 restricts access to oplog (read-only for internal use).

**If Redis leaderboard logic uses change streams:**
- **Option A:** Upgrade to M10+ ($57/mo)
- **Option B:** Refactor to polling/batch updates (keep M0 free)
  - Poll `attempts` collection for new records every minute/hour
  - Compute leaderboard scores via aggregation pipeline
  - Cache in MongoDB or memory-backed cache

**Recommendation:** Audit current Redis change-stream usage. For simple daily leaderboards, polling is simpler and cost-free.

---

## 3. First Paid Upgrade Path: M10

### Specs & Cost
- **Monthly cost:** ~$57 USD (0.08/hour × 720 hours)
- **Storage:** 10-120 GB (configurable)
- **Connections:** No hard limit (thousands)
- **Throughput:** No ops/sec throttle
- **Backup:** Automated (configurable retention, 7-35 days default)
- **Point-in-time recovery:** Continuous, 7-day window default
- **Regions:** Full Atlas region catalog (Singapore, Mumbai, Tokyo available)

### What Changes from M0 → M10
- Dedicated cluster (not shared)
- Change streams ✓
- Custom encryption keys ✓
- Network peering + private endpoints ✓
- Full oplog access ✓
- Dedicated monitoring & alerting ✓

**Verdict:** If change streams needed for beta, jump to M10. No intermediate tier worth consideration.

---

## 4. MongoDB Go Driver

### Official Driver
- **Package:** `go.mongodb.org/mongo-driver/v2`
- **Current version:** 2.6.0 (May 2026)
- **Status:** GA (production-ready since Jan 2025)
- **Release notes:** [Release Notes - Go Driver](https://www.mongodb.com/docs/drivers/go/current/reference/release-notes/)

### Import
```go
import "go.mongodb.org/mongo-driver/v2/mongo"
```

### Key Features
- **Change streams:** Fully supported
- **SCRAM + OIDC auth:** Native support
- **Streaming BSON:** (v2.3+ fixed earlier performance regressions)
- **Client-side operation timeout:** CSOT for timeouts on client side

### Comparison to Couchbase gocb v2
| Dimension | MongoDB driver v2 | Couchbase gocb v2 |
|-----------|------------------|------------------|
| Maturity | GA (2.6.0) | Stable |
| API style | More idiomatic Go (v2.0+) | Couchbase-specific |
| Change streams | Yes, native | No |
| TTL/expiry | TTL indexes | Expiry in SDK |
| Community | Large (MongoDB) | Smaller |

**Verdict:** MongoDB Go driver v2 is drop-in compatible in terms of interface pattern (Open/Close, Find, InsertOne, etc.). Slightly better ergonomics post-v2.0.

---

## 5. Network Access & Authentication

### IP Allowlist Requirement
- **Required:** Yes, always (M0 and all tiers)
- **PrivateLink:** Not available on M0; available on M10+ (AWS, Azure)
- **For beta on OCI ARM64 Coolify:** Add Coolify outbound IP to Atlas allowlist (or use `0.0.0.0/0` for dev, restrict later)
- **Public internet:** All traffic over TLS/1.2+. No hidden fees or latency penalties vs Couchbase.

**Source:** [Configure IP Access List Entries](https://www.mongodb.com/docs/atlas/security/ip-access-list/)

### SCRAM Authentication
- **Connection string format:**
  ```
  mongodb+srv://username:password@cluster.mongodb.net/?authMechanism=SCRAM-SHA-256
  ```
- **Injection method:** Safe to inject `MONGODB_URI` env var in Coolify
- **URL encoding:** Special chars in password → use `encodeURIComponent()` (Go driver handles automatically)
- **Protocol:** SCRAM-SHA-256 recommended (more secure than SCRAM-SHA-1)

**Verdict:** Identical security posture to Couchbase + RBAC. Simple env-var setup.

---

## 6. Backup & Point-in-Time Recovery

### M0 Free Tier
- **Automated backups:** None
- **Point-in-time recovery:** Not available
- **Manual backup:** `mongodump` / `mongorestore` only
- **Recommendation for beta:** Daily `mongodump` to object storage (S3, OCI Object Store) via cron job

### M10+ Clusters
- **Automated backups:** Yes (7-35 day retention, configurable)
- **Point-in-time recovery:** 7-day rolling window (default)
- **Backup storage cost:** Included in tier cost

**Verdict:** For beta, manual daily dumps adequate. Post-beta, M10 automated backups worth $57/mo.

---

## 7. TTL Indexes on M0

**Supported:** Yes. TTL indexes with `expireAfterSeconds` fully functional on M0.

Example (replacing Redis SET+TTL presence):
```javascript
// Expire presence records after 5 minutes
db.presence.createIndex(
  { "lastSeen": 1 },
  { expireAfterSeconds: 300 }
)
```

TTL monitor runs every 60 seconds by default. **No M0 restrictions.**

**Verdict:** Direct Redis TTL replacement; no workarounds needed.

---

## 8. Asia Region Availability

**Confirmed available on AWS:**
- Singapore (ap-southeast-1)
- Mumbai (ap-south-1)
- Tokyo (ap-northeast-1)

**Source:** [Cloud Providers and Regions - Atlas](https://www.mongodb.com/docs/atlas/cloud-providers-regions/)

**Latency from OCI Singapore:**
- OCI → AWS Singapore same-region: ~2-5 ms (negligible vs Couchbase self-hosted)

**Verdict:** No region constraints. Deploy in Singapore or Mumbai based on user location.

---

## 9. M0 Production-Acceptability for Beta

### Viable If:
- No change streams requirement
- <1k DAU
- <1-2 GB total data
- Can tolerate 100 ops/sec soft throttle
- Willing to manual-backup daily

### Not Viable If:
- Change streams hard requirement (upgrade to M10)
- Expecting >5k DAU before paid tier (will hit throughput limit)
- Need point-in-time recovery without effort

**Verdict:** **M0 adequate for <1k beta.** Start here, upgrade to M10 if change streams needed or DAU growth exceeds capacity.

---

## Unresolved Questions

1. **Change stream necessity:** Does dleague leaderboard logic currently depend on change streams in Couchbase? If yes, M0 is disqualified immediately.

2. **Coolify IP static or dynamic?** If dynamic egress IPs, need IP range in allowlist or Coolify proxy with static IP.

3. **Backup SLA post-beta:** Will beta SLA demand <1 day RTO/RPO, or is daily dump acceptable?

4. **Growth projection:** If DAU expected to exceed 5k within 3 months, plan M10 budget now.

---

## Sources

- [Atlas Free Cluster Limits - MongoDB Docs](https://www.mongodb.com/docs/atlas/reference/free-shared-limitations/)
- [MongoDB Pricing | MongoDB](https://www.mongodb.com/pricing)
- [Cloud Providers and Regions - Atlas](https://www.mongodb.com/docs/atlas/cloud-providers-regions/)
- [Configure IP Access List Entries - Atlas](https://www.mongodb.com/docs/atlas/security/ip-access-list/)
- [Recover a Point In Time with Continuous Cloud Backup](https://www.mongodb.com/docs/atlas/recover-pit-continuous-cloud-backup/)
- [MongoDB Go Driver Documentation](https://www.mongodb.com/docs/drivers/go/current/)
- [Release Notes - Go Driver](https://www.mongodb.com/docs/drivers/go/current/reference/release-notes/)
- [TTL Indexes - Database Manual](https://www.mongodb.com/docs/manual/core/index-ttl/)
