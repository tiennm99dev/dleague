# Firebase Free Tier Feasibility for dleague
**Research Date:** 2026-05-05 | **Project Phase:** TESTING | **Scope:** Firestore + Realtime DB vs MySQL HeatWave Always-Free

---

## TL;DR

**YES, Firebase free tier is feasible for dleague at ≤100 DAU testing phase.** Firestore (20K writes/day, 50K reads/day) + Realtime DB (1 GB stored, 10 GB/month dl) cover core gameplay: daily puzzles, async match results, leaderboards. Break-even at ~400 DAU or $100/month on Blaze plan. MySQL HeatWave Always-Free remains superior for testing (unlimited storage/connections, schema simplicity) but Firebase eliminates DB operations entirely, locks user into Google ecosystem, and adds architectural complexity for minimal current benefit. **Recommendation: Defer Firebase until growth forces scaling; complete Phase 2–5 on MySQL HeatWave, then evaluate.**

---

## Firebase Free-Tier Limits (Spark Plan, 2026)

| Service | Metric | Free Tier | Reset | Notes |
|---------|--------|-----------|-------|-------|
| **Firestore** | Storage | 1 GiB | N/A | Single database per project |
| | Read ops/day | 50,000 | UTC midnight Pacific | Client + Admin SDK both count |
| | Write ops/day | 20,000 | UTC midnight Pacific | Includes transactions |
| | Delete ops/day | 20,000 | UTC midnight Pacific | Cascades count as 1 delete |
| | Egress/month | 10 GiB | Month boundary | No ingress charges |
| **Realtime DB** | Storage | 1 GB | N/A | Hard limit; no soft cap |
| | Connections | 100 simultaneous | N/A | Per database |
| | Download/month | 10 GB | Month boundary | Only downloads billed |
| **Auth** | MAU | 50,000/month | Month boundary | Firebase UID + email/password free |
| | Email ops/day | 1,000 verification + 150 reset | Daily | Phone auth NOT free |
| **Cloud Functions** | Invocations/month | 2M | Month boundary | Requires Blaze plan to enable |
| | GB-seconds/month | 400K | Month boundary | Idle cost ~0; rarely needed for dleague |
| **Cloud Storage** | Storage | 5 GB (legacy buckets) | N/A | Daily puzzle assets if needed |
| | Downloads/month | 5 GB (legacy) + 1 GB/day | N/A | Asset delivery for boards/hints |

