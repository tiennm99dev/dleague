# Firebase as Preview-Stage Data Store + Scale-Up Path

**Research Date:** 2026-05-05 15:18 (Asia/Saigon)
**Scope:** Firebase fitness for dleague at testing AND scale, gaps the prior report (`researcher-260505-1407-firebase-as-backend-feasibility.md`) didn't fill.
**Decision being supported:** stay on MySQL HeatWave vs pivot all-Firebase data plane.

---

## TL;DR — **Conditional YES**

**For dleague's testing stage (≤100 DAU, zero budget):** Firebase is a *good* choice. Free tier covers it 100×. Operational overhead near zero. Auth is already locked-in.

**For the scale-up path (1K → 100K DAU):** Firebase is a *defensible* choice up to ~10K DAU at modest cost ($30–60/mo) **if you architect the data model defensively from day 1**. Beyond that it gets expensive and you'll likely want out — and the prior report's "migration = 1 day effort" claim is **wrong**: realistic is **2–6 weeks** for a project dleague's size, longer if denormalization is deep.

**My pick if I were you:** Use **Firebase Auth** (free, mature, JWT compatible everywhere) **+ Supabase Postgres** for data. Same testing-friendliness as full-Firebase, but data layer is portable Postgres from day 1 — no migration ever needed if you scale, just bigger Postgres.

**Pure-Firebase pivot is acceptable** if you want one vendor + zero ops, accept lock-in, and commit to a defensive data-model discipline (no hot docs, sharded counters, flat collections).

**Pure MySQL HeatWave (status quo)** is also defensible but ignores user's explicit ask for "no spend, free tier, good SLA, 3rd party managed" — and HeatWave Always-Free is OCI-managed, which is the same lock-in risk in a different jersey.

---

## 1. Real Cost Curve (1K / 10K / 100K DAU)

dleague workload assumption: ~3 Firestore ops/user/day (1 puzzle read + 1 attempt write + 1 leaderboard read), session-batched listeners, no per-keystroke writes.

| DAU | Monthly Reads | Monthly Writes | Free-Tier Headroom | Monthly Bill (Blaze) |
|-----|---------------|----------------|--------------------|-----------------------|
| 100 | 9 K | 3 K | 100× headroom | **$0** |
| 1,000 | 90 K | 30 K | ~10× headroom | **~$0–2** (under daily free quota all 30 days) |
| 10,000 | 900 K | 300 K | exceeds daily free | **~$30–60** (reads dominate) |
| 100,000 | 9 M | 3 M | far over | **~$200–500** typical, **$300–2000** worst-case |

**Reality check from external sources:** indie consumer apps at 100K DAU report **$298/mo** typical, but **$90–270/day** if screens require 5–15 reads to render. Wordle-style is read-light, so dleague would land near the optimistic end.

**Where this breaks vs MySQL HeatWave:** HeatWave Always-Free has unlimited query ops; Firestore charges per op forever. At 10K DAU, HeatWave is still $0; Firestore is $30–60. At 100K DAU, HeatWave is ~$50–150 paid tier; Firestore is $200–500. **Cost crossover ≈ 10K DAU.**

---

## 2. Firestore Scale Traps (the part you'll hit before the bill)

### 2.1 The 1-write-per-second-per-document limit (real)
- **Soft limit:** 1 write/sec sustained per document. Bursts higher OK.
- **Hard impact at scale:** hotspot → contention errors, latency spike, failed writes.
- **dleague risk surface:**
  - ✅ **Per-match doc (async PvP):** safe — each match is its own doc, low write rate.
  - ⚠️ **Sync PvP** with sub-second turn cadence: hotspot risk on shared match doc. Turn-based ≥1s/turn is fine.
  - 🔴 **Global leaderboard counter / "total games played" gauge:** **will hotspot** at >100 concurrent players. Mitigation: **sharded counter pattern** (10 shards = 10 writes/sec, sum on read). Adds complexity.
  - 🔴 **Daily puzzle attempt counter:** same hotspot. Same mitigation.

