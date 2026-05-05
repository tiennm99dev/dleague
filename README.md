# Dleague

> **League of -dle games.** A PvP twist on Wordle, LoLdle, and friends — race opponents in real-time or compete on shared daily puzzles.

## What is this

Most -dle games are solo: solve the daily puzzle, share your score on Twitter, done. Dleague turns the genre into head-to-head competition.

- **Daily Leaderboard** — everyone plays the same puzzle; ranking by attempts then time
- **Challenge a Friend** — share a link, opponent plays the same seed, results compared
- **Quick Match** — matchmaking queue pairs you with a live opponent for a real-time race
- **Pluggable game types** — Wordle-style at launch; music, geography, image variants planned

## Status

Phase 1 completed — foundation scaffolded. Phase 2 ready. See `plans/260505-0947-dleague-pvp-game/plan.md`.

## Quickstart

```bash
# one-time: install buf + protoc-gen-go into $GOPATH/bin
make tools

# (re)generate protobuf Go code into shared/pb/
make proto-gen

# run server + WASM client (prod)
make dev

# run with -tags debug — every WS message logs as protojson on both sides
make dev-debug
```

Open http://localhost:8080.

## Stack

- **Client:** [Ebitengine](https://ebitengine.org/) (Go → WASM for web), HTML/CSS overlay for input + canvas for animations
- **Backend:** Go (`chi` HTTP + `nhooyr.io/websocket` for sync PvP)
- **DB:** Postgres
- **Deploy:** Fly.io (web first), gomobile compile target prepped for Phase 2 mobile launch

## Repo layout (planned)

```
dleague/
├── client/           # Ebitengine WASM + mobile entry
├── server/           # Go HTTP API + WebSocket hub
├── shared/           # Game interface, DTOs, pluggable -dle registry
├── web/              # static HTML shell + CSS overlay
├── docs/             # design docs, deployment guide
├── plans/            # implementation plans (this is where you are now)
└── docker-compose.yml
```

## Plan

Active plan: [`plans/260505-0947-dleague-pvp-game/plan.md`](plans/260505-0947-dleague-pvp-game/plan.md)

| Phase | Effort | Status |
|-------|--------|--------|
| 1. Foundation & monorepo | 1w | completed |
| 2. Game core (pluggable -dle) | 2w | pending |
| 3. Backend + auth | 1.5w | pending |
| 4. Async PvP | 1.5w | pending |
| 5. Sync PvP (WebSocket) | 2w | pending |
| 6. Polish + deploy + mobile prep | 1.5w | pending |

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
