# MongoDB Atlas-only? Brutal honesty.

**TL;DR — Recommendation: C (status quo) for beta. ~75% confidence.**

Migrating now solves a problem that doesn't exist. The license panic that triggered "MongoDB Atlas only" thinking was already resolved on 2026-05-06: CE permits commercial use within 5-node / 4-core / no-XDCR caps. We are 1 node, fine. Code is shipped, tested, the seam works. **Burn the migration urge until DAU > 1k or ops pain shows up.** If migration *must* happen for non-technical reasons (peace-of-mind, hate self-hosting), then **B (Atlas + managed Redis)** > A. Atlas-only is a category error: Mongo as a Redis replacement at WS heartbeat rates is the wrong tool, and the savings are imaginary because Atlas M0 won't survive 1k DAU on 100-conn cap.

**The one question to ask first:** *"Is this about license fear, ops fatigue, or a belief that 'MongoDB Atlas' is the safe default — and if we eliminated all three, would you still want to migrate?"* The answer determines whether the right move is migrate, document, or therapy.

---

## Grounding facts (from repo)

- **Redis usage is trivial:** 4 files, ~190 LOC. Plain commands: `ZADD GT`, `ZRANGEBYSCORE`, `SET EX`, `EXISTS`. No Lua, no cluster, no pubsub.
- **Couchbase usage is trivial:** flat JSON docs, primary index only, 4 collections. No N1QL aggregations, no XATTRs, no eventing.
- **Seam is real:** 3 impls (couchbase, redis, memstore). grep-isolation enforced. swap = 1 week per migration-readiness.md.
- **License panic is closed:** researcher #2 superseded #1. CE explicitly permits commercial use under caps. Documented `2026-05-06`. **There is no legal pressure to migrate.**
- **Scale:** <1k DAU beta. Single OCI Always-Free A1 (4 OCPU / 24 GB). Free.

---

## Option matrix

### A. MongoDB Atlas only (user pick)

| Pros | Cons |
|---|---|
| One driver, one conn string, one dashboard | TTL indexes purge eventually (~60s), not at expiry — presence will be stale-read-prone |
| Free M0 tier exists | M0 = 512MB / **100 conns max** / shared CPU — WS server with 50+ persistent clients each pinging Mongo on heartbeat hits conn cap fast |
| Atlas operational maturity (backups, multi-AZ paid) | Sorted leaderboards = indexed `find().sort().limit()` on every TopN call; per-call cost > Redis ZRANGE by 1-2 orders of magnitude |
| No license anxiety | Conditional "higher-score-wins" ZADD-GT semantics → `findOneAndUpdate({$gt}, $set)` round-trips with retry on race; non-trivial atomicity reasoning |
| Single backup target | M0 → M10 jump is **$57+/mo** the moment we outgrow free; no graceful intermediate |
| | "Cache" layer becomes serialized BSON over network for every cache hit; cheap-read property is gone |
| | Mongo's write-on-heartbeat for presence at 50 WS clients × 30s = ~100 writes/min just for liveness — wasteful on M0 |

**Net:** moves cognitive complexity *up* (TTL semantics, atomic upserts, index-tuning leaderboards) while moving operational complexity *down* (one less container). For a 1-person beta team, ops complexity savings on a single VM with 2 well-known containers is **near zero**.

### B. MongoDB Atlas + Upstash Redis (or any managed Redis)

| Pros | Cons |
|---|---|
| Right tool per workload — Redis stays Redis | Two managed bills (Upstash free tier exists: 10k cmds/day or 256MB) |
| Atlas M0 survives because cache/leaderboard traffic is off it | Two dashboards, two SDK upgrade cadences |
| Existing Go code maps almost 1:1 (go-redis works against Upstash) | Two failure modes |
| WS heartbeat / leaderboard latency stays ms-scale | Marginal cognitive overhead vs A — but the team already paid that cost |

**Net:** if migration is happening, this is the honest version. ~3-5 days of work because Redis half doesn't change. Strictly better than A on every technical axis except "fewer logos".

### C. Status quo — Couchbase CE + Redis (do nothing)

| Pros | Cons |
|---|---|
| Migration cost = 0 | Couchbase CE has caps; if dleague suddenly needs >5 nodes we have a (good) problem |
| Already tested, integration-test gated, exported, documented | ARM64 manifest dependency for OCI deploy (Phase 11 risk) |
| $0 hosting (OCI Always-Free) | Single-VM SPOF — but acknowledged in beta posture |
| Both backends are hot-paths-optimal | Operationally: 2 containers to monitor (small task, but nonzero) |
| License is settled and documented in-repo | Couchbase 8.0 CE on ARM64 less battle-tested than x86 |
| Seam already exists → future migration deferable, not lost | None of these caps bite at <1k DAU |

**Net:** the option that ships fastest and lets the team focus on Phase 11 deploy + Phase 12 cleanup. **The only option that doesn't pay migration cost twice (now + post-beta reassessment).**

### D. Couchbase Capella + drop Redis (Mongo-A symmetry on Couchbase)

