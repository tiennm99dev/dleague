---
title: "MySQL HeatWave Always-Free integration"
description: "Provision OCI MySQL HeatWave Always-Free, wire it into the dleague Go server as the Phase 3 data layer, automate backups, and document the migration off the placeholder Postgres dev compose."
status: superseded
superseded_by: ../260505-1604-firebase-couchbase-redis-pivot/plan.md
priority: P1
effort: 1w
branch: main
tags: [oci, mysql, heatwave, infrastructure, phase-3-prereq, superseded]
created: 2026-05-05
parent_plan: 260505-0947-dleague-pvp-game/plan.md
research:
  - reports/researcher-260505-1207-aiven-vs-cockroach.md
  - reports/researcher-260505-1207-cockroach-fitness-followup.md
  - reports/researcher-260505-1207-nosql-candidates.md
  - reports/researcher-260505-1253-oci-databases.md
  - reports/researcher-260505-1308-mysql-heatwave-deep-dive.md
---

# MySQL HeatWave Always-Free integration

> **Superseded by:** [`260505-1604-firebase-couchbase-redis-pivot/`](../../260505-1604-firebase-couchbase-redis-pivot/plan.md). MySQL HeatWave Always-Free was abandoned during the platform pivot — the project moved to a document-store model (Couchbase 8.0) on the same Coolify VM, eliminating cross-cloud network hops and the relational schema overhead. Kept here as historical context for the OCI/MySQL evaluation.

## Goal

Provision and integrate **OCI MySQL HeatWave Always-Free** as the dleague backend's data layer. Replace the placeholder Postgres dev container, scaffold a Go `store` package, design the MySQL-flavored schema for Phase 3 auth + game tables, and automate backups beyond Oracle's 1-day retention.

## Why now

Phase 3 (`plans/260505-0947-dleague-pvp-game/phase-03-backend-auth.md`) is blocked on a DB decision. Decision is committed: MySQL HeatWave Always-Free wins on co-location with the Coolify VM (sub-ms latency), 50 GB storage, all-OCI-regions coverage including Tokyo, and zero third-party vendor sprawl. See research reports listed in frontmatter for the full chain.

## Scope (HARD)

- **Data layer + ops only.** This plan does NOT redesign Phase 3 auth flow. Schema design here feeds back into phase-03 as the MySQL-flavor concrete schema.
- **Target:** OCI Always-Free `MySQL.Free` shape, 50 GB primary storage, single 16 GB RAM node, MySQL 8.x LTS, private subnet of existing VCN.
- **No HA, no PITR, no read replicas** on free; design accepts these limits.
- **Migration tool:** plain embedded SQL via `embed.FS` + a small forward-only runner. No `goose` / `golang-migrate` dependency (KISS).

## Non-goals

- Local Postgres for dev (now obsolete; replaced with optional `mysql:8` dev container or skip entirely)
- HeatWave OLAP cluster (we are OLTP-only; do not provision)
- Cross-region replication, failover, multi-tenancy across OCI accounts
- Phase 3 auth code (sessions, registration, password hashing) — owned by phase-03

## Phases

| # | Phase | File | Status | Effort |
|---|-------|------|--------|--------|
| A | OCI infrastructure provisioning | [phase-a-oci-infrastructure.md](phase-a-oci-infrastructure.md) | pending | 0.5d |
| B | Empirical verification (idle + max_connections) | [phase-b-empirical-verification.md](phase-b-empirical-verification.md) | pending | 14d (mostly waiting) |
| C | Go integration scaffolding | [phase-c-go-integration-scaffolding.md](phase-c-go-integration-scaffolding.md) | pending | 1d |
| D | Schema migration design (MySQL dialect) | [phase-d-schema-migration-mysql.md](phase-d-schema-migration-mysql.md) | pending | 0.5d |
| E | Backup automation | [phase-e-backup-automation.md](phase-e-backup-automation.md) | pending | 0.5d |
| F | Documentation + cleanup | [phase-f-documentation-cleanup.md](phase-f-documentation-cleanup.md) | pending | 0.5d |

**Effective wall-clock:** ~3 working days of active work + 14 days of Phase B passive idle observation. Phase B can run in parallel with C/D/E/F because the result only blocks the **production cutover**, not the development scaffolding.

## Dependencies

- **External:** Oracle Cloud Infrastructure tenancy with Always-Free quota available; existing OCI Coolify VM (region must be confirmed by user — see Phase A risk register).
- **Internal:** Phase 1 of `260505-0947-dleague-pvp-game` shipped (Go workspace + server skeleton). Phase 2 (game core) does not need to be done first — the DB layer is independent.
- **Tools:** `mysqldump` and `oci` CLI on the Coolify VM for Phase E backup script.

## Open decisions / blockers

| Decision | Owner | Blocks |
|----------|-------|--------|
| **OCI region for the DB** (must match Coolify VM) | User | Phase A provisioning |
| Confirm idle-reclaim policy (14-day test) | This plan | Production cutover only |
| Confirm `max_connections` actual value | This plan | Phase C pool sizing override (default 25 should fit) |

## Success criteria (plan-level)

- [ ] MySQL HeatWave Always-Free running in private subnet of OCI VCN, reachable from Coolify VM at <5 ms RTT
- [ ] `dleague` schema created, owned by `dleague_app` user with least-privilege grants
- [ ] Go server `Store` package compiles, `db.Ping()` succeeds against the live DB
- [ ] `/health` endpoint surfaces DB status (degraded / healthy)
- [ ] Weekly `mysqldump` cron uploads to OCI Object Storage, retention 6 weeks
- [ ] `docs/code-standards.md` documents MySQL idioms used by the project
- [ ] `phase-03-backend-auth.md` reflects MySQL-flavor schema (no more `citext`, `uuid` PKs are `BINARY(16)`)
- [ ] 14-day idle test result documented in Phase B (RUNNING ✓ or keepalive cron added)

## Risk register (plan-level — phase files have local risks)

| Risk | Severity | Mitigation |
|------|----------|------------|
| 1-day automatic backup retention | Med | Phase E weekly `mysqldump` to Object Storage |
| No PITR on free tier | Med | Same as above + accept "lose at most 1 week of writes" RPO |
| Single-node = brief downtime on maintenance | Low | Acceptable at <100 users; upgrade to HA when traffic warrants |
| Idle-reclaim policy unverified | Low | Phase B 14-day test; mitigation = 5-min keepalive cron |
| OCI Always-Free policy change | Low | Backup plan: migrate to CockroachDB Basic (research already done) |
| Wrong region picked, must re-provision | Med | Phase A starts only after user confirms region |

## Workflow handoff

After all 6 phases complete, `phase-03-backend-auth.md` becomes unblocked for implementation with concrete schema in MySQL flavor. This plan does NOT close phase-03 — it removes phase-03's DB-decision dependency.
