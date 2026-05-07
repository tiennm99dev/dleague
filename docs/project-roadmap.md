# Project Roadmap

Last updated: 2026-05-07.

## Status

No active implementation plan. All shipped + superseded plans live under
[`plans/archive/`](../plans/archive/). The codebase is on the **MongoDB
Atlas + Firebase Auth + Go server + Svelte/Phaser client** stack; the
remaining open work is operational (deploy, CI re-enable), not
implementation.

## Latest shipped initiative

[`plans/archive/260507-1648-mongodb-atlas-only-migration/`](../plans/archive/260507-1648-mongodb-atlas-only-migration/plan.md)
— consolidate the data plane onto MongoDB Atlas (M0 free tier).
**Phases 2–7 implemented 2026-05-07.** Phase 1 is code-side complete (smoke
test CLI, runbook, env wiring); cluster + secrets are external. Phase 6
data migration was skipped (no beta data to migrate). Couchbase + Redis
packages are deleted; `internal/store/mongodb/` is the live impl.

## Predecessor history

[`plans/archive/260505-1604-firebase-couchbase-redis-pivot/`](../plans/archive/260505-1604-firebase-couchbase-redis-pivot/plan.md)
— the self-hosted Couchbase + Redis stack that the Atlas plan superseded.
Phases 1–10 shipped (the `store.Store` seam, Firebase Auth, Svelte/Phaser
client, async + sync PvP); Phase 11 (Coolify deploy of that stack) was
skipped; Phase 12 cleanup folded into the successor plan's Phase 7.

[`plans/archive/260505-0947-dleague-pvp-game/`](../plans/archive/260505-0947-dleague-pvp-game/plan.md)
— the original master plan. Phase 1 (foundation/monorepo) shipped;
Phases 2–6 were superseded by the Couchbase+Redis pivot, which itself
was superseded by the Atlas consolidation.

Older deferred paths (MySQL HeatWave, Firebase platform pivot) are also
in `plans/archive/`.

## Open work (operational, not implementation)

- **GitHub CI re-enable** — `.github/workflows/ci.yml.disabled` still
  targets Go 1.26 + WASM client. Rewrite for Go 1.25.5 + Svelte/Phaser +
  Atlas: `go test ./...`, `make grep-isolation`, `make web-build`.
- **Coolify deploy on the OCI VM** — image build is ready
  (multi-arch via `make image-push`); live VM provisioning is external.
- **Firebase project + Atlas cluster provisioning** — both are signup
  flows captured in `docs/atlas-setup.md` and `docs/deployment-guide.md`.

## Post-beta milestones

- **Tech-stack reassessment** — review whether to migrate further (e.g.
  Atlas M0 → M10, or off Atlas entirely) once usage data exists.
  Decision criteria:
  - Daily active users threshold (~5k DAU as a "managed worth more" pivot).
  - Atlas storage / connection / sustained-ops cap headroom.
  - Operational time spent on infra per week.
- **Backups + replication** — currently beta accepts data loss; revisit
  before public launch (daily `mongodump` to OCI Object Store is the
  expected first step).
- **Observability** — request log middleware, Prometheus metrics,
  `slog`-based structured logging, Atlas connection-count + slow-query
  alerts.
- **Early-adopter rewards** — mechanism for redeeming the `isBetaTester`
  ledger; design depends on monetization model decisions.

## Known risks

- **Single-VM SPOF (Coolify host)** — VM dies → server down (data is
  safe in Atlas). Acceptable under beta posture.
- **M0 cluster scaling** — Atlas M0 has a 512 MB storage cap and a
  100-conn pool. If live data grows beyond ~300 MB or peak CCU exceeds
  ~50, upgrade to M10. See
  [`docs/migration-readiness.md`](./migration-readiness.md) for outbound
  recipe details.
- **Atlas M0 auto-pause after 30d inactivity** — beta `/health` keeps it
  warm; if we ever idle 30d, manual resume + 30–60s cold start.
