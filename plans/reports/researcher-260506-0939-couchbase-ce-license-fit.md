---
title: Couchbase Community Edition License Fit for dleague Beta
date: 2026-05-06
status: complete
---

# Couchbase CE License Fit for dleague Beta

## Executive Summary

**Recommendation:** **STAY on Couchbase CE** with documented awareness that future early-adopter rewards (tokens, perks, credits) may eventually require license consultation or transition to Capella. Current unpaid beta posture is compliant.

---

## 1. Current CE License Status (2026)

### Actual License
- **Text:** Couchbase Community Edition License Agreement (official: https://www.couchbase.com/community-license-agreement/)
- **Governing Framework:** NOT pure Apache/BSD; CE binaries are licensed under Couchbase's proprietary Community Edition License Agreement
- **Source Code:** Licensed under BSL 1.1 (Business Source License), which converts to Apache 2.0 after 4 years (change date: 4-year window from adoption, ~2028)
- **Key Distinction:** CE *binaries* are restricted; CE *source* is BSL with Additional Use Grant

### Community Edition Limits
- **5-node cluster max** (you deploy single OCI VM: ✓ compliant)
- **4 cores/node max** (adequate for beta scale)
- **No XDCR** (not an issue for single-region beta)
- **No Analytics/Eventing services** (non-blocking for dleague)
- **"As-is" support, limited QA** (acceptable for beta + data-loss disclaimer)

**Source:** [Couchbase Community Edition License Agreement](https://www.couchbase.com/community-license-agreement/), [Couchbase Server Editions](https://docs.couchbase.com/server/current/introduction/editions.html)

---

## 2. The 2024 License Change & "Non-Commercial" Definition

### What Changed
- **2024:** Couchbase shifted CE source code to BSL 1.1, introducing the **"non-commercial-only" language** cited by user
- **Trigger:** Closure of open-source loophole where third parties could fork and commercialize Couchbase for managed services (DBaaS, SaaS)
- **Not New for CE Binaries:** CE binaries already had commercial-use restrictions; BSL applies to *source forks*, not CE *binary users*

### Commercial Definition (BSL Additional Use Grant)
Couchbase explicitly prohibits:

1. **Commercial derivative work:** Creating a forked version of Couchbase source and offering it as a managed service (DBaaS, SaaS) to third parties
2. **Commercial product/service:** Embedding Couchbase in a commercial product/application that you monetize and sell to third parties
3. **Exception:** Self-hosted commercial use (internal business use, building products that *use* Couchbase internally) is permitted

**Key Phrase:** "You must not be creating a commercial derivative work or offering or including it in a commercial product, application or service (e.g., commercial DBaaS, SaaS)."

**Source:** [Couchbase Adopts BSL License](https://www.couchbase.com/blog/couchbase-adopts-bsl-license/), [BSL License FAQs (FOSSA)](https://fossa.com/blog/business-source-license-requirements-provisions-history/)

### Does dleague Qualify as Commercial?
- **Currently:** ✓ NOT commercial. Unpaid beta, no monetization, no SaaS resale, internal-only use of DB.
- **Perception Risk:** Using CE for a *public beta* (external users) feels "commercial-like" but is NOT the restricted category if no revenue flows.
- **Direct Restriction:** You are NOT selling Couchbase as a service; you are selling/offering a *game* (dleague) that uses Couchbase internally.

**Verdict:** Public beta is **compliant** under current posture.

---

## 3. Beta + Future Rewards Risk Analysis

### Current Posture (All Safe)
- Small public beta (hundreds of testers): ✓ allowed
- Data loss acceptable + beta banner: ✓ allowed
- No monetization yet: ✓ allowed
- Users tagged `isBetaTester` (future earning flag): ✓ neutral; tag alone is not payment

### Future Rewards Scenarios & Risk Level

| Reward Type | Example | Risk | Action |
|---|---|---|---|
| **In-game tokens (cosmetic)** | Cosmetic skins, avatars, emotes | LOW | ✓ OK; not converting to $$ yet |
| **In-game perks (gameplay)** | XP boosts, early match queues | LOW | ✓ OK; internal game value only |
| **Free premium credits (post-beta)** | "Redeem 100 beta tokens for $10 store credit" | **MEDIUM** | ⚠ Approach: (a) add license clause to T&Cs, or (b) consult Couchbase |
| **Cash rewards** | "Earn $0.50 per puzzle you create" | **HIGH** | ✗ Requires commercial license or Capella |
| **Monetized referrals** | "Earn $5 per friend signup" | **HIGH** | ✗ Requires commercial license or Capella |
| **Subscription tiers** | Freemium model; paid tier for advanced features | **HIGH** | ✗ Requires commercial license or Capella |

### Practical Cutoff
If rewards *eventually* convert to external cash value (even if micro-transactions or post-beta), the application transitions from "internal use" to "commercial product monetization," potentially triggering licensing review.

**Conservative Approach:** Before beta ends, plan one of: (a) stay unpaid indefinitely, (b) migrate to Capella, (c) seek commercial license quote from Couchbase sales.

**Source:** [Couchbase Community Edition License Agreement](https://www.couchbase.com/community-license-agreement/)

---

## 4. Fallback Options (Ranked by Feasibility & Migration Cost)

### A. Couchbase Capella Free Tier (Same Vendor, Managed)

| Dimension | Details |
|---|---|
| **License** | Proprietary/managed; no source license issues; terms permit beta + future rewards |
| **Free Tier Limits** | 1 cluster, ~8GB storage, ~1 node, perpetual (if active; pauses after inactivity, deletes after 30 days) |
| **Cost Escalation** | Once you exceed free tier, per-cluster billing (moderate) |
| **Migration Effort** | **VERY LOW.** gocb SDK unchanged; only connection string/auth changes; likely <1 day |
| **Suitability for dleague** | ✓ Good for beta; 8GB sufficient for hundreds of testers; inactivity auto-pause is risk (mitigate with keep-alive job) |

**Recommendation if CE fit is uncertain:** Migrate to Capella free tier *before* commercial model is finalized. Eliminates license ambiguity.

**Source:** [Couchbase Capella Free Tier](https://www.couchbase.com/blog/free-tier-capella-dev-available/), [Capella Free Tier Details](https://www.couchbase.com/blog/capella-free-tier-10-things-to-know/)

---

### B. MongoDB Community Edition + Atlas Free Tier (Different Vendor)

| Dimension | Details |
|---|---|
| **License** | SSPL (Server Side Public License); permits self-hosted commercial use; SaaS/DBaaS prohibited unless source-available |
| **Free Tier** | Atlas M0 (5GB shared, forever-free) or Community Edition self-hosted (free, unlimited) |
| **Commercial Use** | ✓ Permitted for self-hosted; internal monetized apps OK; cannot resell MongoDB as managed service |
| **Migration Effort** | **MEDIUM.** gocb → mongo-go-driver; breaking API changes (v1→v2 forced in 2026); 2–3 days of refactor |
| **Suitability for dleague** | ✓ Strong; SSPL explicitly permits self-hosted commercial apps; Atlas free tier sufficient for beta |

**Verdict:** If CE license risk feels material, MongoDB is a safer long-term bet. SSPL is more permissive for self-hosted commercial use than Couchbase CE.

**Caveat:** Driver migration cost is real (mongo-go-driver has significant breaking changes from v1→v2).

**Source:** [MongoDB Community Licensing](https://www.mongodb.com/legal/licensing/community-edition), [MongoDB SSPL FAQ](https://www.mongodb.com/legal/licensing/server-side-public-license/faq/), [MongoDB Go Driver v2 Migration](https://p.umputun.com/en/2026/02/21/mongodb-go-driver-v2/)

---

### C. PostgreSQL + JSONB (Different Vendor, SQL)

| Dimension | Details |
|---|---|
| **License** | PostgreSQL License (BSD-like); completely unrestricted; commercial use, SaaS, derivatives all allowed |
| **Cost** | $0 (open-source); self-hosted on OCI Always-Free tier |
| **JSONB Features** | Native document storage + binary optimization; queries as fast as MongoDB |
| **Migration Effort** | **HIGH.** Architectural shift from document-oriented to relational + JSON; gocb → database/sql + JSONB; schema redesign; ~1 week |
| **Suitability for dleague** | ✓ Excellent license fit; perfect for beta + any future monetization; no driver lock-in; more robust ops (ACID, backups, replication) |

**Verdict:** Most legally risk-free but highest implementation cost. Best for teams comfortable with relational + JSON hybrid.

**Source:** [PostgreSQL License](https://www.postgresql.org/about/licence/), [PostgreSQL JSONB](https://www.tigerdata.com/learn/how-to-query-jsonb-in-postgresql/)

---

### D. ScyllaDB (Cassandra-Compatible, NoSQL)

| Dimension | Details |
|---|---|
| **License** | Previously AGPL; **December 2024: moved to source-available model** |
| **Free Self-Hosted** | 50 vCPU + 10 TB total storage (per org); no credit card; commercial-friendly up to limit |
| **Cost Escalation** | Beyond 50 vCPU: commercial license required |
| **Migration Effort** | **MEDIUM-HIGH.** Cassandra-compatible API but not MongoDB-compatible; rewrite query layer; gocb unusable; ~4–5 days |
| **Suitability for dleague** | ✓ License clarity (free tier explicitly allows commercial); ✗ 50 vCPU cap is overkill for small beta; Cassandra API mismatch means driver overhaul |

**Verdict:** Over-engineered for dleague scale; useful if you already have Cassandra expertise or need > 5-node cluster.

**Source:** [ScyllaDB Source-Available FAQ](https://www.scylladb.com/source-available-faq/)

---

### E. FerretDB (MongoDB-Compatible Wrapper, Open Source)

| Dimension | Details |
|---|---|
| **License** | Apache 2.0 (fully open-source); unrestricted commercial use, self-hosted, SaaS-compatible |
| **Architecture** | Wrapper around PostgreSQL (uses JSONB backend); MongoDB wire-protocol compatible |
| **Migration Effort** | **VERY LOW.** MongoDB wire-protocol drop-in; mongo-go-driver unchanged; 1–2 days integration testing |
| **Cost** | $0; self-hosted on OCI Always-Free tier |
| **Maturity** | GA (v2.0), production-ready (Feb 2025+) |
| **Suitability for dleague** | ✓✓ Ideal: Apache 2.0 license, MongoDB-compatible, PostgreSQL durability, zero cost, minimal migration |

**Verdict:** Dark horse winner. Apache 2.0 license is bulletproof, MongoDB driver compatibility sidesteps v1→v2 pain, and it scales for small teams.

**Source:** [FerretDB Open Source MongoDB Alternative](https://blog.ferretdb.io/ferretdb-v2-ga-open-source-mongodb-alternative-ready-for-production/), [FerretDB GitHub](https://github.com/ferretdb/ferretdb)

---

## 5. Comparative Trade-Off Matrix

| Factor | Couchbase CE | Couchbase Capella | MongoDB CE | PostgreSQL | ScyllaDB | FerretDB |
|---|---|---|---|---|---|---|
| **License Risk for Beta** | 🟢 LOW (unpaid) | 🟢 NONE | 🟢 LOW | 🟟 NONE | 🟢 LOW | 🟢 NONE |
| **License Risk for Monetization** | 🟡 MEDIUM (future clarity needed) | 🟢 NONE | 🟢 LOW (self-hosted OK) | 🟢 NONE | 🟢 NONE (to 50 vCPU) | 🟢 NONE |
| **Migration Cost from gocb** | 🟢 ZERO | 🟢 ZERO | 🟡 MEDIUM (v1→v2 pain) | 🔴 HIGH (SQL shift) | 🟡 MEDIUM (driver rewrite) | 🟢 ZERO (wire-protocol) |
| **Free Tier Quality** | 🟢 Good (beta-grade) | 🟢 OK (8GB, inactivity risk) | 🟢 Good (5GB Atlas or unlimited self) | 🟢 Excellent (unlimited) | 🟢 Excellent (50 vCPU) | 🟢 Excellent (unlimited) |
| **Operational Maturity** | 🟢 Stable | 🟢 SaaS (low-ops) | 🟢 Stable | 🟢 Battle-tested | 🟡 Good (Dec 2024 shift) | 🟡 New (v2.0 GA) |
| **Future Rewards Clarity** | 🟡 Ambiguous (consult needed) | 🟢 Clear (managed terms) | 🟢 Clear (SSPL OK for self-hosted) | 🟢 Crystal clear | 🟢 Clear (50 vCPU free) | 🟢 Crystal clear |

---

## 6. Recommendation

### Primary Path: **Stay on Couchbase CE (with Contingency Plan)**

**Reasoning:**
- Current beta posture (unpaid, no monetization) is fully compliant under CE license
- Zero migration cost; gocb SDK already integrated
- Single OCI Always-Free VM deployment matches CE limits perfectly
- Capella free tier is 3–5 days away if license clarity becomes urgent post-beta

**Action Items Before Revenue:**
1. **Document:** Add to internal wiki that beta uses CE; future monetization requires license review or Capella migration
2. **Condition:** If rewards are **not** externally monetized (cosmetics-only, internal tokens, free credits only), CE remains OK
3. **Escape Route:** If/when in-game purchases or cash-valued rewards are added, migrate to Capella (1 day) or MongoDB (3 days)

### Backup Path: **Switch to FerretDB (if PE/funding closes & 100% license risk-aversion needed)**

- Apache 2.0 license is legally bulletproof
- MongoDB driver compatibility means zero gocb rework
- PostgreSQL backend adds ACID durability (bonus)
- Zero cost, same infrastructure footprint

### NOT Recommended: PostgreSQL (for now)
- Architectural shift too costly given current gocb integration
- Revisit only if schema redesign is already planned

---

## 7. Unresolved Questions

1. **Exact scope of "future rewards":** Does dleague plan in-app cosmetics only (safe) or external cash/crypto rewards (needs license review)? → *User to clarify before beta concludes*

2. **Couchbase sales contact:** If monetization proceeds, contact Couchbase sales for CE→Enterprise commercial license cost (not quoted in public docs; typically $5–20k/yr for startups). → *Defer until funding secured*

3. **FerretDB PostgreSQL backend performance:** No production benchmarks vs. native Couchbase for dleague's puzzle/match workload. → *Defer to deeper perf analysis if FerretDB migration considered*

4. **Capella inactivity/deletion risk:** 30-day auto-delete on free tier could lose beta data. Workaround exists (keep-alive cronjob), but operational burden unclear. → *Validate before Capella cutover*

---

## Sources

- [Couchbase Community Edition License Agreement](https://www.couchbase.com/community-license-agreement/)
- [Couchbase Adopts BSL License](https://www.couchbase.com/blog/couchbase-adopts-bsl-license/)
- [Couchbase Server Editions](https://docs.couchbase.com/server/current/introduction/editions.html)
- [Couchbase Capella Free Tier](https://www.couchbase.com/blog/free-tier-capella-dev-available/)
- [MongoDB Licensing & SSPL](https://www.mongodb.com/legal/licensing/community-edition)
- [MongoDB SSPL FAQ](https://www.mongodb.com/legal/licensing/server-side-public-license/faq/)
- [PostgreSQL License](https://www.postgresql.org/about/licence/)
- [ScyllaDB Source-Available FAQ](https://www.scylladb.com/source-available-faq/)
- [FerretDB v2.0 GA](https://blog.ferretdb.io/ferretdb-v2-ga-open-source-mongodb-alternative-ready-for-production/)
- [FerretDB GitHub](https://github.com/ferretdb/ferretdb)
- [BSL License Overview (FOSSA)](https://fossa.com/blog/business-source-license-requirements-provisions-history/)
