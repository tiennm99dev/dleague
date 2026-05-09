# Project Overview & PDR — Dleague

## Product Goal

Dleague turns solo -dle games into head-to-head competition. Most Wordle-style
games are solitary: solve the daily puzzle, post your score, done. Dleague adds
real-time matchmaking, challenge links, and a shared daily leaderboard so players
compete directly rather than comparing grid emojis on social media.

## Pillars

| Pillar | Description |
|--------|-------------|
| **Daily Leaderboard** | Everyone plays the same puzzle each day. Rankings by attempts, then time. Anonymous users excluded. |
| **Challenge a Friend** | Generate a share link; opponent plays the same seed; results compared side-by-side. Async — no live connection required. |
| **Quick Match** | In-memory matchmaking queue pairs two players for a synchronous, real-time race over WebSocket. |
| **Game** | Wordle is the only shipped game. The `shared/game` package contains a reserved (currently inactive) scaffold for future game types. |

## MVP Scope

| In scope | Out of scope (v2+) |
|----------|--------------------|
| Wordle game type only | Additional -dle types (music, geography, image) |
| Email/Password + Google + Anonymous sign-in | Apple sign-in, SSO |
| Web client (SvelteKit + Phaser) | Native iOS / Android |
| Fly.io single-region deploy (iad) | Multi-region, CDN edge |
| MongoDB Atlas M0 (free tier) | M10+ with VPC peering |
| Per-day leaderboard | Historical cross-day rankings |
| Challenge link (async PvP) | Tournaments, brackets |
| Quick match (sync PvP, 2 players) | Spectator, team modes |
| Admin claim + CLI for moderation plumbing | Full moderation dashboard |

## Success Metrics (MVP)

| Metric | Target |
|--------|--------|
| Daily active users (Week 1) | > 10 (internal) |
| Average match completion rate | > 80% (both players finish) |
| Server uptime | > 99.5% (Fly.io SLA) |
| Health check p99 | < 200 ms |
| WS round-trip p99 | < 100 ms (same-region) |
| Bundle size (gzip) | < 450 KB (target 400 KB) |
| Test coverage — game logic | > 80% |
| Test coverage — WS handlers | > 70% |

## Constraints

- **Free tier first:** All infrastructure (Atlas M0, Firebase free tier, Fly.io hobby)
  must fit within free-tier limits at MVP launch.
- **No WASM:** Dropped in Phase 06 (bundle size + complexity). Pure JS client.
- **Server-authoritative:** Clients never see the solution until game ends.
  Score calculation happens server-side only.
- **Single WebSocket per client:** All game, auth, and match messages travel
  over one `/ws` connection. No REST game endpoints.
- **Stateless server:** No sticky sessions; reconnect restores state from Mongo.
  In-memory sync match state is ephemeral (grace timer handles reconnects).

## Technical Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Client framework | SvelteKit + adapter-static | Minimal bundle, SSR optional, fast DX |
| Game rendering | Phaser 3 canvas | Rich animation; Svelte owns DOM state |
| Auth | Firebase Auth | Zero-cost at MVP scale; proven JWT flow |
| Database | MongoDB Atlas M0 | Free replica set with transactions; Go driver v2 |
| WS library | coder/websocket | Maintained fork of archived nhooyr; minimal API |
| Deploy | Fly.io | Simple Docker deploy; free hobby tier; `iad` ↔ Atlas us-east-1 |
| Wire format | Binary protobuf | Type-safe; compact; `buf generate` for Go + TS |
| Game interface | Go interface + Registry | Reserved scaffold for future game types (v2); not wired in current release |
| Session auth | ID token in Sec-WebSocket-Protocol | Token refresh without re-upgrading |

## References

- Build plan (archived 2026-05-09): `plans/archive/260508-2300-svelte-phaser-firebase-mongo-pivot/plan.md`
- Active plan: `plans/260509-1331-improvement-plan/plan.md` (post-MVP hardening)
- Architecture: `docs/system-architecture.md`
- Code standards: `docs/code-standards.md`
- Deployment: `docs/deployment-guide.md`
