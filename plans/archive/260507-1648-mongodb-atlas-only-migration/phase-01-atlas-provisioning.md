---
phase: 1
title: "Atlas provisioning + env wiring"
status: completed
priority: P1
effort: "0.5d"
dependencies: []
---

# Phase 1: Atlas provisioning + env wiring

## Context links

- Research: [`plans/reports/researcher-260507-1648-mongodb-atlas-tiers-and-limits.md`](../reports/researcher-260507-1648-mongodb-atlas-tiers-and-limits.md)
- Current env shape: [`docker-compose.yml`](../../docker-compose.yml), [`.env.example`](../../.env.example)

## Overview

Stand up a free-tier MongoDB Atlas M0 cluster in AWS Singapore. Configure SCRAM-SHA-256 user, IP allowlist, and SRV connection string. Add `MONGODB_URI` to `.env.example` + Coolify config. Smoke-test connectivity from a local Go program before any code in `internal/store/mongodb/` is written.

## Requirements

**Functional:**
- One M0 cluster, one DB user, one IP allowlist entry, one connection string.
- Database name: `dleague`. Collections created lazily on first write.
- TLS enforced (default on Atlas).

**Non-functional:**
- Region must minimize latency from OCI Singapore. Default: `ap-southeast-1` (AWS Singapore).
- Connection string injected as env var; never committed.

## Architecture

```
Atlas project: dleague-beta
  └── Cluster: dleague-m0 (M0, ap-southeast-1, 3-node replica set)
       └── DB: dleague
            └── (collections created at first write)
  └── DB user: dleague-app (SCRAM-SHA-256, role: readWrite on dleague)
  └── Network: IP allowlist
       └── (TBD): static Coolify egress IP, or 0.0.0.0/0 during beta
```

## Related code files

- **Modify:** `.env.example` — add `MONGODB_URI=mongodb+srv://...` placeholder.
- **Modify:** `docker-compose.yml` — Coolify will inject `MONGODB_URI`; in local dev keep as-is until Phase 5 wires it.
- **Create:** `scripts/atlas-smoke.go` (or one-off cmd) — connect, run `db.runCommand({ping: 1})`, exit.
- **Create:** `docs/atlas-setup.md` — step-by-step Atlas provisioning runbook (one page).

## Implementation steps

1. **Sign up / log in** to Atlas (https://cloud.mongodb.com). Use the project owner's account.
2. **Create project** `dleague-beta`.
3. **Create M0 cluster** `dleague-m0` in AWS `ap-southeast-1`. Provider: AWS. Tier: M0 Sandbox (free).
4. **Create DB user** `dleague-app` with role `readWrite` on database `dleague`. Use a random 32-char password from `openssl rand -base64 24`. Save in 1Password (or equivalent).
5. **Configure IP allowlist.** Decision point:
   - **Option A (chosen during beta):** `0.0.0.0/0` — simpler, auth still SCRAM-secured. Document this as a beta-only choice.
   - **Option B:** pin Coolify static egress IP. Requires confirming OCI VM egress is stable. Defer to Phase 7 if option A is fine.
6. **Copy SRV connection string** from "Connect → Drivers → Go". Format: `mongodb+srv://dleague-app:<password>@dleague-m0.xxxxx.mongodb.net/?retryWrites=true&w=majority&appName=dleague-server`.
7. **Add `MONGODB_URI` to `.env.example`** (placeholder; not the real URI).
8. **Inject real `MONGODB_URI` into Coolify** as a secret env var.
9. **Smoke test from dev machine:** write `scripts/atlas-smoke.go` that imports `go.mongodb.org/mongo-driver/v2/mongo`, connects, runs `client.Database("dleague").RunCommand(ctx, bson.D{{"ping", 1}})`, prints OK, exits.
10. **Document the runbook** at `docs/atlas-setup.md` — exact steps, what to copy from Atlas UI, where to put `MONGODB_URI`, how to rotate the DB user password.

## Todo list

- [ ] Atlas account + project + cluster created
- [ ] DB user + password stored securely
- [ ] IP allowlist set (decision logged)
- [ ] `MONGODB_URI` in `.env.example` (placeholder)
- [ ] `MONGODB_URI` set in Coolify (real value)
- [ ] `scripts/atlas-smoke.go` written + executed successfully (`Pong: 1`)
- [ ] `docs/atlas-setup.md` written (under 80 lines)

## Success criteria

- Running `go run ./scripts/atlas-smoke.go` from a dev machine prints `ping ok` and exits 0.
- A second person can follow `docs/atlas-setup.md` to recreate the cluster from scratch.
- No SCRAM credentials live in any committed file.

## Risk assessment

- **Atlas signup may require credit card even on M0.** Mitigation: it does not (verified 2026-05). If policy has changed, project owner provides card; M0 has $0 charge unless tier upgraded.
- **`mongodb+srv://` requires DNS SRV resolution.** Most container runtimes handle this fine; verified in Phase 2 integration test.
- **Wrong region picked.** Mitigation: latency-test from OCI Singapore in Phase 4; if >50ms, recreate cluster in correct region (M0 cluster recreation is fast, no data lost since none yet).

## Security considerations

- DB user password is not committed; lives only in Coolify secrets + 1Password.
- `0.0.0.0/0` allowlist during beta is acceptable because SCRAM auth is required; document the trade-off in `docs/atlas-setup.md` and revisit in Phase 7.
- Connection string contains the password — never log it; mask `MONGODB_URI` in any startup log output.

## Next steps

Unblocks Phase 2 (need a real cluster to run the integration test against).