### 2.2 The "555 rule" for traffic ramps
- Start ≤500 ops/sec, increase ≤50% every 5 minutes. Past that, expect throttling.
- **dleague reality:** you won't hit 500 ops/sec until ~50K concurrent users. Not a near-term concern, but launch-day spike (e.g., HN front page → 5K users in 10 min) **could** trip this.

### 2.3 Real-time listeners: cheap to maintain, billed on document returns
- Open connection: free. Each doc that crosses the listener boundary: 1 read billed.
- A leaderboard listener returning top-100 every score change = 100 reads per change × N watchers. **A top-100 leaderboard with 1K watchers and 1 score-update/sec = 360M reads/month = ~$200/mo** just for that one feature.
- **Mitigation:** server-side aggregated leaderboard doc, polled (not subscribed). Or denormalize "top 10 only" client-side, full leaderboard on demand.

### 2.4 500 ops per transaction
- A transaction can touch ≤500 docs. Cross-document ACID at scale → forced into batched-writes + idempotency keys.
- **dleague impact:** essentially none. Per-match transactions are 2–5 doc operations.

### 2.5 Composite indexes
- Each unique `WHERE` + `ORDER BY` combo needs a manually-declared composite index.
- Stored size = your data size × index count. **Index storage is billed.**
- Easy to forget; adding indexes after a feature ships requires a backfill that takes minutes-to-hours and consumes write quota.

---

## 3. Migration-Off Realism (correcting the prior report)

The prior report claimed "1 day effort, 2–3 hour downtime." **This is unrealistic.** External evidence:

| Story | Project Size | Real Effort |
|-------|--------------|-------------|
| Functional Firestore→PG migration (Mouret) | small SaaS | **~3 weekends** |
| Traba | production | **~1 year** to fully deprecate |
| AICU Life | Firebase → Go + PG + MinIO | **months** |
| Generic dev community average | small/medium | **2–6 weeks** |

### Why it's never 1 day:
1. **Denormalized docs → relational schema.** "Just dump to jsonb" works as a holding pattern but doesn't unlock relational benefits. Real normalization = manual schema design per collection. **2–5 days alone.**
2. **Security rules → server-side authz.** Firestore rules live in client trust boundary. Postgres needs RLS or server-enforced checks. **Rewrite, not port. ~1 week** for non-trivial rules.
3. **Client SDK rewrite.** `firestore.collection().where().onSnapshot()` becomes HTTP polling or Postgres LISTEN/NOTIFY or Supabase Realtime. **Every read/write site touched.**
4. **Index re-declaration.** Postgres indexes are different shape; some Firestore queries don't have efficient Postgres equivalents (e.g. array-contains-any with ordering).
5. **Live cutover with dual-write window.** Lest you lose data: instrument dual-write, verify parity, cut reads, then cut writes. **1–2 weeks.**
6. **Realtime listeners → polling or Realtime equivalent.** Architectural rethink in the client.

### Cheapest exit: hybrid jsonb mirror
- Postgres table with `(doc_id, data jsonb)` columns mirrors Firestore literally.
- Defer normalization indefinitely. Costs you query-flexibility and storage size, but bridges in **~1 week** rather than **~1 month**.
- For dleague, this is the realistic exit ramp: ~1 week migration if scale forces a move, with normalization as ongoing tech debt.

**Bottom line:** budget **2 weeks minimum** for migration off Firebase if you ever leave. Not 1 day. Plan accordingly.

---

## 4. Free-Tier-Friendly Alternatives (with scale path)

### 4.1 Quick comparison (May 2026)

