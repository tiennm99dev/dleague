---
title: "Firebase Auth + self-hosted Couchbase 8.0 + Redis 8.4 (beta)"
description: "Self-hosted data plane via docker-compose: Couchbase Community 8.0 (primary store) + Redis 8.4 (cache/leaderboards). Firebase Auth external. React/Capacitor client + existing Go WS hub. Discards Aiven external-service path; everything but Firebase Auth runs on the Coolify VM. Go 1.25.5. **Beta posture: data loss acceptable on public release; early adopters tagged for future rewards. Migration-friendly via `store.Store` interface seam.**"
status: pending
priority: P1
effort: 4-5w
branch: main
tags: [firebase-auth, couchbase, couchbase-community, redis, docker-compose, self-hosted, react, capacitor, websocket, pivot, beta]
created: 2026-05-05
parent_plan: 260505-0947-dleague-pvp-game/plan.md
supersedes:
  - archive/260505-1407-firebase-platform-pivot/plan.md
  - archive/260505-1319-mysql-heatwave-integration/plan.md
  - 260505-0947-dleague-pvp-game/phase-02-game-core-pluggable.md
  - 260505-0947-dleague-pvp-game/phase-03-backend-auth.md
  - 260505-0947-dleague-pvp-game/phase-04-async-pvp.md
  - 260505-0947-dleague-pvp-game/phase-05-sync-pvp-websocket.md
  - 260505-0947-dleague-pvp-game/phase-06-polish-deploy-mobile.md
research:
  - reports/researcher-260505-1407-engine-survey-web-first.md
  - reports/researcher-260505-1407-firebase-as-backend-feasibility.md  # historical; recommendation overridden
  - reports/researcher-260505-1518-firebase-scaleup-fitness.md  # historical; recommendation overridden
  - reports/researcher-260505-1648-ebitengine-vs-phaser-fitness.md
---

# Firebase Auth + self-hosted Couchbase 8.0 + Redis 8.4 (beta)

## Goal

Self-hosted data plane on the Coolify VM via docker-compose. **Couchbase Community 8.0** as primary document store (users, puzzles, matches, attempts). **Redis 8.4** as cache + leaderboards. **Firebase Auth** is the only external dependency (free Spark plan). React/Capacitor client + existing Go WS hub unchanged. Go 1.25.5.

## Decisions locked

- Host: **OCI Always-Free Ampere A1 Flex** — 4 OCPU + 24 GB RAM + **ARM64**. Coolify-managed.
- Toolchain: **Go 1.25.5** (downgrade from 1.26). Builds target `linux/arm64`.
- Auth: Firebase (email/password + Google + anonymous), Firebase UID as primary key.
- Session: Firebase ID token verified per-WS-connect via Admin SDK; no server sessions.
- **Primary store: Couchbase Community 8.0** (self-hosted via `couchbase/server-community:8.0.0` Docker image). Bucket `dleague`, collections `users`/`puzzles`/`matches`/`attempts`.
- **Cache + leaderboards: Redis 8.4** (self-hosted via `redis:8.4-alpine`). ZSETs for leaderboards, SET+TTL for presence, generic JSON cache.
- Single VM (Coolify) hosts: Go server + Couchbase + Redis on a docker-compose internal network. Only Go server port exposed.
- Client: React + Capacitor (web first, mobile later).
- Credentials: env-injected via Coolify, no file mounts. Service-to-service auth via docker-compose internal network + strong passwords.
- Persistence: docker named volumes for Couchbase data + Redis AOF.

## Beta posture

- All sign-in screens show a **"Beta — data may reset"** banner.
- Every user is tagged `isBetaTester: true` + `betaSignupAt: <timestamp>` on first auth (early-adopter ledger for future reward).
- T&Cs / signup copy explicitly states: data is collected for product evaluation; not contractually preserved.
- VM disk failure or `docker compose down -v` = acceptable data loss; beta scope.

## Migration-friendly design (load-bearing)

This is a *short-term* stack — optimized for self-hosted simplicity now, paid managed services later. Design so the swap costs ~1 week, not a rewrite:

1. **`store.Store` Go interface** is the seam. Methods like `GetUser`, `UpsertUser`, `SubmitAttempt`, `LeaderboardTopN`. Concrete impl in `internal/store/composed/` wires together a `couchbase` client + `redis` client. Future managed-service swap: change one wiring line in `main.go`.
2. **`gocb` import only inside `internal/store/couchbase/`. `go-redis` import only inside `internal/store/redis/`.** Verifiable with grep test in CI.
3. **Stable doc shapes** — flat JSON documents in Couchbase (no deep nesting); leaderboard data in Redis ZSETs only. Migration off either backend = `cbexport` / `SCAN+HGETALL` + write-to-target.
4. **No Couchbase-specific stored procedures, no Redis Lua.** Plain N1QL queries + plain Redis commands.
5. **`memstore` impl** of the same interface ships alongside, for unit tests AND as proof that "second backend exists" (you only build a true seam by having two impls).
6. **Migration export ships as `cmd/dleague-export`** (Phase 12) — wraps `store.Export()` to JSONL of all persistent entities. Same seed for future imports to anywhere.

