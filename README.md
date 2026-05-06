# Dleague

> **League of -dle games.** A PvP twist on Wordle, LoLdle, and friends — race opponents in real-time or compete on shared daily puzzles.

## What is this

Most -dle games are solo: solve the daily puzzle, share your score on Twitter, done. Dleague turns the genre into head-to-head competition.

- **Daily Leaderboard** — everyone plays the same puzzle; ranking by attempts then time
- **Challenge a Friend** — share a link, opponent plays the same seed, results compared
- **Quick Match** — matchmaking queue pairs you with a live opponent for a real-time race
- **Pluggable game types** — Wordle-style at launch; music, geography, image variants planned

## Status

**Beta** — data may reset. Active plan: `plans/260505-1604-firebase-couchbase-redis-pivot/`. Phases 2–10 + 8 complete; Phase 11 (deploy) + Phase 12 (cleanup) in progress. See [`docs/project-roadmap.md`](docs/project-roadmap.md).

## Quickstart

```bash
# one-time: install buf + protoc-gen-go into $GOPATH/bin
make tools

# (re)generate protobuf Go code into shared/pb/
make proto-gen

# bring up Couchbase + Redis + Go server
docker compose up -d

# in another terminal: run the Svelte/Phaser client in dev mode
cd client/web && npm install && npm run dev
```

Server health: <http://localhost:8080/health>. Client dev server: <http://localhost:5173>.

## Stack

- **Web client:** Svelte 5 (shell + HUD) + Phaser 4 (game canvas), Vite, vitest
- **Mobile shell:** Capacitor (web first; iOS/Android later)
- **Auth:** Firebase Auth (Spark plan: Email/Google/Anonymous)
- **Backend:** Go 1.25.5 (`chi` HTTP + `nhooyr.io/websocket`)
- **Primary store:** Couchbase Community 8.0 (self-hosted)
- **Cache + leaderboards:** Redis 8.4 (self-hosted)
- **Hosting:** OCI Always-Free Ampere A1 Flex (4 OCPU + 24 GB RAM, ARM64) via Coolify

Migration-ready: every store backend lives behind a Go `Store` interface so a future swap to a managed service (Capella, Atlas, etc.) costs ~1 week. See [`docs/migration-readiness.md`](docs/migration-readiness.md).

## Beta posture

- Sign-in screens show a "Beta — data may reset" banner.
- Every user is tagged `isBetaTester: true` + `betaSignupAt` on first auth (early-adopter ledger).
- VM disk failure or `docker compose down -v` is acceptable data loss; `cmd/dleague-export` is the escape hatch.

## Repo layout

```
dleague/
├── client/web/         # Svelte 5 + Phaser 4 + Capacitor (active)
├── client/             # Ebitengine WASM (legacy; Phase 12 decision pending)
├── server/             # Go HTTP API + WebSocket hub + export CLI
├── shared/pb/          # Generated protobuf code
├── proto/              # .proto sources
├── web/                # Static legacy shell
├── docs/               # PDR, architecture, code standards, roadmap, migration
├── plans/              # Implementation plans (active + archived)
└── docker-compose.yml
```

## Plan

Active plan: [`plans/260505-1604-firebase-couchbase-redis-pivot/plan.md`](plans/260505-1604-firebase-couchbase-redis-pivot/plan.md). See [`docs/project-roadmap.md`](docs/project-roadmap.md) for current phase status.

## Credits & References

This project draws on patterns from the following permissively-licensed projects.
Where their code is incorporated, original copyright + license notices are
preserved per their license terms.

| Project | License | Used for |
|---------|---------|----------|
| [ratel-online/server](https://github.com/ratel-online/server), [/core](https://github.com/ratel-online/core), [/client](https://github.com/ratel-online/client) | MIT | Go networking patterns: client-server protocol, room-based multiplayer, message dispatch |
| [hajimehoshi/ebiten](https://github.com/hajimehoshi/ebiten) — official examples | Apache-2.0 | Game loop, scene management, grid-based input (`2048`, `blocks`), keyboard handling, animation |

## License

**Proprietary — All Rights Reserved.** See [LICENSE](LICENSE).

This is a private project. No copying, redistribution, modification, reverse
engineering, scraping, AI training, or clone derivatives without prior written
permission from the copyright holder.
