# Project Roadmap

Last updated: 2026-05-07.

## Active plan

[`plans/260507-1648-mongodb-atlas-only-migration/plan.md`](../plans/260507-1648-mongodb-atlas-only-migration/plan.md) — consolidate Couchbase + Redis onto MongoDB Atlas (M0 free tier). 7 phases, ~5–6d effort. **Phases 2–7 implemented 2026-05-07** (Atlas provisioning is the only external step remaining; Phase 6 data-migration skipped per "no beta data deployed yet"). Supersedes Phase 11 deploy of the previous plan.

| # | Phase | Status |
|---|-------|--------|
| 1 | Atlas provisioning + env wiring | code-side complete (smoke-test CLI, runbook, env); cluster + secrets external |
| 2 | mongodb scaffold (client + indexes + integration test) | completed |
| 3 | Persistent half port (users, puzzles, attempts, matches, Export) | completed |
| 4 | Cache half port (leaderboards $max, presence + cache TTL) | completed |
| 5 | Wiring swap (`cmd/api/main.go`); delete `composed/` | completed |
| 6 | Data migration | **skipped** (no beta data) |
| 7 | Cleanup + docs + supersession | completed |

## Predecessor plan (data-plane history)

[`plans/260505-1604-firebase-couchbase-redis-pivot/plan.md`](../plans/260505-1604-firebase-couchbase-redis-pivot/plan.md) — Phases 1–10 shipped (the `store.Store` seam, Firebase Auth, Svelte/Phaser client, async + sync PvP). Phase 11 (Coolify deploy with Couchbase+Redis) skipped in favor of the Atlas-only plan above. Phase 12 cleanup folded into the successor plan's Phase 7. **Marked superseded 2026-05-07.**

| # | Phase | Status |
|---|-------|--------|
| 1 | Provisioning (Firebase + docker-compose) | code-side complete; VM provisioning external |
| 2 | Strip MySQL + revise config + Go 1.25.5 downgrade | completed |
| 3 | Couchbase 8.0 Go integration | code-side complete; live integration test gated by `COUCHBASE_TEST_CONN` |
| 4 | Redis 8.4 Go integration | code-side complete; live integration test gated by `REDIS_TEST_ADDR` |
| 5 | Firebase Admin SDK + token verifier | code-side complete; live token tests gated externally |
| 6 | Wire-format auth handshake | completed |
| 7 | Svelte 5 + Phaser 4 + Capacitor client scaffold | completed |
| 8 | Pluggable Wordle on Phaser 4 | **completed (this iteration)** |
| 9 | Async PvP via store.Store | completed |
| 10 | Sync PvP via Go WS | completed |
| 11 | Deploy on Coolify | pending (out of scope for local-only iteration) |
| 12 | Supersession + cleanup + migration-export CLI | non-deploy work complete (export CLI shipped, docs sweep done, residual-ref audit clean as of 2026-05-07); deploy-guide finalization blocked on Phase 11 |

## Recently shipped

- **MongoDB Atlas consolidation implemented (2026-05-07)** — Phases 2–7 of `260507-1648-mongodb-atlas-only-migration/` shipped in one session: new `internal/store/mongodb/` package (10 store methods + indexes + integration test gated by `MONGODB_TEST_URI`), `cmd/api` rewired to single-backend, `composed/` + `couchbase/` + `redis/` + `cmd/dleague-export/` deleted, docker-compose drops both DB containers, env vars collapse to `MONGODB_URI`, every doc in `docs/` refreshed, predecessor plan marked superseded. `make grep-isolation` enforces the new boundary. `go build`, `go vet`, `go test ./...` all green.
- **MongoDB Atlas-only plan drafted (2026-05-07)** — backed by 2 researcher reports + 1 brainstorm + 1 code-reviewer red-team review. Brainstormer recommended status quo (75% confidence); user decided to migrate; red-team review surfaced 3 BLOCKING issues (Phase 6 sequencing, importer upsert filters, `time.Time` round-trip) + 3 HIGH issues — all addressed in plan revision before any code landed.
- **Phase 12 audit pass (2026-05-07)** — verified the predecessor plan's Phase 12 success criteria; cleanup work absorbed into the new plan's Phase 7.
- **Phase 8 (Pluggable Wordle)** — `client/web/src/games/{types,registry}.ts`, `runner/GameRunner.svelte`, `wordle/{WordleScene.ts,WordleHud.svelte,scoring.ts}`. Lobby integrates the runner. 14 unit tests pass; production build green.
- **Auth'd puzzle endpoint** — added `GET /api/v1/puzzles/me/today` and `/{date}` so authenticated clients can render per-guess color feedback. Public endpoint still hides the solution.
- **Doc sweep** — refreshed `code-standards.md`; created `project-overview-pdr.md`, `system-architecture.md`, `codebase-summary.md`, `migration-readiness.md`, `project-roadmap.md`.

## Up next (local-completable)

- **Phase 12 — re-enable GitHub CI**: rewrite `.github/workflows/ci.yml.disabled` for Go 1.25.5 + Svelte+Phaser client (drop WASM + MySQL refs).
- **Plan + phase status sync** — bump `phase-08` and partial `phase-12` to `completed`/`in_progress`.
- **Decision: drop legacy `client/` Ebitengine WASM** — done. Removed in Phase 12; `client/` now contains only `client/web/` (active). Recoverable via git history.

## Up next (requires external infra)

- **Phase 11 — Coolify deploy** (skipped in this iteration per user instruction).
- **Phase 1 external** — Firebase project signup + service-account JSON.
- **Phase 1 external** — ARM64 manifest verification on the OCI VM.
- ~~**Phase 12 license review**~~ — done 2026-05-06. CE License grants self-hosted commercial use; hard caps are 5 nodes / 4 cores per node / no XDCR. See `docs/migration-readiness.md` § License watchout.

## Post-beta milestones

- **Tech-stack reassessment** — review whether to migrate to managed services (Capella, Mongo Atlas, managed Redis) once usage data exists. Decision criteria:
  - Daily active users threshold for "managed worth it" (~5k DAU).
  - Cost ceiling for self-hosted (currently OCI Always-Free).
  - Operational time spent on infra per week.
- **Backups + replication** — currently beta accepts data loss; revisit before public launch.
- **Observability** — request log middleware, Prometheus metrics, slog-based structured logging.
- **Early-adopter rewards** — mechanism for redeeming the `isBetaTester` ledger; design depends on monetization model decisions.

## Known risks

- **Single-VM SPOF** — VM dies → all data gone. Acceptable under beta posture; `mongodump` is the escape hatch.
- **M0 cluster scaling** — Atlas M0 (free tier) has a 512 MB storage cap. If live data grows beyond ~300 MB, upgrade to M10 (paid). See `docs/migration-readiness.md` § Outbound recipe for details on migration costs.
