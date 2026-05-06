---
phase: 1
title: "Provisioning (Firebase + docker-compose)"
status: in_progress
priority: P1
effort: "0.5d"
dependencies: []
---

# Phase 1: Provisioning (Firebase + docker-compose)

## Context Links

- Plan: [plan.md](plan.md)

## Overview

Two parallel tracks: (1) external Firebase project signup for Auth (only external service in the stack), (2) docker-compose definition for self-hosted Couchbase 8.0 + Redis 8.4 on the Coolify VM. Output: a working `docker-compose.yml`, a `.env.example`, and verified pingability of both data services on a local dev box.

## Key Insights

- Host VM: **OCI Ampere A1 Flex (Always-Free): 4 OCPU + 24 GB RAM + ARM64**. All images must have `linux/arm64` manifest.
- Couchbase Community 8.0 image: `couchbase/server-community:8.0.0` — **verify ARM64 manifest first** (`docker manifest inspect couchbase/server-community:8.0.0 | grep arm64`). Fallback: `7.6` (confirmed ARM64 since 2024).
- Redis 8.4 image: `redis:8.4-alpine` — Redis official images are multi-arch including ARM64 (low risk). Fallback `8.0-alpine`.
- Firebase Spark: 50K MAU/mo, all sign-in providers free; only Auth used here.
- Internal docker network means Go server addresses Couchbase via service name (`couchbase:8091`) and Redis via `redis:6379`. No public exposure for either store.
- Couchbase Community license: free for self-hosted use including commercial products. Hard caps: ≤ 5 nodes, ≤ 4 cores/node, no XDCR. (Verified 2026-05-06; see `docs/migration-readiness.md` § License watchout.)

## Requirements

- Functional: Firebase project ready with all 3 providers; docker-compose brings up Couchbase + Redis with bind-mounted volumes; both reachable from a Go probe on the host.
- Non-functional: Couchbase memory quotas set with headroom (data 2 GB + index 1 GB on the 24 GB VM); Redis maxmemory 256 MB configured.

## Architecture

```
Coolify VM
└── docker-compose
    ├── couchbase  (couchbase/server-community:8.0.0, ports 8091-8096 + 11210, internal only)
    │     └── volume: couchbase_data
    ├── redis      (redis:8.4-alpine, port 6379, internal only)
    │     └── volume: redis_data (AOF)
    └── dleague    (Go server, port 8080 → host)
                   └── env: FIREBASE_*, COUCHBASE_*, REDIS_*
```

Browser/iOS/Android → Firebase JS SDK → Firebase Auth → Go server (Bearer JWT verified via Admin SDK).

## Related Code Files

- Create:
  - `.env.example` (root)
  - `docs/deployment-guide.md` (signup + compose walkthrough)
- Modify:
  - `docker-compose.yml` — add `couchbase`, `redis` services; rewrite `dleague` service for new env vars

## Implementation Steps

1. **Firebase project**
   1. Create project at console.firebase.google.com.
   2. Auth → Sign-in providers → enable Email/Password, Google, Anonymous.
   3. Project Settings → Service Accounts → Generate new private key (JSON). Stash JSON content for env injection.
   4. Project Settings → General → copy `apiKey`, `authDomain`, `projectId` for client config.
2. **Verify ARM64 image tags** (ON THE OCI VM, not local dev if dev is x86_64)
   - `docker manifest inspect couchbase/server-community:8.0.0 | grep -i 'arm64\|aarch64'` — must show arm64. If absent, fallback to `7.6` (which has confirmed arm64 manifest).
   - `docker pull --platform linux/arm64 couchbase/server-community:8.0.0` and confirm pull succeeds.
   - `docker pull --platform linux/arm64 redis:8.4-alpine` (or fallback `8.0-alpine`).
   - Document the chosen tag in `.env.example` comment.
