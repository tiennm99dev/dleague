# Development Roadmap

## Active Plan: Svelte + Phaser + Firebase + MongoDB Pivot

Plan directory: `plans/260508-2300-svelte-phaser-firebase-mongo-pivot/`

### Phase Status

| # | Phase | Commit | Status |
|---|-------|--------|--------|
| 00 | Phase 1 foundation — Go workspace + protobuf wire + /health + WS ping-pong | `9937c7d` | completed |
| 01 | Archive + docs bootstrap | `1819359` | completed |
| 02 | Server hardening — concurrency + security + rate limiting | `ea677a3` | completed |
| 03 | WS lib migration nhooyr → coder/websocket | `f5b9c9d` | completed |
| 04 | MongoDB store rewrite — repos + EnsureIndexes + models | `54a037d` | completed |
| 05 | Firebase Auth integration — ID token verify + UpsertByUID | `dd89f9c` | completed |
| 06 | SvelteKit + Phaser client scaffold — drop Ebitengine WASM | `5f837ea` | completed |
| 07 | Game core pluggable + server-authoritative Wordle | `a4b3762` | completed |
| 08 | Async PvP — challenge link + daily leaderboard | `9f66faa` | completed |
| 09 | Sync PvP — live race over WebSocket | `70c8904` | completed |
| 10 | Deploy + polish — Fly.io infra + carryover fixes + docs sweep | pending commit | completed |

---

## v2 Backlog

Features deferred from MVP. Prioritized by impact:

### High priority
- **Real wordlist (2315 words):** Replace 772-word placeholder with the
  permissively-licensed NYT Wordle list (or equivalent public-domain set).
  Blocked on license verification — do not embed any list without clear
  permissive license.
- **Redis pub/sub matchmaking:** Replace in-memory `Queue` with Redis
  Streams/pub-sub for multi-region or multi-instance deployments.
- **MongoDB Change Streams for live leaderboard:** Replace 5-min scheduler
  refresh with real-time leaderboard updates via Atlas Change Streams.
- **M10 + VPC Peering:** Upgrade Atlas M0 → M10 when storage > 70% or ops > 50/s;
  add private endpoint to close the `0.0.0.0/0` allowlist accepted-risk.

### Medium priority
- **Tournament brackets:** Bracket-style elimination tournaments with seeding,
  match scheduling, and result tracking.
- **Spectator mode:** Read-only WebSocket stream for watching live matches;
  opponent letters still hidden until game resolves.
- **Full-text search:** Atlas Search index on `display_name` for user lookup and
  friend challenges.
- **Custom Phaser build:** Strip audio, physics, and unused Phaser plugins to
  bring bundle from ~341 KB → ~240 KB gzip. Saves ~100 KB against 400 KB budget.
- **Admin dashboard:** Web UI for moderators using `Conn.isAdmin` flag (plumbing
  already in place from Phase 10).

### Lower priority / exploratory
- **Native mobile (Capacitor):** Wrap SvelteKit app in Capacitor for iOS/Android
  distribution. Phaser canvas works in WebView.
- **Additional -dle game types:** Music (hum the note), geography (map tile),
  image (reveal by quadrant). Pluggable via `shared/game.Game` interface.
- **Custom-claims roles beyond admin:** Moderator, verified-creator, tournament-host.
- **`VerifyIDTokenAndCheckRevoked` on WS upgrade:** Currently only on admin ops.
  Enable for all connections when revocation latency is acceptable (~1 s extra).
- **Mongo aggregation pipeline for leaderboards:** Replace Go-side join in
  `leaderboards.go` with `$lookup` + `$group` pipeline for scalability.
- **200-connection load test:** Go test spawning 200 mock WS connections running
  full match loops, asserting no goroutine leaks under `-race`.
- **Structured JSON logging:** Replace `log.Printf` with `slog` (Go 1.21+) for
  queryable production logs in Fly.io log drain.
- **Light mode:** CSS custom-property swap; currently dark-mode only.

---

## Architecture Evolution Triggers

| Condition | Action |
|-----------|--------|
| Atlas M0 storage > 70% (360 MB) | Migrate to M10 ($57/mo) |
| Concurrent WS conns > 300 sustained | Add second Fly machine |
| Match volume > 50 ops/sec | Add Redis for queue |
| Leaderboard refresh lag > 10 s | Add Change Streams |
| Bundle size > 450 KB gzip | Phaser custom build |