## Phases

| # | Phase | File | Effort | Status |
|---|-------|------|--------|--------|
| 1 | Provisioning (Firebase + docker-compose) | [phase-01-free-tier-provisioning.md](phase-01-free-tier-provisioning.md) | 0.5d | pending |
| 2 | Strip MySQL + revise config + Go 1.25.5 downgrade | [phase-02-strip-mysql-revise-config.md](phase-02-strip-mysql-revise-config.md) | 0.5d | pending |
| 3 | Couchbase 8.0 Go integration (gocb v2, primary store) | [phase-03-couchbase-go-integration.md](phase-03-couchbase-go-integration.md) | 2d | pending |
| 4 | Redis 8.4 Go integration (go-redis v9, cache + leaderboards) | [phase-04-redis-go-integration.md](phase-04-redis-go-integration.md) | 1d | pending |
| 5 | Firebase Admin SDK + token verifier + beta-tag user upsert | [phase-05-firebase-admin-token-verifier.md](phase-05-firebase-admin-token-verifier.md) | 1d | pending |
| 6 | Wire-format auth handshake | [phase-06-wire-format-auth-handshake.md](phase-06-wire-format-auth-handshake.md) | 0.5d | pending |
| 7 | React + Capacitor client scaffold + beta banner | [phase-07-react-capacitor-client-scaffold.md](phase-07-react-capacitor-client-scaffold.md) | 3d | pending |
| 8 | Pluggable game-engine layer (web) | [phase-08-pluggable-game-engine-web.md](phase-08-pluggable-game-engine-web.md) | 2d | pending |
| 9 | Async PvP via store.Store (Couchbase + Redis) | [phase-09-async-pvp-on-couchbase.md](phase-09-async-pvp-on-couchbase.md) | 2d | pending |
| 10 | Sync PvP via Go WS (auth-gated) | [phase-10-sync-pvp-via-go-ws.md](phase-10-sync-pvp-via-go-ws.md) | 1d | pending |
| 11 | Deploy on Coolify (single docker-compose) | [phase-11-deploy-coolify.md](phase-11-deploy-coolify.md) | 0.5d | pending |
| 12 | Supersession + cleanup + migration-export CLI | [phase-12-supersession-cleanup.md](phase-12-supersession-cleanup.md) | 0.5d | pending |

## Sequencing

```
1 → 2 → (3, 4, 5) parallel → 6 → 7 → 8 → (9, 10) parallel → 11 → 12
```

Phases 3+4+5 are independent backend integrations; can fan out. Phase 7 unlocks 8-10. Phase 12 is the final docs sweep + migration-export CLI.

## Top risks

- **ARM64 image availability:** all images must publish `linux/arm64` manifests. Couchbase CE historically AMD64-only; v7.6+ added ARM64 but CE-specific tag must be verified. Redis official images are multi-arch (low risk). **Phase 1 must `docker pull --platform linux/arm64` both images before continuing.**
- **VM RAM ceiling:** **24 GB on the VM — non-issue.** Stack uses ~2 GB total (Couchbase 1 GB + Redis 256 MB + Go 100 MB + Coolify 500 MB). Bump Couchbase quotas to 2 GB data + 1 GB index for headroom.
- **Couchbase 8.0 GA status:** if `couchbase/server-community:8.0.0` tag isn't published (or not published for arm64), fall back to `7.6` ARM64.
- **Redis 8.4 image availability:** if `redis:8.4-alpine` tag isn't published, fall back to `redis:8.0-alpine` or latest stable (both multi-arch).
- **Couchbase Community licensing:** CE is non-commercial-only since the 2024 license update. Public beta with rewards/monetization signal could be borderline. **Verify license terms before going public** (Phase 12 todo).
- **Single-VM SPOF:** VM dies → all data gone. Acceptable under beta data-loss policy; export CLI is the escape hatch.
- **Beta data loss is by design:** users informed via banner + signup copy; no backup work needed.

## Unresolved questions

1. ~~Coolify VM size~~ — **resolved: 24 GB RAM, 4 OCPU ARM64**.
2. **Verify ARM64 manifests** for `couchbase/server-community:8.0.0` and `redis:8.4-alpine` (Phase 1).
3. Couchbase CE license — confirm non-commercial-fit for our beta + rewards model in Phase 12.
4. Sync PvP through Coolify proxy — WS upgrade behavior at scale.
5. Phaser 4 spike outcome — adopt vs stay DOM (Phase 8).
6. Early-adopter reward mechanism — out of scope here; revisit post-beta.
7. Future managed-service migration target — Capella / Mongo Atlas / Postgres? Decide post-beta from collected usage data.