| Platform | Free Tier | Go SDK | Realtime? | Migration Cost from Firebase | Lock-in |
|----------|-----------|--------|-----------|-------------------------------|---------|
| **Firebase** (status quo option) | 50K reads/20K writes/day, 1 GiB, 50K MAU auth | Admin SDK, official | yes (listeners) | N/A | **HIGH** |
| **Supabase** | 500 MB Postgres, 50K MAU, 1 GB storage, 2M edge fn invocations | `supabase-go` (community), or `pgx` direct | yes (Postgres LISTEN/NOTIFY + WebSockets) | **medium** (auth still Firebase, data → PG) | **LOW** (it's just Postgres) |
| **Neon** | 0.5 GB/project × 100 projects, scale-to-zero | `pgx` or any PG driver | no (use polling or pgmq) | medium | **LOW** |
| **Turso** | **5 GB**, 500M row reads/month, 100 DBs | `libsql-client-go` | no (replication, not push) | high (SQLite, not PG) | **MEDIUM** (libSQL is OSS but ecosystem thin) |
| **Convex** | 1M function calls/month, 1 GB, realtime included | **no Go SDK** (TS only) | yes (built-in) | high (rewrite, no Go) | **HIGH** |
| **PlanetScale** | **GONE** (removed free tier 2024) | n/a | n/a | n/a | n/a |
| **Cloudflare D1** | 5 GB, 5M reads/day, 100K writes/day | HTTP API | no | high (SQLite-flavored) | **MEDIUM** |

### 4.2 What this means for dleague

**Best fit for dleague's profile (Go server, web+mobile client, daily puzzle, leaderboards, PvP):**

1. **Supabase** — the closest "Firebase replacement" with Postgres underneath. Free tier covers ≤100 DAU easily. Built-in Auth (compatible with Firebase Auth via JWT bridge), Realtime, Storage. Go usage is via `pgx` directly — no SDK needed, just SQL. **No migration ever** because at scale you just upgrade Postgres tier.

2. **Firebase Auth + Supabase Postgres (hybrid)** — keep Firebase Auth (already decided), use Supabase for data. Best of both: Google-managed auth, portable Postgres data. Slight integration friction (Firebase JWT verified server-side, then Supabase RLS keyed by Firebase UID claim).

3. **Neon** — purer Postgres, scale-to-zero (cheap idle), 100 free projects (can use one per environment). No realtime — you'd build PvP sync on your existing Go WebSocket layer (which dleague already has). **Most aligned with dleague's existing architecture.**

4. **Turso** — extremely generous for read-heavy. Wordle-style daily puzzle is read-heavy. But SQLite semantics (no advanced concurrent writes until late 2025; concurrent writes still limited) make sync PvP awkward.

5. **Full Firebase** — works, but you're paying lock-in tax for "data plane" benefits you don't strongly need (your existing WS layer already does realtime fine).

---

## 5. Indie Pattern — What Successful Projects Actually Do

The search yielded no clean "indie game studio" 2024–2026 case study with regret narrative, but the SaaS pattern is consistent:

1. **Prototype on Firebase** — 1–6 months, free tier, fast iteration.
2. **Hit the cost wall around 5K–20K DAU** — Firestore reads dominate the bill.
3. **Migrate to Postgres** (usually Supabase) — takes 2–6 weeks.
4. **Stay on Postgres forever** — last platform they need.

**The teams that don't regret Firebase:**
- Stayed small (≤1K DAU long-term).
- Or migrated *early* (before deep denormalization).
- Or used Firebase **Auth-only** + their own data layer.

**The teams that regret it:**
- Scaled past 10K DAU on Firestore without sharded counters → hotspots.
- Built deep relational features on top of denormalized docs → migration nightmare.
- Used real-time listeners as the primary read path → bill explosion.

**dleague's risk profile in this taxonomy:**
- Wordle daily puzzle = naturally read-heavy → Firebase reads cost dominates.
- Leaderboards = hotspot risk → must shard from day 1.
- PvP = each match own doc = safe.
- → **medium risk on Firebase**, **low risk on Postgres-anything**.

---

## 6. Recommendation Matrix

| Option | Testing (≤100 DAU) | 1–10K DAU | 10–100K DAU | Lock-in | Migration cost if you leave |
|--------|---------------------|-----------|-------------|---------|------------------------------|
| **Stay MySQL HeatWave** (prior report's pick) | $0, more headroom | $0 | $50–150 paid tier | OCI-bound | low (standard MySQL) |
| **Full Firebase** (the user's instinct) | $0, zero ops | $2–60 | $200–500+ | **HIGH** | **2–6 weeks** |
| **Supabase data + Firebase Auth** ⭐ my pick | $0, zero ops | $0–25 (Pro) | $25–75 | low | trivial (it's Postgres) |
| **Neon + Firebase Auth + your existing WS** | $0, scale-to-zero | $0–19 | $19–69 | low | trivial |

**My pick: Supabase data + Firebase Auth.** Reasons:
1. Matches user's "free tier, good SLA, 3rd-party managed" criteria (Supabase is SOC 2, EU/US regions, managed Postgres).
2. Same prototyping speed as full-Firebase.
3. Postgres = no migration ever.
4. Firebase Auth gives you the all-providers thing you wanted; Supabase happily consumes Firebase JWTs server-side.
5. Realtime via Supabase Realtime OR your existing Go WS layer (you already have one).

**Acceptable second pick: Full Firebase.** If you value "one vendor, one bill, zero plumbing" over portability, and you commit to defensive data modeling (sharded counters, flat collections, server-side leaderboard aggregation, no listener-as-primary-read).

**Worth keeping on the table: MySQL HeatWave as-is.** If user revises the "no Coolify-side state" stance — HeatWave is genuinely free forever and you've already scaffolded the Go store layer.

---

## 7. Concrete Defensive Patterns if You Pick Full Firebase

If user picks the full-Firebase path despite my pick, these are non-negotiable:

1. **No deep nesting.** All collections are flat. `users/{uid}`, `matches/{matchId}`, `puzzles/{date}`. Subcollections only for hard 1-to-many (e.g. `matches/{id}/turns`).
2. **Sharded counters** for any aggregate ≥100 writes/min. Pattern: `counters/{name}/shards/{0..9}`, write to random shard, sum on read.
3. **Aggregated leaderboard doc** updated by Cloud Function on attempt-write, polled by clients (not subscribed). Top-N only; tail accessed via paginated query.
4. **No real-time listeners on collection-level queries.** Listen on single docs or capped subcollections only.
5. **Schema discipline.** Same field shape across all docs in a collection. Future you migrating: thank past you.
6. **Pre-write Firestore→Postgres mirror script** in Phase 1, sitting unused. Validates your data model is migration-friendly. Costs nothing if never used.

---

## 8. Concrete Q&A vs Prior Report

| Prior report claim | This report's verdict |
|---------------------|------------------------|
| "Migration = 1 day, 2–3 hour downtime" | **Wrong.** 2–6 weeks realistic. |
| "Firebase free tier sufficient ≤100 DAU" | Confirmed. |
| "Lock-in HIGH" | Confirmed. Acceptable with hybrid (Auth + external data). |
| "Stay on MySQL HeatWave" | **Defensible but ignores user's explicit pivot.** |
| "$10–15/mo at 400 DAU" | Roughly correct; reads dominate. |
| Firestore scale fit for leaderboards | **Underweighted.** Hotspot risk requires sharded counters from day 1. |

---

## 9. Unresolved Questions

1. **What's dleague's expected sync-PvP turn cadence?** If turns are >1s apart, full Firebase is fine. If sub-second (real-time grid puzzles), needs sharded match doc or stays on WS layer.
2. **Is global leaderboard scope global-all-time, daily, friends-only?** Friends-only sidesteps the 1K-watcher fan-out problem entirely. Global all-time forces sharded counter from day 1.
3. **Will you actually pay the $25/mo Supabase Pro at 1K DAU, or does that violate "free tier only"?** Supabase free tier is 500 MB / 50K MAU / 5 GB egress; dleague at 1K DAU likely fits free tier comfortably until ~5K DAU.
4. **Does Coolify deployment story prefer Firebase Hosting or Supabase static hosting or stay on Coolify?** Affects Phase 8 of current pivot plan.
5. **Mobile-platform Firestore SDK story for Capacitor:** the Capacitor Firebase plugin is community-maintained; risk if it lags Firebase JS SDK on iOS WKWebView quirks.
6. **Firebase Auth → Supabase JWT bridge maturity:** Supabase docs cover external JWT verification, but Go-server-side it's a custom claims-extraction step. Spike before committing.

---

## Sources

- [See a Cloud Firestore pricing example | Firebase](https://firebase.google.com/docs/firestore/billing-example)
- [Firestore pricing | Google Cloud](https://cloud.google.com/firestore/pricing)
- [Google Firestore Pricing Guide: Real-World Costs & Optimization Tips | Airbyte](https://airbyte.com/data-engineering-resources/google-firestore-pricing)
- [Understanding Firebase Costs and Integration Challenges | MetaCTO](https://www.metacto.com/blogs/the-true-cost-of-firebase-setup-integration-and-maintenance-in-2024-complete-breakdown)
- [Functionally migrating from Firestore to PostgreSQL | Valentin Mouret (Medium)](https://medium.com/@ValentinMouret/functionally-migrating-from-firestore-to-postgresql-64947b5dff0d)
- [Out of the Fire(store): Traba's Journey to Postgres](https://engineering.traba.work/firestore-postgres-migration)
- [Why I Switched from Firebase to Supabase + PostgreSQL (And Cut My Costs 80%) | DEV](https://dev.to/th19930828/why-i-switched-from-firebase-to-supabase-postgresql-and-cut-my-costs-80-1ofg)
- [Migrate from Firebase Firestore to Supabase | Supabase Docs](https://supabase.com/docs/guides/platform/migrating-to-supabase/firestore-data)
- [Migration Journey: From Firebase to Go, PostgreSQL, MinIO | AICU Life](https://blog.aicu.life/posts/migration-journey-from-firebase-to-go-postgresql-and-minio-stack-vol-1/)
- [Firebase Database Migration to PostgreSQL (not a tutorial) | J. M. Rodrigues (Medium)](https://medium.com/@jm_rodrigues/firebase-database-migration-to-postgresql-not-a-tutorial-6a894c5ddc59)
- [Database Free Tier Comparison 2026 — Supabase vs Neon vs Firebase vs Turso vs PlanetScale](https://agentdeals.dev/database-free-tier-comparison-2026)
- [Best Backend Platforms for Indie Hackers in 2026 | MindStudio](https://www.mindstudio.ai/blog/best-backend-platforms-indie-hackers)
- [Database Pricing Comparison (April 2026) | Supabase, Neon, PlanetScale](https://www.buildmvpfast.com/api-costs/database)
- [No More Free Tier on PlanetScale, Here Are Free Alternatives | Niall Maher (Codú)](https://www.codu.co/niall/no-more-free-tier-on-planetscale-here-are-free-alternatives-q4wzqcu9)
- [Understand reads and writes at scale | Firestore | Firebase](https://firebase.google.com/docs/firestore/understand-reads-writes-scale)
- [Building Scalable Real Time Applications with Firestore | Google Cloud Blog](https://cloud.google.com/blog/products/databases/building-scalable-real-time-applications-with-firestore)
- [How to Handle Firestore 10-Write-Per-Second Document Limit | OneUptime](https://oneuptime.com/blog/post/2026-02-17-how-to-handle-firestore-10-write-per-second-document-limit/view)
- [Best practices for Cloud Firestore | Firebase](https://firebase.google.com/docs/firestore/best-practices)
- [Firebase vs Supabase in 2025: Which one actually scales | DEV](https://dev.to/dev_tips/firebase-vs-supabase-in-2025-which-one-actually-scales-with-you-2374)
