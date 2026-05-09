---
type: research
title: "CockroachDB Basic fitness re-evaluation for dleague — supersedes earlier verdict"
date: 2026-05-05
session: 260505-1207
supersedes: researcher-260505-1207-aiven-vs-cockroach.md
sources_consulted: 4 web searches 2026-05-05 + plan re-read
project: dleague
---

# CockroachDB re-evaluation — we DO NOT need real Postgres

## TL;DR — verdict flipped

**Pick CockroachDB Basic.** The previous report's "deal-breaker" was wrong. dleague's actual Phase 3–5 plans:

- ✅ Use **in-memory WS hub** for live updates (`server hub broadcasts opponent events` in `phase-05-sync-pvp-websocket.md`) — **no `LISTEN/NOTIFY` ever invoked**
- ✅ Use **`SELECT FOR UPDATE`** for race conditions (`phase-04-async-pvp.md:100`) — **no advisory locks ever invoked**
- ✅ Only one PG-specific identifier in any phase doc: **`citext`** (1 column, `users.email`) — **and CITEXT IS natively supported in CockroachDB** ([docs](https://www.cockroachlabs.com/docs/stable/citext))

The previous report invented requirements that the plans don't have. With those phantoms removed, every concrete schema operation in dleague's plans runs unchanged on Cockroach.

## What changed since the previous report

Re-read `phase-03..05` and grepped for PG-only features. Result:

```
LISTEN / NOTIFY:           0 occurrences in any plan file
pg_advisory*:              0 occurrences in any plan file
Broadcast:                 in-memory WS hub (phase-05 line 40)
SELECT FOR UPDATE:         used (phase-04 line 100) — Cockroach supports
citext:                    1 column (users.email) — Cockroach has it natively
```

I had assumed advisory locks for matchmaking and `LISTEN/NOTIFY` for live updates. Wrong on both. dleague is a **single-process Go server** holding all WebSocket connections in memory; cross-process pub/sub is not part of the architecture.

## Real Cockroach gotchas for dleague

These are concrete and minor:

### 1. Serializable isolation → app-side retry handler

Cockroach defaults to SERIALIZABLE. Transactions occasionally abort with SQLSTATE 40001 ("restart transaction"). Apps must retry.

**Solution:** drop in the official `crdbpgx` wrapper:

```go
import "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"

err := crdbpgx.ExecuteTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
    // your match-join logic with SELECT FOR UPDATE
    return nil
})
```

That's the entire integration cost. ~30 lines wrapping our existing `pgxpool`. Source: [crdbpgx pkg.go.dev](https://pkg.go.dev/github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx), [Cockroach blog: when and why SELECT FOR UPDATE](https://www.cockroachlabs.com/blog/when-and-why-to-use-select-for-update-in-cockroachdb/).

### 2. JSONB has one weird edge case

CockroachDB JSONB ≈ PostgreSQL JSONB except: **JSONPath comparisons against empty arrays return `null` instead of `false`** ([Cockroach JSONB docs](https://www.cockroachlabs.com/docs/stable/jsonb)).

Operators `->`, `->>`, `@>`, `?` all match Postgres semantics. We use JSONB only for opaque game-state snapshots (`State` is `[]byte`-shaped per `shared/game/game.go:23`). We don't query inside it. **Edge case unreachable for dleague.**

### 3. CITEXT cost is marginally higher

Cockroach CITEXT uses a collation engine per comparison — slightly more CPU than PG's. 1 column, single email-uniqueness lookup per registration. Negligible.

### 4. SERIAL → use UUID

Cockroach's `SERIAL` produces `unique_rowid()` values that are cluster-monotonic, not sequence-monotonic. Plan already uses UUIDs everywhere (`users.id uuid pk`). No change.

## Updated comparison (corrected)

| | **Aiven Free PG** | **Cockroach Basic Free** |
|---|---|---|
| Storage | 1 GB ⚠ | **10 GiB** ✓ |
| Backups on free | ❌ | **Daily** ✓ (since Dec 2024) |
| Autopause / cold start | **Auto power-off** when idle ⚠ | None — wakes <100 ms ✓ |
| Free tier permanence (recent track record) | Cut storage 5 GB → 1 GB on 2025-05-15 ⚠ | $15 credit unchanged through 2024 rebrand ✓ |
| LISTEN/NOTIFY support | Yes, **but unused by plan** | No — **plan never needs it** |
| Advisory locks | Yes, **but unused by plan** | Stubs only — **plan never needs them** |
| `SELECT FOR UPDATE` (used by plan) | ✓ | ✓ |
| `citext` (used by plan) | ✓ | ✓ |
| `JSONB` (used by plan, opaque) | ✓ | ✓ (edge case unreachable for us) |
| Serializable retry handling | Not needed (snapshot isolation default) | Need `crdbpgx.ExecuteTx` wrapper (~30 LOC) |
| Connection limits | ~25–100 (free, undocumented) | 500/cluster ✓ |
| Tokyo region (free) | ✓ ap-northeast-1 | ❌ Singapore only |
| Frankfurt region (free) | ✓ | ✓ |
| Cost above free at 2 GB / 1k users | ~$25/mo Startup tier (cliff) | ~$5–15/mo metered overage |

Sources: [Cockroach Basic plan docs](https://www.cockroachlabs.com/docs/cockroachcloud/plan-your-cluster-basic), [Cockroach Transaction Retry Reference](https://www.cockroachlabs.com/docs/stable/transaction-retry-error-reference), [Cockroach Serverless: Free. Seriously.](https://www.cockroachlabs.com/blog/serverless-free/).

## Why Cockroach now wins for dleague

For a project that:
- Caps at <500 MB year 1 but might grow if the game catches on (10 GiB headroom is forward insurance)
- Bursts during sync-PvP matches then idles (no autopause = no babysitting cron)
- Has zero use of PG-only features once you actually read the plans
- Wants smooth cost growth, not plan-tier cliffs

…Cockroach is the better operational fit. The integration cost is one `crdbpgx` wrapper.

**The previous report's only valid concern that survives:** Tokyo region. If the Coolify VM lands in OCI Tokyo/Osaka, Cockroach forces ~70 ms RTT to Singapore. For a sync-PvP game targeting <50 ms DB ops, that hurts.

Mitigation: pick **OCI Frankfurt** (or **Ashburn / Phoenix**) for the dev VM. Both regions have Cockroach Basic + reasonable latency to Vietnam (~150–200 ms client → server, dominated by user-to-server hop, not server-to-DB). DB latency stays inside the same AWS region as Coolify.

## Migration cost estimate

Going Cockroach instead of Aiven for Phase 3:

```
+ Add cockroach-go/v2 dependency                            5 min
+ Wrap our matchmaking/match-join txns in crdbpgx.ExecuteTx 30 min
+ Test SELECT FOR UPDATE under contention (already in plan) 1 hr (already budgeted)
+ Switch DSN env var                                        2 min
+ Document retry behavior in code-standards.md              10 min
─────────────────────────────────────────────────────────────────
Total: ~2 hours
```

Going Aiven (zero code change but ongoing ops):

```
+ Set up Aiven free instance                                10 min
+ Add 5-min keepalive ping (cron or goroutine)              30 min
+ Set up weekly pg_dump → R2/S3 backup script               1 hr
+ Monitor Aiven free-plan changelog                         ongoing
─────────────────────────────────────────────────────────────────
Total: ~2 hours upfront + ongoing vigilance
```

Roughly equal upfront effort. **Cockroach has lower ongoing maintenance** (no keepalive, free backups).

## Updated recommendation

> **Use CockroachDB Basic Free.**

**Decisive reason (corrected):** The plan never invokes any PG-only feature beyond CITEXT (which Cockroach supports natively). With that phantom dependency removed, Cockroach's 10× storage, free daily backups, and no-autopause behavior dominate Aiven's only remaining advantage (smaller serializable-retry surface, which `crdbpgx` solves in ~30 LOC).

**Pick region:**
- Coolify in OCI Frankfurt → Cockroach AWS eu-central-1
- Coolify in OCI Phoenix → Cockroach GCP us-west1 or AWS us-east-1
- **Avoid OCI Tokyo/Osaka** — Cockroach has no Japan region on Basic (Singapore = ~70 ms RTT penalty)

**Setup checklist (Phase 3):**

1. Create CockroachDB Cloud account (no card required for Basic)
2. Create cluster: Region matching OCI VM, single-region (free tier)
3. DSN env: `DATABASE_URL=postgres://user:pass@host:26257/dleague?sslmode=require`
4. Add deps: `go get github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx github.com/jackc/pgx/v5`
5. All multi-statement transactions go through `crdbpgx.ExecuteTx`
6. Single-statement reads/writes use `pgxpool` directly (no retry wrapper needed)
7. Document the retry pattern in `docs/code-standards.md`

**Backup plan:**

- If Cockroach changes free-tier policy → migrate to **Aiven** (the original second choice)
- If sync-PvP latency budget breaks because the OCI VM was placed in Tokyo → either move VM to Frankfurt OR migrate to Aiven Tokyo
- Migration vector: `cockroach dump` → `pg_restore` is non-trivial because of CITEXT/JSON edge cases. Easier vector: drop CITEXT pre-migration, use `LOWER(email)` unique index instead, then export plain SQL

## What changed since I last said the opposite

I conflated **what the plans could have used** with **what they actually use**. The previous report's matrix was correct on Cockroach's PG-feature gaps; the error was assuming dleague intended to use those features. Re-reading `phase-04-async-pvp.md:100` (which says `SELECT FOR UPDATE`, not advisory lock) and `phase-05-sync-pvp-websocket.md:40` (which says in-memory `Broadcast`, not `LISTEN/NOTIFY`) corrects the picture.

Process lesson: when evaluating a DB choice against a project, grep the project's existing plans for the disputed features before listing them as deal-breakers.

## Unresolved questions

1. **Cockroach Basic single-region cold-start latency** — docs claim "<100 ms" for Serverless dormant clusters, but the rebranded "Basic" tier specs aren't crystal clear about cold-start behavior on the free $15 credit. Need to spin up + benchmark before Phase 5 sync-PvP integration testing.
2. **CRDB JSONB empty-array edge case** — confirmed unreachable for dleague today (we use JSONB as opaque blob). Re-check if Phase 6 polish ever queries inside game-state JSON for analytics.
3. **OCI VM region choice** — locked decision needed. If Vietnam-based dev wants OCI Tokyo for low admin-RTT, Cockroach's Singapore-only APAC presence is a problem. Either commit to Frankfurt now or revert to Aiven.
4. **`crdbpgx` vs raw pgx maintenance burden** — wrapper is officially maintained by Cockroach Labs. Risk of abandonment is low but non-zero. Compare commit cadence at next phase boundary.
5. **Cockroach $15 credit consumption rate at our scale** — 50M RUs/mo for 100 users at <100 queries/user/day = ~10k queries/day = ~300k/mo. Each query ≈ 5–20 RUs. Estimate ~6M RUs/mo, well under cap. Verify after Phase 4 ships.

## Sources (this report)

- [CockroachDB Transaction Retry Error Reference](https://www.cockroachlabs.com/docs/stable/transaction-retry-error-reference)
- [CockroachDB blog: When and why to use SELECT FOR UPDATE](https://www.cockroachlabs.com/blog/when-and-why-to-use-select-for-update-in-cockroachdb/)
- [crdbpgx package docs](https://pkg.go.dev/github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx)
- [Cockroach JSONB docs](https://www.cockroachlabs.com/docs/stable/jsonb)
- [Cockroach CITEXT docs](https://www.cockroachlabs.com/docs/stable/citext)
- [Cockroach Basic cluster planning](https://www.cockroachlabs.com/docs/cockroachcloud/plan-your-cluster-basic)
- [Cockroach Serverless: Free. Seriously. (cold-start <100 ms)](https://www.cockroachlabs.com/blog/serverless-free/)
- Plan files at `plans/260505-0947-dleague-pvp-game/phase-0[3-5]*.md`
