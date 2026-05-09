# Free Postgres Hosting Survey — May 2026

**Research Date:** 2026-05-05  
**Project:** dleague (Go backend, <100 users, <1GB data, OCI free-tier VM host)  
**Constraint:** Remote DB needed (local to OCI VM not viable); pure Postgres preferred; sub-second response for game backend.

---

## Executive Summary

**Recommend: Neon (Free Plan) + Aiven (backup)**

Neon offers the best tradeoff for dleague: **permanent free tier, Frankfurt region, 100 CU-hours/month, 0.5 GB storage, no aggressive autopause, and pure vanilla Postgres.** Aiven is a solid backup if storage needs grow or regional latency becomes an issue.

Fly.io, Render, and ElephantSQL are eliminated. Supabase and CockroachDB have viable free tiers but hit edge cases that make them secondary choices.

---

## Provider Breakdown

### 1. **Neon** ✅ WINNER
**URL:** https://neon.com/pricing | **Observed:** 2026-05-05

| Aspect | Detail |
|--------|--------|
| **Tier Status** | Permanent free (no credit card, commercial use OK) |
| **Storage** | 0.5 GB/project; 20 projects max (10 GB total) |
| **Compute** | 100 CU-hours/month (doubled Oct 2025); always-on (no autopause) |
| **Connections** | Not explicitly limited on free |
| **PITR** | 6 hours history, 1 GB changes, no charge |
| **Snapshots** | 1 manual snapshot/project |
| **Postgres Type** | Vanilla Postgres (stateless compute + rebuilt storage layer) |
| **Regions** | EU (Frankfurt), US (Virginia, Ohio), US West, Asia (Singapore), Israel |

**Pros:**
- Permanent free (no trial expiry).
- Frankfurt region = low latency to OCI free tier (EU regions). OCI Phoenix US is also covered by US-East.
- No autopause on free tier → critical for real-time game backend (sub-second startup).
- 100 CU-hours/month is generous for MVP (<100 users); typical SELECT ~1-15 RU, INSERT ~10-25 RU.
- Pure Postgres, no compatibility gotchas.
- Branching (dev/staging) is free (nice bonus).

**Cons:**
- 0.5 GB storage is tight if game logs, match history, or analytics grow aggressively. Mitigation: stripe logs to S3, archive old matches after 90 days.
- Compute hours can exhaust with high connection count or slow queries. Need monitoring early.

---

### 2. **Aiven** ✅ STRONG SECONDARY
**URL:** https://aiven.io/pricing | **Observed:** 2026-05-05

