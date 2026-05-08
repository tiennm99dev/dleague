# Project Overview & PDR — Dleague

**Status:** skeleton — populated incrementally as phases land.

## Purpose
PvP twist on -dle puzzle games. Players race head-to-head (sync) or compete on shared daily puzzles (async). Single Wordle-style game at launch; pluggable architecture for music/geography/image variants.

## Vision
> League of -dle games. Take the daily-puzzle social loop (Wordle, LoLdle, Heardle) and add competitive PvP modes.

## Pillars
1. **Daily Leaderboard** — everyone plays the same puzzle; ranked by attempts then time.
2. **Challenge a Friend** — share a link; opponent plays the same seed; results compared.
3. **Quick Match** — matchmaking queue pairs against a live opponent; real-time race.
4. **Pluggable game types** — Wordle at launch; music, geography, image planned.

## Non-goals (v1)
- ELO / MMR ranked ladder
- Tournaments, brackets, spectator
- Multiple game variants live simultaneously
- Native mobile app store releases
- Cosmetics / monetization
- Localization

## Stack
See [`../README.md`](../README.md) Stack table — kept as the single source of truth.

## Roadmap
See [`development-roadmap.md`](development-roadmap.md). Active plan: [`../plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`](../plans/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md).

## Stakeholders
- **Owner / Eng:** TODO
- **Design:** TODO (Phase 06 lead)

## Success metrics (v1)
- [ ] WASM bundle replaced by Svelte JS bundle <400 KB gz (was: WASM <10 MB target)
- [ ] Sync match opponent-progress propagation p95 <200 ms
- [ ] Daily puzzle resets at consistent UTC midnight
- [ ] Atlas M0 handles MVP load (<8.6M ops/day budget)
- [ ] CI green on `go test -race`, `golangci-lint`, `buf lint`, `npm run build`

## Open questions
Tracked in active plan's §Unresolved Questions.

## Revision log
- 2026-05-08 — created (Phase 01).
- TODO — first revision after Phase 06 lands.