3. **docker-compose.yml** — define `couchbase`, `redis`, `dleague` services. Use named volumes `couchbase_data`, `redis_data`. Internal network `dleague-net`. Only `dleague:8080` exposed to host.
4. **Couchbase first-run init** — manual one-shot:
   - `docker compose up -d couchbase`
   - Wait healthy (~30s)
   - Web UI at `http://<vm-host>:8091` (temporarily expose for setup, re-internal after)
   - Init cluster: name `dleague-beta`, services Data + Query + Index, memory quotas 512 MB data + 256 MB index.
   - Create bucket `dleague` (256 MB ram quota), scope `_default`, collections `users`, `puzzles`, `matches`, `attempts`.
   - Create user `dleague_app` with role `Application Access` on bucket `dleague`.
   - Create primary index on each collection: `CREATE PRIMARY INDEX ON \`dleague\`.\`_default\`.\`users\`;` etc.
   - Re-internal the Couchbase ports.
5. **Redis config** — pass `--appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru --requirepass $REDIS_PASSWORD` via compose command.
6. **Env file**: write `.env.example` with placeholders.
7. **Verify**: `redis-cli -h localhost -a $REDIS_PASSWORD PING`; Couchbase `curl -u dleague_app:$COUCHBASE_PASSWORD http://localhost:8091/pools/default`; `curl` Firebase Identity Toolkit lookup endpoint.

## Todo List

**Code-side (this branch):**
- [x] `docker-compose.yml` defines couchbase + redis + dleague + named volumes + internal network
- [x] Redis configured with AOF + maxmemory 256mb + password (compose `command:`)
- [x] `.env.example` committed (placeholders only)
- [x] `scripts/cb-init.sh` checked in for reproducible Couchbase init
- [x] `docs/deployment-guide.md` walks through Firebase signup + compose bring-up + init + verify

**External / on-VM (out-of-repo work, blocks "phase complete" stamp):**
- [ ] Firebase project created, providers enabled, service-account JSON downloaded
- [ ] **ARM64 manifests verified** for both Couchbase + Redis tags (or ARM64 fallbacks chosen and `image:` line updated in compose)
- [ ] Couchbase cluster initialized via `scripts/cb-init.sh` (or manual Option B in deployment guide): bucket + collections + app user + primary indexes
- [ ] All three services pinged successfully from VM host
- [ ] Couchbase Web UI port (8091) re-internalized (not exposed to host) after setup — already off by default in compose

## Success Criteria

- [ ] `docker compose up -d` brings everything up green; healthchecks pass within 60s
- [ ] `.env.example` contains: `FIREBASE_CREDENTIALS_JSON`, `FIREBASE_PROJECT_ID`, `COUCHBASE_CONN_STRING` (e.g. `couchbase://couchbase`), `COUCHBASE_USERNAME`, `COUCHBASE_PASSWORD`, `COUCHBASE_BUCKET`, `REDIS_ADDR`, `REDIS_PASSWORD`
- [ ] Internal docker network: `dleague` service can `nc -zv couchbase 8091` and `nc -zv redis 6379`
- [ ] No data ports exposed to public internet (only `dleague:8080`)

## Risk Assessment

- **Couchbase 8.0 not GA / unstable** — fallback to `7.6`. Document version chosen in `.env.example` comment.
- **Couchbase memory quotas misconfigured** — quotas too high → swap; too low → poor query perf. With 24 GB VM, set generous quotas (data 2 GB, index 1 GB) and revisit if needed.
- **Redis password leaked** — internal-only port mitigates; rotate via env-only.
- **First-run Couchbase init manual** — boring but error-prone. Mitigation: automation script `scripts/cb-init.sh` checked in alongside compose so re-init is reproducible.

## Security Considerations

- Couchbase Web UI port (8091) only exposed during initial setup; re-internal afterward.
- Redis bound to internal network + password.
- Firebase service-account JSON only as env var, never committed.
- Couchbase app user is bucket-scoped (`Application Access`), not admin.

## Next Steps

Phase 2: strip MySQL code, downgrade Go to 1.25.5, swap config schema.

## Unresolved Questions

1. ~~VM RAM size~~ — resolved: 24 GB.
2. Couchbase 8.0 image GA — verify; otherwise pick stable fallback.
3. ~~Couchbase CE license fit for beta-with-rewards — Phase 12 review.~~ — resolved 2026-05-06; CE License Agreement permits commercial use; hard caps 5 nodes / 4 cores/node / no XDCR.
4. Coolify volume mount strategy — named volumes vs host bind mounts; pick during compose write.
