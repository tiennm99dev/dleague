# Deployment Guide

**Status:** skeleton — final runbook produced by Phase 10.

## Targets
| Component        | Target                              |
|------------------|-------------------------------------|
| Go server + static | Fly.io (single app)               |
| Database         | MongoDB Atlas M0 (free tier)        |
| Auth             | Firebase Auth (Google-managed)      |
| Local dev        | Docker Compose (`mongo:7` + emulator) |

## Local development
TODO Phase 06.

```bash
# 1. Start mongo + mongo-express
docker compose up -d

# 2. Start Firebase Auth emulator (Phase 05)
firebase emulators:start --only auth

# 3. Run Go server (debug)
make dev-debug

# 4. Run SvelteKit dev server (proxies /ws to :8080)
npm --prefix web run dev
```

## Environment variables
TODO Phase 05/04/10. Expected set:
- `DLEAGUE_BIND_ADDR` — default `:8080`
- `DLEAGUE_WS_ORIGINS` — comma-separated allowlist (Phase 02)
- `DLEAGUE_MAX_CONNS` — default `1000` (Phase 02)
- `DLEAGUE_TRUSTED_PROXIES` — proxy IP allowlist for `RealIP` (Phase 02)
- `MONGO_URI` — Atlas SRV string (Phase 04)
- `FIREBASE_PROJECT_ID` — for Admin SDK (Phase 05)
- `GOOGLE_APPLICATION_CREDENTIALS` — path to SA JSON (Phase 05; Fly.io: secret)
- `FIREBASE_AUTH_EMULATOR_HOST` — local dev only

## Production deploy (Fly.io)
TODO Phase 10.

```bash
fly secrets set MONGO_URI=...
fly secrets set FIREBASE_SA_JSON="$(cat sa.json)"  # mounted via process flag
fly deploy
```

## Atlas
- Free M0 cluster (512 MB, 500 conns, 100 ops/sec, replica set).
- IP allowlist: `0.0.0.0/0` until VPC peering on M10 (decision recorded in active plan §Unresolved Q1).
- Backups: M0 has none. Manual `mongodump` weekly until v2.

## Firebase
- Sign-in providers enabled: Email/Password, Google, Anonymous.
- Service account JSON downloaded once, stored only as Fly secret (never committed).
- Quota: 50 K MAU free. Alert at 40 K MAU.

## CI/CD
TODO Phase 10. GitHub Actions pipeline:
- All third-party actions pinned to SHA.
- Build → test (race) → lint → buf → npm build → fly deploy on tag.

## Rollback
TODO Phase 10. `fly releases` and `fly deploy --image <prev>`.
