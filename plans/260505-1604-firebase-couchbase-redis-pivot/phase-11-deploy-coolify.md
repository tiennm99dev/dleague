---
phase: 11
title: "Deploy on Coolify (single docker-compose)"
status: pending
priority: P1
effort: "0.5d"
dependencies: [9, 10]
---

# Phase 11: Deploy on Coolify

## Context Links

- Plan: [plan.md](plan.md)
- Compose definition: produced in Phase 1, finalized here
- Existing repo: `docker-compose.yml`, `Makefile`

## Overview

Single docker-compose stack on the Coolify VM. Three services on an internal network: `couchbase`, `redis`, `dleague` (Go server, also serves the Svelte+Phaser web build). Only `dleague:8080` exposed via Coolify reverse proxy. Firebase Auth is the only external dependency.

## Key Insights

- All data services run in-VM. No external SaaS bills, no public exposure of data ports.
- Coolify's native docker-compose support handles the multi-service deploy from the same repo.
- Web client built in CI (Vite), copied into the `dleague` container final stage; Go binary serves `web/` on `/`.
- Persistence via named volumes. Backup = `docker run --rm -v couchbase_data:/data -v $PWD:/backup alpine tar czf /backup/cb.tar.gz /data` (manual, beta-only).

## Requirements

- Functional: `git push` to main → Coolify rebuilds → all 3 services up with healthchecks passing.
- Non-functional: full stack rebuild + deploy <8 min; healthcheck on `dleague` returns 200 within 60s of container start (Couchbase warm-up included).

## Architecture

```
Coolify VM
└── docker-compose
    ├── couchbase  (couchbase/server-community:8.0.0)
    │     ports: internal 8091-8096, 11210
    │     volume: couchbase_data
    │     healthcheck: curl -fsS http://localhost:8091/pools/default
    ├── redis      (redis:8.4-alpine)
    │     ports: internal 6379
    │     volume: redis_data (AOF)
    │     healthcheck: redis-cli -a $REDIS_PASSWORD PING
    └── dleague    (built from Dockerfile)
          ports: 8080 → host
          depends_on: couchbase (healthy), redis (healthy)
          healthcheck: wget -qO- http://localhost:8080/health
```

Coolify reverse proxy: HTTPS → `dleague:8080`.

Env vars (Coolify-managed):
```
DLEAGUE_ADDR=:8080
DLEAGUE_WEB=/app/web
DLEAGUE_WS_ORIGINS=https://dleague.example.com
FIREBASE_PROJECT_ID=...
FIREBASE_CREDENTIALS_JSON={"type":"service_account",...}
COUCHBASE_CONN_STRING=couchbase://couchbase
COUCHBASE_USERNAME=dleague_app
COUCHBASE_PASSWORD=***
COUCHBASE_BUCKET=dleague
REDIS_ADDR=redis:6379
REDIS_PASSWORD=***
```

## Related Code Files

- Modify:
  - `docker-compose.yml` — three services, volumes, internal network, healthchecks
  - `Makefile` — `make image`, `make up`, `make logs`, `make down`
- Create:
  - `Dockerfile` — multi-stage: Go binary + Vite build + minimal alpine runtime
  - `scripts/cb-init.sh` — idempotent Couchbase first-run init (bucket + user + indexes)
  - `.github/workflows/deploy.yml` — push image on main (or rely on Coolify webhook)
  - `docs/deployment-guide.md` — Coolify setup walkthrough + env var schema

## Implementation Steps

1. `Dockerfile`:
   - Stage 1: `golang:1.25.5-alpine` → `go build ./server/cmd/api -o /out/dleague-api` (also build `cmd/dleague-export` from Phase 12)
   - Stage 2: `node:22-alpine` → `cd client/web && npm ci && npm run build`
   - Stage 3: `alpine:latest` → copy binaries + `dist/` into `/app/web/` → ENTRYPOINT `/app/dleague-api`
2. Finalize `docker-compose.yml` from Phase 1 — add Coolify-friendly labels if needed.
3. Coolify project setup (manual, doc'd):
   - Connect repo + main branch
   - Set 11 env vars
   - Domain + Let's Encrypt
   - Deploy
4. Run `scripts/cb-init.sh` once after first deploy to ensure bucket + indexes exist (idempotent).
5. Verify: production URL → sign in → play one puzzle → leaderboard updates → reconnect.

## Todo List

- [ ] Dockerfile multi-stage with Go 1.25.5 base
- [ ] docker-compose.yml: couchbase + redis + dleague services + healthchecks + depends_on
- [ ] cb-init.sh idempotent first-run script
- [ ] Coolify project configured + 11 env vars set
- [ ] Domain + TLS provisioned via Coolify
- [ ] Smoke test on production URL: full sign-in → puzzle → leaderboard
- [ ] Deployment guide doc'd
- [ ] Volumes confirmed persistent across `docker compose restart`

## Success Criteria

- [ ] `git push origin main` → Coolify auto-deploys
- [ ] All three services healthy within 90s of deploy start
- [ ] /health: 200 ok with both Couchbase + Redis reachable
- [ ] End-to-end smoke: sign in + complete puzzle + see leaderboard
- [ ] Volumes survive container recreate (data persists)

## Risk Assessment

- **VM RAM is non-issue** — 24 GB available. Stack uses ~2 GB total. Bump Couchbase quotas to 2 GB data + 1 GB index for performance headroom.
- **ARM64 image manifest gaps** — see Phase 1; verified up-front.
- **Cross-arch local dev** — if developer is on x86_64 (Intel/AMD) Mac/Linux, `docker buildx` cross-builds arm64 images via QEMU (slow). Native arm64 dev (Apple Silicon, ARM Linux) builds at native speed. Document in deployment guide.
- **Couchbase healthcheck race** — Couchbase takes 30-45s to be query-ready. Mitigation: `dleague` `depends_on: couchbase: condition: service_healthy` plus generous startup grace.
- **Build cache misses bloat CI time** — split Dockerfile layers carefully (deps first, code second).
- **License compliance for Couchbase CE** — confirmed in Phase 12 before public-beta announcement.

## Security Considerations

- Service-account JSON in Coolify env: marked secret, not in build logs.
- Reverse proxy enforces TLS (Coolify); Go server only listens on plain HTTP locally.
- WS origin allowlist (`DLEAGUE_WS_ORIGINS`) set to production domain — block CSRF-via-WS.
- Couchbase Web UI port 8091 NOT exposed to host outside initial setup window.

## Next Steps

Phase 12: docs sweep + supersede prior plans + ship migration-export CLI + license review.

## Unresolved Questions

- Coolify volume backup strategy — manual `tar` for beta, automated for post-beta? Defer.
- Single-region only — out of scope for testing.