| Pros | Cons |
|---|---|
| Capella free tier auto-pauses on inactivity (cost predictable) | Same "wrong tool for cache" problem as A — Couchbase KV TTL works but leaderboards via N1QL ORDER BY are not Redis-class |
| Same `gocb` driver; even smaller diff than A | Capella free tier auto-pause = cold-start latency on first request after idle (nightmare for a "race-an-opponent" UX) |
| No license risk | Couchbase ecosystem is smaller; less Stack Overflow gravity than Atlas |
| | Capella doesn't have the "everyone's heard of it" comfort signal Atlas does |

**Net:** technically defensible, no real advantage over status quo for beta, and auto-pause is genuinely bad for a real-time game. **Skip.**

---

## Direct answers to the six questions

**1. "One backend" — simpler or just fewer logos?**
Just fewer logos for *this team at this scale*. The two containers are not the operational pain. The pain (if any) is monitoring/backup discipline — and Atlas doesn't fix that automatically; you still need to set alerts, IP allowlists, billing caps. Cognitive complexity per workload **goes up** with Mongo-as-Redis (TTL purge timing, conditional upserts, index sort). For a 1-person team, this is a net loss.

**2. Atlas M0 realism.**
M0 will hold 50 WS connections × heartbeat-write-presence-on-Mongo for *a few hundred users*. The 100-conn cap and shared CPU are the binding constraints, not storage. **The honest answer:** M0 is a demo tier, not a beta tier. Plan for M10 ($57+/mo) the moment you have 200+ concurrent WS sockets. That's not "free managed" — that's "$700/yr tax to avoid two containers".

**3. Cache-in-Mongo honest?**
**No.** Self-deception. Redis cache hit = ~0.5ms in-VM. Mongo cache hit = ~5-30ms over network + BSON roundtrip + Atlas auth. The "cache" stops being a cache; it's just a slower read of the same data. Either keep Redis (option B) or admit you don't need caching at <1k DAU and remove the abstraction.

**4. Migration cost asymmetry.**
Correct framing. Seam is real → Mongo-now and Mongo-later are equally cheap. **Therefore the rational move is: migrate when there's a forcing function, not preemptively.** Currently no forcing function exists.

**5. License-panic stress reaction?**
**Strong "yes" suspicion.** The roadmap shows researcher #1's wrong reading was superseded by researcher #2 only one day before this brainstorm. The "MongoDB Atlas only" ask carries the emotional residue of "what if Couchbase comes after us". The right fix is `docs/migration-readiness.md` § License watchout, which already exists and is correct. **Do not migrate to soothe an already-resolved fear.**

**6. What does the user actually want?**
Most likely (b) "free managed for peace-of-mind" + (d) "burned by self-hosting in some past life". Less likely (c). If (b)/(d), the fix may be **pre-staging the migration plan** (already done — seam exists) and *not executing it* until ops pain manifests. That delivers peace-of-mind without paying the cost.

---

## Confidence calibration

- **75% C (status quo for beta)** — code shipped, license settled, Phase 11/12 in flight, no forcing function.
- **20% B (Atlas + managed Redis)** — *if* user produces a non-emotional reason: e.g. OCI VM access lost, or genuine Couchbase ARM64 deployment failure in Phase 11.
- **5% A (Atlas-only)** — only if user explicitly accepts the M10 bill, the WS-presence latency hit, and the loss of Redis-class leaderboard semantics.
- **0% D** — Capella auto-pause kills real-time UX.

---

## What I'd actually recommend doing now

1. **Don't migrate.** Finish Phase 11 (Coolify deploy on OCI) on the current stack.
2. **Add one alert:** Couchbase node count + core count (so the cap is visible if we ever scale up by accident).
3. **Keep `migration-readiness.md` warm** — that's the insurance policy. The seam is the point. We already paid for optionality; we don't have to spend it.
4. **Defer the migration decision** to the post-beta tech-stack reassessment milestone (~5k DAU per roadmap). At that point we'll have *data* about ops pain and traffic shape — which makes A vs B vs C a real engineering decision, not a vibes-driven one.
5. **If a migration is non-negotiable** (the user really wants to move regardless): pick **B**, not A. Two-day Mongo port + zero touch on the Redis half.

---

## Unresolved questions (ask user before any migration)

1. **Forcing function?** What concretely changed since 2026-05-06 license clarification that made "Atlas-only" surface? If nothing — this is residual anxiety, not a requirement.
2. **OCI VM access stable?** If the Always-Free instance is at risk (account review, region issues), that *is* a forcing function and changes the math toward managed.
3. **Budget reality:** Is $0/mo a hard constraint, or is $20-60/mo acceptable for beta peace-of-mind? (Atlas-only at scale = $57+; B = $0-30 with Upstash free + Atlas free.)
4. **WS connection projection:** Realistic peak concurrent WS clients in beta? 50 = M0 survives. 200 = M0 dies, M10 mandatory.
5. **Are we conflating "managed" with "MongoDB"?** If the real desire is "managed", Capella + Upstash is symmetric and keeps zero code changes (`gocb` works against Capella unchanged). Worth comparing.
6. **Phase 11 deploy status:** if Phase 11 has hit a real ARM64 / Couchbase blocker not yet documented, that escalates B's priority.

