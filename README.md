# Dleague

> **League of -dle games.** A PvP twist on Wordle, LoLdle, and friends — race opponents in real-time or compete on shared daily puzzles.

## What is this

Most -dle games are solo: solve the daily puzzle, share your score on Twitter, done. Dleague turns the genre into head-to-head competition.

- **Daily Leaderboard** — everyone plays the same puzzle; ranking by attempts then time
- **Challenge a Friend** — share a link, opponent plays the same seed, results compared
- **Quick Match** — matchmaking queue pairs you with a live opponent for a real-time race
- **Pluggable game types** — Wordle-style at launch; music, geography, image variants planned

## Status

Phase 1 (Go workspace + protobuf wire + WS ping-pong + `/health`) shipped. Pivoting to **Svelte+Phaser client + MongoDB Atlas + Firebase Auth**. See active plan: [`plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`](plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md).

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

| Layer  | Tech                                                                                  |
|--------|---------------------------------------------------------------------------------------|
| Client | [SvelteKit](https://kit.svelte.dev/) (adapter-static) + [Phaser 3.80+](https://phaser.io/) + [`@bufbuild/protobuf-es`](https://github.com/bufbuild/protobuf-es) |
| Server | Go (`chi` HTTP + [`coder/websocket`](https://github.com/coder/websocket) — replaces archived `nhooyr.io/websocket`) |
| Auth   | [Firebase Auth](https://firebase.google.com/products/auth) — Email/Password + Google + Anonymous; ID token over WS |
| DB     | [MongoDB Atlas M0](https://www.mongodb.com/atlas) (free tier; replica-set transactions) |
| Wire   | Binary protobuf over single WebSocket (`-tags debug` adds protojson logging)          |
| Deploy | [Fly.io](https://fly.io/) for Go server; static `web/dist/` served by the same binary |

## Repo layout (planned)

```
dleague/
├── web/              # SvelteKit + Phaser client (vite dev server, static dist)
├── server/           # Go HTTP + WebSocket hub + Mongo store + Firebase verifier
├── shared/           # Game interface, DTOs, pluggable -dle registry
│   └── pb/           # Generated Go protobuf (committed)
├── proto/            # Protobuf schema + buf config (gen Go + TS)
├── docs/             # PDR, code standards, system architecture, deployment, roadmap, changelog
├── plans/            # active plan + plans/archive/ for superseded work
├── firebase.json     # Firebase emulator config (local dev)
└── docker-compose.yml  # mongo:7 + mongo-express for local dev
```

## Plan

Active plan: [`plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`](plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md)

| #  | Phase                              | Effort | Status    |
|----|------------------------------------|--------|-----------|
| 00 | Phase 1 foundation (Go + WS + pb)  | 1w     | completed |
| 01 | Archive + docs bootstrap           | 0.5w   | pending   |
| 02 | Server hardening                   | 1w     | pending   |
| 03 | WS lib migration nhooyr → coder    | 0.5w   | pending   |
| 04 | MongoDB store rewrite              | 1w     | pending   |
| 05 | Firebase Auth integration          | 1w     | pending   |
| 06 | Svelte+Phaser client scaffold      | 1.5w   | pending   |
| 07 | Game core pluggable + Wordle       | 2w     | pending   |
| 08 | Async PvP                          | 1w     | pending   |
| 09 | Sync PvP                           | 1.5w   | pending   |
| 10 | Deploy + polish                    | 1w     | pending   |

Superseded plans live under [`plans/archive/`](plans/archive/README.md).

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
