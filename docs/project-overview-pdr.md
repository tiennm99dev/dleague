# Dleague — Product Development Requirements

## Elevator pitch

**League of -dle games.** Dleague turns the daily-puzzle genre (Wordle, LoLdle, Sumdle, etc.) into PvP. Instead of solo-and-tweet, players race opponents in real-time or compete on shared daily puzzles with global leaderboards.

## Core modes

| Mode | Description | Phase |
|------|-------------|-------|
| Daily Leaderboard | Same puzzle for everyone; ranked by attempts then time | 9 (async) |
| Challenge a Friend | Share a link, opponent plays the same seed | 9 |
| Quick Match | Matchmaking queue → live opponent, real-time race | 10 (sync via WS) |
| Pluggable variants | Wordle at launch; music/geo/image variants planned | 8 |

## Stack (current — beta)

| Layer | Tech |
|-------|------|
| Auth | Firebase Auth (Spark, free) — Email/Google/Anonymous |
| Backend | Go 1.25.5 (`chi` + `nhooyr.io/websocket`), one binary |
| Primary store | Couchbase Community 8.0 (self-hosted via docker-compose) |
| Cache + leaderboards | Redis 8.4 (self-hosted) |
| Web client | Svelte 5 (shell + HUD) + Phaser 4 (game canvas) |
| Mobile shell | Capacitor (web first; iOS/Android later) |
| Hosting | OCI Always-Free Ampere A1 Flex (4 OCPU + 24 GB RAM, ARM64) via Coolify |

## Beta posture

- All sign-in screens show a **"Beta — data may reset"** banner.
- Every user is tagged `isBetaTester: true` + `betaSignupAt` on first auth.
- T&Cs make it explicit: data is collected for product evaluation; not contractually preserved.
- VM disk failure or `docker compose down -v` = acceptable data loss; export CLI is the escape hatch.

## Migration-ready architecture

Short-term self-hosted stack, designed so the swap to managed services costs ~1 week, not a rewrite.

1. **`store.Store` Go interface** is the seam (`server/internal/store/store.go`).
2. **`gocb`** import only in `internal/store/couchbase/`. **`go-redis`** only in `internal/store/redis/`. Composed impl wires both.
3. **Stable doc shapes** — flat JSON (no Couchbase-specific stored procedures, no Redis Lua).
4. **`memstore`** impl ships alongside as test backend + proof of seam.
5. **`cmd/dleague-export`** streams every persistent doc as JSONL — same seed for future imports anywhere.

## Out of scope (post-beta)

- Production-grade backups (current beta accepts data loss).
- Real-time spectator mode for sync PvP.
- Cross-region presence / multi-VM deployment.
- Couchbase CE → Capella (paid) migration: optional (operational, not legal); decided based on usage data, ops budget, or hitting CE's 5-node / 4-core caps.
- Early-adopter reward mechanism for beta testers.

## License & legal

- **Proprietary** — All Rights Reserved (see `LICENSE`).
- **Couchbase Community Edition** is governed by the [CE License Agreement](https://www.couchbase.com/community-license-agreement/). It explicitly permits commercial use ("develop or commercialize products that interact with the Community Software"). Hard caps: ≤ 5 nodes, ≤ 4 cores/node, no XDCR. See `docs/migration-readiness.md` § License watchout.

## Active plan

[`plans/260505-1604-firebase-couchbase-redis-pivot/plan.md`](../plans/260505-1604-firebase-couchbase-redis-pivot/plan.md)
