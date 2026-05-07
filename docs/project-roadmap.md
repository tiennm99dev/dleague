# Project Roadmap

Last updated: 2026-05-07.

## Active plan

[`plans/260505-1604-firebase-couchbase-redis-pivot/plan.md`](../plans/260505-1604-firebase-couchbase-redis-pivot/plan.md)

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

- **Phase 12 audit pass (2026-05-07)** — verified Phase 12 success criteria: residual `mysql`/`heatwave`/`firestore` sweep clean (only "removed/Strip MySQL" historical framing in live docs; `firestore` is indirect transitive of Firebase Admin SDK); fresh-contributor onboarding via README quickstart confirmed; roadmap refreshed. Phase 12 stays `in_progress` solely on the deploy-guide tail (blocked on Phase 11).
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

- **Couchbase Community license** — CE License Agreement permits commercial use; the hard caps are 5 nodes / 4 cores per node / no XDCR. Crossing any cap means moving to Capella or Enterprise. See `docs/migration-readiness.md` § License watchout.
- **Single-VM SPOF** — VM dies → all data gone. Acceptable under beta posture; export CLI is the escape hatch.
- **ARM64 image availability** — Couchbase 8.0 CE ARM64 manifest must be verified at deploy time.
