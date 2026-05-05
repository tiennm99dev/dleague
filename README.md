# Dleague

> **League of -dle games.** A PvP twist on Wordle, LoLdle, and friends — race opponents in real-time or compete on shared daily puzzles.

## What is this

Most -dle games are solo: solve the daily puzzle, share your score on Twitter, done. Dleague turns the genre into head-to-head competition.

- **Daily Leaderboard** — everyone plays the same puzzle; ranking by attempts then time
- **Challenge a Friend** — share a link, opponent plays the same seed, results compared
- **Quick Match** — matchmaking queue pairs you with a live opponent for a real-time race
- **Pluggable game types** — Wordle-style at launch; music, geography, image variants planned

## Status

Pre-implementation. Plan written, scope locked. See `plans/260505-0947-dleague-pvp-game/plan.md`.

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
| 1. Foundation & monorepo | 1w | pending |
| 2. Game core (pluggable -dle) | 2w | pending |
| 3. Backend + auth | 1.5w | pending |
| 4. Async PvP | 1.5w | pending |
| 5. Sync PvP (WebSocket) | 2w | pending |
| 6. Polish + deploy + mobile prep | 1.5w | pending |

## License

**Proprietary — All Rights Reserved.** See [LICENSE](LICENSE).

This is a private project. No copying, redistribution, modification, reverse
engineering, scraping, AI training, or clone derivatives without prior written
permission from the copyright holder.
