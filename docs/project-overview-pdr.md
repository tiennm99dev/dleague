# Dleague — Product Development Requirements

## Elevator pitch

**League of -dle games.** Dleague turns the daily-puzzle genre (Wordle, LoLdle, Sumdle, etc.) into PvP. Instead of solo-and-tweet, players race opponents in real-time or compete on shared daily puzzles with global leaderboards.

## Core modes

| Mode | Description |
|------|-------------|
| Daily Leaderboard | Same puzzle for everyone; ranked by attempts then time |
| Challenge a Friend | Share a link, opponent plays the same seed |
| Quick Match | Matchmaking queue → live opponent, real-time race |
| Pluggable variants | Wordle at launch; music/geo/image variants planned |

## Stack (current — beta)

| Layer | Tech |
|-------|------|
| Auth | Firebase Auth (Spark, free) — Email/Google/Anonymous |
| Backend | Go 1.25.5 (`chi` + `nhooyr.io/websocket`), one binary |
| Data plane | MongoDB Atlas M0 (free tier, AWS Singapore) — documents + leaderboards + presence + cache |
| Web client | Svelte 5 (shell + HUD) + Phaser 4 (game canvas) |
| Mobile shell | Capacitor (web first; iOS/Android later) |
| Hosting | OCI Always-Free Ampere A1 Flex (4 OCPU + 24 GB RAM, ARM64) via Coolify |

## Beta posture

- All sign-in screens show a **"Beta — data may reset"** banner.
- Every user is tagged `isBetaTester: true` + `betaSignupAt` on first auth.
- T&Cs make it explicit: data is collected for product evaluation; not contractually preserved.
- Atlas M0 wipe or accidental drop = acceptable data loss; `mongodump --uri "$MONGODB_URI"` is the escape hatch.

## Migration-ready architecture

Atlas is the chosen managed backend, but the seam stays intact in case we ever want to leave.

1. **`store.Store` Go interface** is the seam (`server/internal/store/store.go`).
2. **`go.mongodb.org/mongo-driver/v2`** import only in `internal/store/mongodb/`. `make grep-isolation` enforces it.
3. **Stable doc shapes** — flat JSON (no aggregation pipelines, no Atlas-only features beyond `$max` + TTL indexes — both standard MongoDB).
4. **`memstore`** impl ships alongside as test backend + proof of seam.
5. **`(store.Store).Export`** method streams every persistent doc as JSONL — same seed for any future imports anywhere; for outbound migration off MongoDB use `mongodump`.

## Out of scope (post-beta)

- Production-grade backups (current beta accepts data loss).
- Real-time spectator mode for sync PvP.
- Cross-region presence / multi-VM deployment.
- Atlas M0 → M10 (paid) upgrade: triggered when concurrent WS clients exceed ~100 or M0 storage cap (512 MB) gets close.
- Early-adopter reward mechanism for beta testers.

## License & legal

- **Proprietary** — All Rights Reserved (see `LICENSE`).
- **MongoDB Atlas** is governed by the Atlas commercial terms; our use is as a customer of a managed service. The MongoDB CE SSPL has no bearing on dleague's deployment. See `docs/migration-readiness.md` § License watchout.

## Latest shipped initiative

[`plans/archive/260507-1648-mongodb-atlas-only-migration/plan.md`](../plans/archive/260507-1648-mongodb-atlas-only-migration/plan.md) — Atlas consolidation (Couchbase + Redis dropped; single managed backend). All implementation plans live under `plans/archive/` once they ship or get superseded.