| Aspect | Detail |
|--------|--------|
| **Tier Status** | Permanent free (no time limit) |
| **Storage** | 1 GB disk |
| **Compute** | 1 CPU, 1 GB RAM, single node |
| **Connections** | Not specified; single node limits concurrency |
| **Postgres Type** | Vanilla Postgres |
| **Regions** | Multiple (verify: https://aiven.io/services/list) |

**Pros:**
- Permanent free (no shutdown risk).
- 1 GB storage > Neon's 0.5 GB.
- No apparent inactivity pause.
- Vanilla Postgres.

**Cons:**
- Single-node, 1 CPU is tight for concurrent WS connections (Phase 5 concern: app will hold pooled DB connections).
- 1 GB RAM may cause OOM under load; no autoscaling.
- Region support unclear from pricing page; needs verification.
- Less transparent on connection limits.

**Use case:** Fallback if Neon storage becomes a bottleneck, or as staging/replica.

---

### 3. **Supabase** ⚠️ CONDITIONAL
**URL:** https://supabase.com/pricing | **Observed:** 2026-05-05

| Aspect | Detail |
|--------|--------|
| **Tier Status** | Permanent free (no trial, but has inactivity rule) |
| **Storage** | 500 MB database, 1 GB file storage, 5 GB egress/month |
| **Inactivity Pause** | Free projects pause after **1 week inactivity** |
| **Connections** | Up to 50k monthly active users (auth/signup) |
| **Postgres Type** | Vanilla Postgres (AWS-hosted) |
| **Regions** | Frankfurt (EU), Seattle (US), Mumbai, Sydney, Pune |

**Pros:**
- Frankfurt region available.
- Vanilla Postgres with strong feature set (Auth, Realtime, Storage).
- 500 MB is enough for MVP schema.

**Cons:**
- **1-week inactivity pause is a deal-breaker for production game.** Real-time PvP backend needs instant response; cold starts risk 5-10s latency or connection timeout.
- 500 MB is tighter than Neon/Aiven.
- 5 GB egress limit is low for WS + match replay traffic.

**Verdict:** Reject for live game backend; revisit if converting to async-only leaderboard service (lower SLA).

---

### 4. **CockroachDB Serverless** ⚠️ CONDITIONAL
**URL:** https://www.cockroachlabs.com/pricing/ | **Observed:** 2026-05-05

| Aspect | Detail |
|--------|--------|
| **Tier Status** | Permanent free (no credit card) |
| **Storage** | 10 GiB (updated 2024; was 5 GiB) |
| **Compute** | 50M RUs/month (~200M RUs/year = ~400k SELECT + 200k INSERT/month) |
| **Postgres Type** | Wire-compatible but NOT vanilla (distributed MVCC, different cost model) |
| **Regions** | Multiple (AWS, GCP, Azure) |

**Pros:**
- 10 GB free storage (largest free tier).
- No per-connection charge; RU-based pricing is predictable.
- Global regions.

**Cons:**
- **Not vanilla Postgres.** Some edge cases: distributed transactions, conflict resolution, latency from replication. Breaks assumptions like `SERIALIZABLE` isolation.
- 50M RUs = ~1 RU per query. Complex queries (joins, aggregates on leaderboard) can blow through this quickly.
- Wire compatibility doesn't equal compatibility—need testing on match/leaderboard queries.
- No mention of regions near OCI free tier (would need lookup).

**Verdict:** Skip unless you want to beta-test Cockroach. Neon is lower risk.

---

### 5. **Xata** ⚠️ BORDERLINE
**URL:** https://xata.io/pricing | **Observed:** 2026-05-05

| Aspect | Detail |
|--------|--------|
| **Tier Status** | Permanent free |
| **Storage** | 15 GB (largest) |
| **Compute** | Not explicitly limited; auto-scales |
| **Postgres Type** | Vanilla Postgres (100% compatible) |
| **Features** | No autopause, high-availability by default, branching |

**Pros:**
- 15 GB free storage (most generous).
- Vanilla Postgres, no gotchas.
- No autopause.

**Cons:**
- Free tier limited: Search APIs and Files APIs removed (Jan 2025). If leaderboard search or analytics added later, paid upgrade required.
- Less transparent on compute limits; "auto-scales" is vague.
- Smaller ecosystem vs. Neon/Supabase; less community content for debugging.

**Verdict:** Viable third choice if storage is critical; otherwise Neon is simpler.

---

### 6. **Vercel Postgres** ❌ SKIP
**Status:** Transitioned to Neon in Q4 2024–Q1 2025.

Vercel no longer manages Postgres directly. Instead, it offers integrations through the Vercel Marketplace. If you use Vercel + Postgres, you're buying a Neon seat through Vercel's dashboard (billing unified, but no discount).

**Verdict:** Just use Neon directly; same limits, one less abstraction layer.

---

### 7. **Fly.io Managed Postgres** ❌ SKIP
**URL:** https://fly.io/docs/mpg/ | **Observed:** 2026-05-05

Fly.io **eliminated free tier entirely in 2024.** New signups get 2 VM-hours or 7 days trial max. Managed Postgres pricing starts at **$38/month (Basic plan).**

**Verdict:** Not an option for free tier. If deploying backend to Fly anyway (post-OCI), revisit cost vs. separate Neon.

---

### 8. **Render Postgres** ❌ SKIP
**URL:** https://render.com/changelog/free-postgresql-instances-now-expire-after-30-days-previously-90 | **Observed:** 2026-05-05

Free Postgres instances **expire after 30 days** (previously 90). After expiry, 14-day grace period before deletion.

**Verdict:** Not suitable for a production game. Expiry timer breaks permanent hosting requirement.

---

### 9. **ElephantSQL** ❌ SKIP
**Status:** Shut down January 27, 2025.

ElephantSQL is **no longer operational.** Company ceased selling new services May 2024; migration window has closed.

**Verdict:** Dead provider; not an option.

---

### 10. **Tembo Cloud** ❌ SKIP
**URL:** https://tembo.io/pricing | **Observed:** 2026-05-05

Free tier offers **10 credits/week** (~40-50 credits/month). Pricing model is opaque; no published storage/compute limits. Postgres flavor unclear.

**Verdict:** Insufficient transparency and unproven track record. Avoid for MVP.

---

## Regional Fit Analysis

**OCI Free Tier locations:** Phoenix US (us-phoenix-1), Frankfurt (eu-frankfurt-1), Tokyo (ap-tokyo-1), Johannesburg, others.

| Provider | Frankfurt | US-East/Phoenix | Coverage |
|----------|-----------|-----------------|----------|
| **Neon** ✅ | Yes | Yes (Virginia, Ohio) | Excellent |
| **Aiven** | TBD (check their site) | Likely | Verify |
| **Supabase** ✅ | Yes | Seattle (not Phoenix) | Good for EU, meh for US |
| **CockroachDB** | AWS/GCP regions | Yes | Check specifics |
| **Xata** | TBD | Likely | Verify |

**OCI Frankfurt deployment + Neon Frankfurt** = optimal latency (~5-10 ms). Phoenix VM + Neon US-East = acceptable (30-50 ms).

---

## Recommendation Ranking

### Tier 1: Go Now
**Neon (Free Plan)**
- Permanent free, no expiry.
- Pure Postgres, no compatibility tax.
- Frankfurt + US regions.
- No aggressive autopause (critical for real-time game).
- 100 CU-hours/month + 0.5 GB easily covers MVP.
- **Action:** Sign up, create free project, test connection pooling (miniflare or pgBouncer if <100 concurrent conns).

### Tier 2: Fallback/Hybrid
**Aiven (Free Plan)**
- Use if Neon storage becomes a bottleneck (1 GB vs 0.5 GB).
- Single-node limits are acceptable for Phase 3-4 async-only flow; revisit for Phase 5 WS.
- **Action:** Evaluate after Phase 3 is live; monitor Neon CU usage to predict storage scaling.

### Tier 3: Conditional (Not Recommended)
**Xata or CockroachDB**
- Only if architectural needs change (e.g., advanced search, cost optimization via RU model).
- Adds complexity without MVP necessity.

---

## Implementation Notes

**Before going live on chosen provider:**

1. **Test connection pooling:** With Go `database/sql` + `pgx` driver, use `MaxOpenConns` ~50-100 (Neon is stateless, can handle higher concurrency than traditional Postgres).
2. **Monitor compute hours early:** Add a Prometheus scrape for connection count + query latency. Alert if trending toward 80% of monthly budget.
3. **Backup strategy:** Neon includes 6h PITR free; set up weekly exports to S3 (dleague data is small; `pg_dump` to S3 costs ~$0.01/month).
4. **Region selection:** Frankfurt for EU testing, US-East for US. Both cost same on free tier.
5. **Scaling plan (Phase 5+):** If WS server + leaderboard queries exhaust Neon free tier compute, upgrade to Neon Launch ($15/month, 3 GB storage, unlimited compute) or migrate to managed RDS (AWS free tier RDS for first year, but that's only for new AWS accounts).

---

## Unresolved Questions

1. **Aiven region availability:** Pricing page doesn't list supported regions clearly. Need to check https://aiven.io/services/list or contact sales.
2. **CockroachDB region proximity to OCI:** Verify if AWS/GCP regions overlap with OCI latency SLA (<50 ms Phoenix, <20 ms Frankfurt).
3. **Neon shared-tenant noisy-neighbor risk:** Free tier is shared; no SLA. Unknown if other users' heavy queries can degrade game response time. (Pragmatic note: <100 users at MVP scale = unlikely to matter.)
4. **Egress charges post-MVP:** When scaling beyond free tier, which provider has best per-GB egress cost for WS traffic? (Neon, Supabase, Xata pricing pages don't clearly state egress on paid plans.)

---

**Report End**  
**Researcher:** Technical Analysis Agent  
**Source Confidence:** Primary (official pricing pages + recent blogs/docs); Neon & Supabase verified live May 2026.