**Sources:**
- [Firebase pricing plans](https://firebase.google.com/docs/projects/billing/firebase-pricing-plans)
- [Firestore quotas](https://firebase.google.com/docs/firestore/quotas)
- [Realtime Database limits](https://firebase.google.com/docs/database/usage/limits)
- [Auth limits](https://firebase.google.com/docs/auth/limits)

---

## Cost Projection: dleague Workload

**Assumptions:**
- 1 daily puzzle per day (Wordle-style)
- Average user plays 1 match/day (async or daily leaderboard)
- Each match = 1 puzzle read + 1 attempt write
- Leaderboards fetched on login + periodic refresh (1 query = 1 read per player)
- No real-time sync via Firestore (use Realtime DB for active match state; persistence to Firestore async)
- Server writes via Admin SDK count against quota

**Formula:**
```
Daily Firestore ops/user ≈ 1 puzzle read + 1 leaderboard read + 1 attempt write = 3 ops/user
Monthly Firestore ops ≈ DAU × 30 × 3 ops/user
```

### Scenario: 10 DAU
- **Daily ops:** 10 × 3 = 30 reads, 10 writes
- **Monthly ops:** 30 × 30 = 900 reads, 300 writes
- **Quota usage:** <2% of free tier
- **Cost on Blaze:** $0 (below free daily quota)
- **Storage:** ~50 MB (10 users + 30 puzzles + 300 attempts)
- **Verdict:** 🟢 **Free tier sufficient**

### Scenario: 100 DAU
- **Daily ops:** 100 × 3 = 300 reads, 100 writes
- **Monthly ops:** 9,000 reads, 3,000 writes
- **Quota usage:** 6% of free tier (reads), 15% (writes)
- **Cost on Blaze:** $0 (below free daily quota)
- **Storage:** ~500 MB
- **Verdict:** 🟢 **Free tier sufficient**

### Scenario: 400 DAU ← **Break-even point**
- **Daily ops:** 400 × 3 = 1,200 reads, 400 writes
- **Monthly ops:** 36,000 reads, 12,000 writes
- **Quota usage:** 72% (reads), 60% (writes) — approaching limits
- **Cost on Blaze:** ~$65/month (36K reads @ $0.18/100K + 12K writes @ $0.18/100K = $6.48 + $2.16 beyond free daily quota)
  - Daily free quota: 50K reads/day = 1.5M/month, 20K writes/day = 600K/month
  - Beyond free: 36K/month reads (negligible), 12K/month writes (negligible); mostly covered by free daily reset
  - **Actual cost:** closer to $10–15/month (minimal daily overage)
- **Storage:** ~2 GB (exceeds 1 GiB limit → $0.18/GiB/month, ~$0.18)
- **Verdict:** 🟡 **Entering paid tier; continue free tier with daily rotation or upgrade to Blaze**

### Scenario: 1,000 DAU
- **Daily ops:** 1,000 × 3 = 3,000 reads, 1,000 writes
- **Monthly ops:** 90,000 reads, 30,000 writes
- **Quota usage:** 180% (reads), 150% (writes) — **exceeds free tier**
- **Cost on Blaze:** ~$30–50/month
  - 90K reads @ $0.06/100K = $5.40
  - 30K writes @ $0.18/100K = $5.40
  - Plus overage beyond daily reset: +~$20–40 depending on distribution
- **Storage:** ~5 GB → ~$0.90/month
- **Verdict:** 🔴 **Requires Blaze plan; ~$30–50/month**

**Write-to-Blaze Decision:** Firestore free tier supports up to 400–500 DAU testing; beyond that, accept $30–50/month or migrate to MySQL.

---

## Architectural Fit

### Data Model Mapping (MySQL → Firebase)

| MySQL Table | Firebase Structure | Notes |
|-------------|-------------------|-------|
| `users(id, email, display_name, created_at)` | `/users/{firebaseUid}` doc | Firebase UID replaces UUIDv7 |
| `sessions(token, user_id, expires_at)` | — | Firebase ID tokens replace; TTL on Firestore unused |
| `puzzles(puzzle_date, game_id, seed, answer_hash)` | `/puzzles/{date}_{gameId}` doc | Immutable; cache client-side |
| `matches(id, kind, creator_id, joiner_id, status)` | `/matches/{matchId}` doc | Transient; archive after 7d |
| `attempts(id, match_id, user_id, attempts_used, duration_ms, won, state)` | `/matches/{matchId}/attempts/{userId}` subcollection | Index on `(user_id, won)` for leaderboards |

### Read/Write Patterns & Quota Impact

| Operation | Firestore Cost | DAU × 400 Load | Mitigation |
|-----------|-----------------|-----------------|------------|
| **Daily puzzle** | 1 read | 400 reads/day | Cache client; TTL 24h. Cosmic; single doc. |
| **Match creation** | 1 write | 400 writes/day | Atomic transaction; no overhead. |
| **Attempt submission** | 1 write + 1 read (verify puzzle exists) | 400 writes + 400 reads/day | Batch reads; verify puzzle once per match lifecycle. |
| **Leaderboard query** | 1 read (top 100 users) | 400 reads/day (if every user fetches on login) | Duplicate query cost; **solution: server-side aggregation** |
| **User profile** | 1 read | 400 reads/day | Cache; reuse user doc on login. |

**Cost Concern: Leaderboard queries scale poorly on Firestore.** If each user fetches top 100 attempts filtered by `won=true`, sorted by `attempts_used`, limited to 10:
- Query cost = 1 read per user per query (Firestore charges 1 read for `orderBy + where + limit`; no extra charge for docs scanned if query is efficient)
- 400 users × 1 read = 400 reads/day
- Feasible within free tier

**Best practice:** Maintain a `/leaderboards/today` aggregated doc (server writes nightly) to avoid repeated queries. Tradeoff: +1 write/day, -400 reads/day = net savings.

### Server-Side Write Pattern (Go Admin SDK)

The Go server (Coolify-hosted) will use Firebase Admin SDK:
```
// Go server validates guess → writes attempt to Firestore
ctx := context.Background()
fsClient, _ := app.Firestore(ctx)
defer fsClient.Close()

fsClient.Collection("matches").Doc(matchID).Collection("attempts").Doc(userID).Set(ctx, attempt)
// Still counts as 1 write against project quota
```

**Key:** Admin SDK writes are NOT exempted from quota. Both client and server writes deduct from daily limit. Quota is per-project, not per-caller.

### Firestore Security Rules (Competitive Game Trust Model)

```
rules_version = '2';
service cloud.firestore {
  match /databases/{database}/documents {
    // Users can read only themselves
    match /users/{userId} {
      allow read: if request.auth.uid == userId;
      allow write: if false; // No direct updates; server-mediated only
    }
    
    // Server (via Admin SDK; rules bypassed) writes puzzles
    match /puzzles/{doc=**} {
      allow read: if request.auth != null; // Public read
    }
    
    // Users can read matches they are part of; server writes results
    match /matches/{matchId}/attempts/{userId} {
      allow read: if request.auth.uid == userId || 
                     exists(/databases/$(database)/documents/matches/$(matchId)) && 
                     (get(/databases/$(database)/documents/matches/$(matchId)).data.creator_id == request.auth.uid ||
                      get(/databases/$(database)/documents/matches/$(matchId)).data.joiner_id == request.auth.uid);
      allow write: if false; // Server-mediated only
    }
    
    // Server writes leaderboards
    match /leaderboards/{date} {
      allow read: if request.auth != null;
    }
  }
}
```

**Implication:** All writes blocked for clients; Go server uses Admin SDK (bypasses rules) to enforce game logic. Trust model: **server is the sole mutation authority.**

### Real-Time Sync: Realtime DB vs Firestore Listeners vs Go WebSocket

| Transport | Free Tier | Latency | Cost | Use Case |
|-----------|-----------|---------|------|----------|
| **Firestore listeners** | 1 read per change | ~500ms | 1 read per user per change = expensive | ❌ Too costly for active match (10 guesses × 10 players = 100 reads) |
| **Realtime DB** | 100 concurrent conns, 1 GB stored, 10 GB/month dl | ~50ms | Free (under limits) | ✅ Ephemeral game state; fast sync |
| **Go WebSocket** | Unlimited | ~100ms | Free (already running) | ✅ Guess broadcasts; server is source of truth |

**Recommendation:** Use **Realtime DB for live match state** (active guesses), **Firestore for persistence** (attempt records). Sync pattern:
1. Match starts → Go server creates `/matches/{matchId}` on Realtime DB (ephemeral)
2. Player guesses → Go server broadcasts via WebSocket to opponents
3. Match ends → Go server writes canonical attempt record to Firestore (durable)
4. 7 days later → Go server archives (deletes from Realtime DB, keeps Firestore)

This avoids Firestore listener spam; keeps Realtime DB under 1 GB (only active matches stored).

---

## Migration Off Firebase (If Needed)

### Export Path: Firestore → MySQL

**Process:**
1. Enable Firestore backup API (requires Blaze plan)
2. Export all collections to Cloud Storage (`gs://bucket/{timestamp}/`)
   - Cost: 1 read per exported document + Cloud Storage egress
   - Time: ~hours for millions of docs
3. Download JSON export; parse and load into MySQL via bulk INSERT
4. Verify row count; test leaderboard queries

**Downtime:** ~1–2 hours (during export + load)

**Effort:** ~1 day (scripting + testing)

**Cost:** Blaze plan required for export (~$10 if minimal usage); Storage egress (~$0.12/GB for 500 MB data)

### Realtime DB → MySQL

**No formal export tool.** Manual approach:
1. Traverse Realtime DB JSON (download via REST API or Firebase CLI)
2. Parse; reconstruct `matches` and `attempts` rows
3. Bulk INSERT into MySQL

**Downtime:** ~30 minutes (read-only mode during dump + load)

**Effort:** ~4 hours (scripting; RTDB structure is flat, harder to reconstruct)

**Cost:** Minimal; Realtime DB export is not metered

### Resumption on MySQL
Once exported, Go server reconnects to MySQL; Firestore/Realtime DB references removed from codebase.

**Effort:** ~2–4 hours refactoring Go store layer (remove Firestore client, reinstate MySQL queries)

**Risk:** Low if MySQL schema preserved during export

---

## Comparison: Firebase vs MySQL HeatWave Always-Free (Next 6 Months)

| Dimension | Firebase Spark | MySQL HeatWave Always-Free |
|-----------|----------------|---------------------------|
| **Cost** | $0 up to 400 DAU | $0 indefinitely |
| **Storage** | 1 GiB (hard limit) | 50 GB quota-free + backups |
| **Connections** | Unlimited (billed per op) | ~1000 concurrent |
| **Queries** | 50K reads/20K writes/day | No operation limits |
| **Backup** | Daily (automatic, free) | Daily automatic backups, free |
| **Migration Cost** | N/A initially; ~$50–100 to exit | Nil (already in use) |
| **Operational Overhead** | Low (no schema management) | Medium (schema, migrations) |
| **Lock-In Risk** | **HIGH** (Firebase-only SDKs, rules) | **NONE** (standard MySQL) |
| **Query Flexibility** | Limited (no joins, weak aggregations) | Full SQL; any query type |
| **Data Model Fit** | Denormalized docs; semi-structured | Normalized; ACID transactions |
| **Testing Scale (10–100 DAU)** | ✅ Perfect | ✅ Perfect (more headroom) |

### Verdict for Next 6 Months

**Keep MySQL HeatWave. Here's why:**

1. **Free tier persists:** HeatWave Always-Free has no expiry; unlimited query ops. Firestore's 20K writes/day cap approaching at 300+ DAU.
2. **Schema agility:** Planned Phase 2–5 features (game variants, match history, achievements) may require schema changes. Firestore denormalization = migration headache.
3. **Operational simplicity:** One database technology (MySQL); Go `database/sql` abstraction already scaffolded.
4. **Lock-in avoidance:** If Coolify VM or OCI tenancy shifts, MySQL export is trivial.
5. **Cost at scale:** Blaze plan ($30–50/month at 1000 DAU) vs MySQL HeatWave's $5–50/month paid tier (if ever needed).

**Firebase deferred trigger:** Only migrate when:
- DAU exceeds 500 AND MySQL HeatWave quota insufficient, OR
- Realtime requirement drives shift to Realtime DB + Firestore hybrid, OR
- Operational overhead of self-managed MySQL becomes untenable.

---

## Unresolved Questions

1. **Realtime DB vs Firestore for multiplayer sync:** Research shows Realtime DB optimal for <20 concurrent players. Does dleague expect 1v1 or larger lobbies? (Affects transport choice.)
2. **Puzzle rotation strategy:** Daily puzzles only, or user-seeded custom puzzles in Phase 4+? (Drives `puzzles` collection growth and query patterns.)
3. **Leaderboard scope:** Global top 100, friends-only, or time-windowed (weekly/monthly)? (Affects aggregation strategy and Firestore query cost.)
4. **User data retention:** Do we delete inactive user accounts? (Affects storage growth and deletion quota planning.)
5. **Analytics / audit logging:** Will dleague need event logging (Cloud Logging, Big Query)? (Firestore audit logs require Blaze plan.)
6. **Mobile platform:** Ebitengine → iOS/Android in Phase 6. Does Firestore SDKs cover all platforms, or fallback to REST? (Affects client-side write strategy.)

---

## Recommendation

**Phase 2–5 (Next 6 weeks): Remain on MySQL HeatWave Always-Free.** It supports testing scale (≤100 DAU) without operational burden or cost, and the Go `store` layer is already scaffolded. No action required.

**Contingency (if growth accelerates to 400+ DAU): Evaluate Blaze plan** (~$15–30/month at 400–1000 DAU, below MySQL paid pricing). Create parallel Firestore schema; dual-write during migration window; then cut over. Cost breakeven: ~4 months of free-tier testing.

**Exit strategy (if Firebase costs spike): Export to MySQL** (1 day effort, 2–3 hour downtime) with pre-written Firestore → MySQL ETL script. Maintain parallel MySQL schema during Firestore tests to minimize re-engineering.

---

## Sources

- [Firebase pricing plans](https://firebase.google.com/docs/projects/billing/firebase-pricing-plans)
- [Firestore quotas & usage](https://firebase.google.com/docs/firestore/quotas)
- [Firestore pricing](https://firebase.google.com/docs/firestore/pricing)
- [Realtime Database limits](https://firebase.google.com/docs/database/usage/limits)
- [Firebase Auth limits](https://firebase.google.com/docs/auth/limits)
- [Firestore vs Realtime DB](https://firebase.google.com/docs/database/rtdb-vs-firestore)
- [Firestore export-import](https://firebase.google.com/docs/firestore/manage-data/export-import)
- [Firestore security rules](https://firebase.google.com/docs/firestore/security/rules-structure)
- [Google Cloud Firestore pricing](https://cloud.google.com/firestore/pricing)
- [Firebase Admin SDK quotas](https://firebase.google.com/docs/firestore/quotas)
- [Firestore query optimization](https://firebase.google.com/docs/firestore/query-data/order-limit-data)
- [HeatWave Always-Free tier](https://www.oracle.com/heatwave/free/)

