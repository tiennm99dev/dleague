# Development Roadmap

**Status:** living — updated as phases complete.

## Active plan
[`../plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`](../plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md)

## Phases
| #  | Phase                              | Effort | Status    | Notes |
|----|------------------------------------|--------|-----------|-------|
| 00 | Foundation (Go + WS + protobuf)    | 1w     | completed | Commit `9937c7d`; archived plan |
| 01 | Archive + docs bootstrap           | 0.5w   | in-progress | Pivot drift cleanup |
| 02 | Server hardening                   | 1w     | pending   | Folds in code-review H1-H3 + security H2-H3 |
| 03 | WS lib migration nhooyr → coder    | 0.5w   | pending   | Archived dep (security-review L1) |
| 04 | MongoDB store rewrite              | 1w     | pending   | Replace MySQL store |
| 05 | Firebase Auth integration          | 1w     | pending   | `Sec-WebSocket-Protocol` upgrade gate |
| 06 | Svelte+Phaser client scaffold      | 1.5w   | pending   | Replaces Ebitengine client |
| 07 | Game core (pluggable + Wordle)     | 2w     | pending   | Server-authoritative |
| 08 | Async PvP                          | 1w     | pending   | Challenge link + daily leaderboard |
| 09 | Sync PvP                           | 1.5w   | pending   | Matchmaking + Mongo txn |
| 10 | Deploy + polish                    | 1w     | pending   | Fly.io + final docs sweep |

**Total:** ~10-12 weeks (~3 months solo).

## Milestones
- **M1** (Phase 04 done) — Server hardened + Mongo store live + WS lib migrated.
- **M2** (Phase 06 done) — End-to-end auth + new client speaking WS.
- **M3** (Phase 07 done) — Single-player Wordle playable in browser.
- **M4** (Phase 09 done) — Async + sync PvP both functional.
- **M5** (Phase 10 done) — Production on Fly.io with all docs current.

## Out of scope (v2+)
- Native mobile (gomobile / Capacitor)
- Custom-claims roles beyond admin
- Redis pub-sub for multi-region matchmaking
- Mongo Change Streams for live leaderboard
- Full-text search
- Backups (M0 lacks them)
- SMS auth, tournament brackets, spectator mode

## History
- **2026-05-05** — Original plan created (Ebitengine + Postgres). Phase 1 shipped.
- **2026-05-05** — DB pivot to MySQL HeatWave (commit `be9e68b`).
- **2026-05-08** — Stack pivot decision recorded; Svelte+Phaser + Mongo Atlas + Firebase Auth chosen.
- **2026-05-08** — Three prior plans archived; new plan adopted.
