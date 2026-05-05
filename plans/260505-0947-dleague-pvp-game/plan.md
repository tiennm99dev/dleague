---
title: "Dleague — PvP -dle game (Ebitengine + Go)"
description: "PvP variant of -dle puzzle games (Wordle/LoLdle style). Single game at launch with pluggable architecture. Sync + async PvP. Ebitengine WASM client + Go backend. Web first, mobile Phase 2."
status: pending
priority: P2
effort: 8-12w
branch: main
tags: [ebitengine, go, websocket, postgres, wasm, pvp, puzzle-game, dle]
created: 2026-05-05
brainstorm: reports/brainstorm-260505-0947-dleague-pvp-game-naming.md
---

# Dleague Implementation Plan

## Goal

Ship a competitive PvP twist on -dle puzzle games. Players race head-to-head (sync) or compete on shared daily puzzles (async). Single Wordle-style game at launch, but architected so new -dle types plug in without rewrite.

## Stack (HARD)

- **Client:** Ebitengine (Go → WASM for web). Hybrid HTML/CSS overlay for text input + canvas for animations
- **Backend:** Go (`chi` for static + WS upgrade only). All game/auth/match messages over **single WebSocket** transport
- **Wire format:** **Binary protobuf** (`proto.Marshal/Unmarshal`) via `buf`-generated Go (`proto/dleague/v1/*.proto` → `shared/pb/`). Generated `.pb.go` files **committed** to git
- **Debug logging:** When built with `-tags debug`, every WS message is also serialized via `protojson` and logged to browser console (client) / stdout (server). Production builds exclude `protojson` entirely (smaller bundle, ~700KB → ~400KB)
- **No gRPC, no Envoy.** Plain WebSocket carrying protobuf-encoded messages. Drops bundle weight + ops complexity for a -dle game
- **DB:** Postgres (users, sessions, games, matches, attempts, leaderboards)
- **Auth:** session cookie set on HTML page load → bound to WS connection at upgrade
- **Deploy:** Fly.io / Railway for backend; static WASM bundle on Cloudflare Pages or same backend
- **Mobile:** gomobile bindings (Phase 6 stub, full build deferred)
- **Repo:** monorepo, Go workspaces, kebab-case dirs, files <200 LOC

## Constraints (HARD)

- KISS / YAGNI / DRY — no premature optimization
- No ranked ELO at launch (use win/loss + simple leaderboard); add ranked v2
- No monetization at launch (free, no ads, no IAP); revisit post-validation
- Each Go file <200 LOC, split modules early
- WASM bundle target: <10MB gzipped

## Phases

| # | Phase | File | Status | Effort |
|---|-------|------|--------|--------|
| 1 | Foundation & monorepo | [phase-01-foundation-monorepo.md](phase-01-foundation-monorepo.md) | completed | 1w |
| 2 | Game core (pluggable -dle interface) | [phase-02-game-core-pluggable.md](phase-02-game-core-pluggable.md) | pending | 2w |
| 3 | Backend + auth | [phase-03-backend-auth.md](phase-03-backend-auth.md) | pending | 1.5w |
| 4 | Async PvP | [phase-04-async-pvp.md](phase-04-async-pvp.md) | pending | 1.5w |
| 5 | Sync PvP (WebSocket) | [phase-05-sync-pvp-websocket.md](phase-05-sync-pvp-websocket.md) | pending | 2w |
| 6 | Polish + deploy + mobile prep | [phase-06-polish-deploy-mobile.md](phase-06-polish-deploy-mobile.md) | pending | 1.5w |

**Total estimated effort:** 9.5 weeks (~2.5 months solo)

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| PvP mode | Sync + async at launch | User chose both — async for retention, sync for engagement |
| MVP scope | Single game, platform-ready arch | User chose middle ground — ship fast, scale later |
| Engine | Ebitengine (Go→WASM) | User pick — enables web→mobile from one codebase |
| Backend lang | Go | Matches engine, single-language stack |
| Game type at launch | Wordle-style word guessing | Most familiar, validates PvP loop fastest |
| Ranked system | Simple win/loss + leaderboard | YAGNI — no ELO until volume proves need |
| Monetization | None at MVP | Validate engagement first |
| Wire format | Binary protobuf + build-tag JSON debug logging | Smallest production bundle (~400KB protobuf-go alone), zero parsing overhead; debug builds add protojson for human-readable console logs |
| Generated code | `.pb.go` committed to git | Consumers don't need protoc/buf installed to build. CI verifies regeneration produces no diff |
| Transport | Single WebSocket | One channel for auth, game, match, sync. HTTP only serves static + upgrade. Simpler client, fewer reconnect paths |

### Plan revision impact (post-xia)

Phase 3-5 phase files reflect the original HTTP-REST + WS-for-sync design. Single-WS pivot means:
- Phase 3: most "endpoints" become WS message handlers; HTTP layer reduced to static + upgrade
- Phase 4-5: share dispatch infra under `server/internal/ws/handlers/`
- Recommend re-scoping Phase 3-5 after Phase 1+2 ship. Existing files remain valid as feature spec; only transport binding changes

## Dependencies

- Domain + trademark + social handles for "Dleague" — homework before launch (see brainstorm report)
- Postgres host (Fly.io managed PG, Neon, or self-hosted)
- Wordlist for Wordle-style game (open-source list, e.g. Wordle's original 2315 answer list)

## Open Risks

- WASM bundle size — Ebitengine baseline ~5-8MB, must monitor
- Canvas accessibility — screen reader users blocked unless HTML overlay path implemented
- Sync PvP fairness — network latency affects "first to solve" calls; need server-authoritative timing
- Daily puzzle generation — needs scheduled job + timezone strategy

## Success Criteria

- [ ] User can play Wordle-style game solo
- [ ] User can challenge friend via shareable link (async)
- [ ] User can match into queue and race live opponent (sync)
- [ ] Daily puzzle resets at consistent UTC midnight
- [ ] Leaderboard shows top players by wins
- [ ] WASM bundle <10MB gzipped
- [ ] Plan ready to extend with new -dle game type (music, geography) without rewriting core
- [ ] Mobile stub compiles via gomobile (no app store yet)

## Out of Scope (v2+)

- ELO/MMR ranked ladder
- Multiple -dle game variants (music, geography, image)
- Tournaments / brackets
- Cosmetics / monetization
- Friends list / chat
- Native mobile app store releases
- Localization (Vietnamese, etc.)
