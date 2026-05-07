# Dleague

> **League of -dle games.** A PvP twist on Wordle, LoLdle, and friends — race opponents in real-time or compete on shared daily puzzles.

## What is this

Most -dle games are solo: solve the daily puzzle, share your score on Twitter, done. Dleague turns the genre into head-to-head competition.

- **Daily Leaderboard** — everyone plays the same puzzle; ranking by attempts then time
- **Challenge a Friend** — share a link, opponent plays the same seed, results compared
- **Quick Match** — matchmaking queue pairs you with a live opponent for a real-time race
- **Pluggable game types** — Wordle-style at launch; music, geography, image variants planned

## Status

**Beta** — data may reset. Active plan: `plans/260507-1648-mongodb-atlas-only-migration/`. Data plane consolidated to MongoDB Atlas (M0 free tier). See [`docs/project-roadmap.md`](docs/project-roadmap.md).

## Quickstart

```bash
# one-time: install buf + protoc-gen-go into $GOPATH/bin
make tools

# (re)generate protobuf Go code into shared/pb/
make proto-gen

# Provision MongoDB Atlas (one-time): docs/atlas-setup.md
# Then copy .env.example → .env and fill in MONGODB_URI + Firebase creds.
cp .env.example .env

# Smoke test Atlas connectivity
make atlas-smoke

# bring up the dleague Go server (data plane is Atlas — no local DB containers)
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
- **Data plane:** MongoDB Atlas M0 (free, AWS Singapore) — documents + leaderboards (`$max` + index) + presence/cache (TTL indexes)
- **Hosting:** OCI Always-Free Ampere A1 Flex (4 OCPU + 24 GB RAM, ARM64) via Coolify

Migration-ready: the store backend lives behind a Go `Store` interface (`server/internal/store/store.go`); `make grep-isolation` enforces the boundary. A future swap costs days, not weeks. See [`docs/migration-readiness.md`](docs/migration-readiness.md).

## Beta posture

- Sign-in screens show a "Beta — data may reset" banner.
- Every user is tagged `isBetaTester: true` + `betaSignupAt` on first auth (early-adopter ledger).
- Atlas M0 wipe is acceptable data loss; `mongodump --uri "$MONGODB_URI"` is the escape hatch.

## Repo layout

```
dleague/
├── client/web/         # Svelte 5 + Phaser 4 + Capacitor (active web client)
├── server/             # Go HTTP API + WebSocket hub + export CLI
├── shared/pb/          # Generated protobuf code
├── proto/              # .proto sources
├── web/                # Static legacy shell
├── docs/               # PDR, architecture, code standards, roadmap, migration
├── plans/              # Implementation plans (active + archived)
└── docker-compose.yml
```

## Plan

Active plan: [`plans/260507-1648-mongodb-atlas-only-migration/plan.md`](plans/260507-1648-mongodb-atlas-only-migration/plan.md). See [`docs/project-roadmap.md`](docs/project-roadmap.md) for current phase status.

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
