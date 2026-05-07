# Deployment Guide

Dev/beta deploy walkthrough for the dleague stack:
**Firebase Auth + MongoDB Atlas + Go server**, all behind one
`docker-compose.yml`. Authoritative plan:
[`plans/260507-1648-mongodb-atlas-only-migration/plan.md`](../plans/260507-1648-mongodb-atlas-only-migration/plan.md).

> **Beta posture:** data loss is acceptable on this stack; backups are not
> in scope. `mongodump --uri "$MONGODB_URI"` is the escape hatch for
> ad-hoc snapshots.

## 1. Firebase project (one-time)

1. Go to <https://console.firebase.google.com> → **Add project** → name it (e.g. `dleague-beta`).
2. **Build → Authentication → Get started → Sign-in method**. Enable:
   - Email/Password
   - Google
   - Anonymous
3. **Project settings → Service accounts → Generate new private key**.
   Download the JSON. Stringify for env injection:

   ```bash
   jq -c . < ~/Downloads/dleague-beta-firebase-adminsdk-*.json
   ```

   Paste the single-line output as `FIREBASE_CREDENTIALS_JSON` in your `.env`.
4. **Project settings → General**. Copy `Project ID` into
   `FIREBASE_PROJECT_ID`. The web `apiKey` / `authDomain` / `appId` are for
   the Svelte client config, not the server.

## 2. MongoDB Atlas project (one-time)

Follow [`docs/atlas-setup.md`](./atlas-setup.md). Tldr:

1. Create project `dleague-beta`, M0 cluster `dleague-m0` in AWS Singapore
   (`ap-southeast-1`).
2. Create DB user `dleague-app` with `readWrite` on `dleague`.
3. Network access: `0.0.0.0/0` during beta (SCRAM still required); tighten
   before non-beta launch.
4. Copy SRV connection string into `MONGODB_URI`.
5. Smoke-test:
   ```bash
   MONGODB_URI='mongodb+srv://...' make atlas-smoke
   ```
   Expected output: `ping ok (db=dleague)`.

## 3. Bring up the stack

```bash
cp .env.example .env
# Fill in FIREBASE_CREDENTIALS_JSON, FIREBASE_PROJECT_ID, MONGODB_URI.

docker compose up -d
docker compose ps  # dleague server should be (healthy) within ~30s
```

The compose file defines a single service: the Go server. The data plane
lives in Atlas, so there are no local DB containers to manage.

## 4. Verify

```bash
# Health
curl -fsS http://localhost:8080/health
# → ok

# Atlas reachability (uses the same MONGODB_URI as the server)
make atlas-smoke

# Firebase reachability (no auth required for this endpoint shape).
curl -fsS "https://identitytoolkit.googleapis.com/v1/projects/$FIREBASE_PROJECT_ID:lookup" \
  -H "Content-Type: application/json" -d '{}' || true
```

## 5. Image build (multi-arch via buildx)

`server/Dockerfile` is a 3-stage build:

1. `golang:1.25.5-alpine` → builds `cmd/api` (static, trimpath).
2. `node:22-alpine` → `npm ci && npm run build-nolog` for the SvelteKit
   client. With `adapter-static`, output lands in `client/web/build/`.
3. `alpine:3.20` runtime — binary on `PATH`, web build at `/app/web`,
   `HEALTHCHECK` against `/health`.

Default env baked in: `DLEAGUE_ADDR=:8080`, `DLEAGUE_WEB=/app/web`.

Build targets:

```bash
# Multi-arch image, no local load (buildx restriction).
make image                 IMAGE=ghcr.io/you/dleague IMAGE_TAG=v0.1

# Single-arch local image (host arch), loaded into docker daemon.
make image-load            IMAGE=dleague-server IMAGE_TAG=dev

# Multi-arch image, push to registry.
make image-push            IMAGE=ghcr.io/you/dleague IMAGE_TAG=v0.1
```

Defaults: `IMAGE=dleague-server`, `IMAGE_TAG=dev`,
`IMAGE_PLATFORMS=linux/amd64,linux/arm64`. Coolify pulls the ARM64
manifest on the OCI Ampere VM.

## 6. Coolify deploy

Coolify-injected env vars (set on the dleague service):

| Variable | Value |
|----------|-------|
| `DLEAGUE_WS_ORIGINS` | `https://your.domain` |
| `FIREBASE_CREDENTIALS_JSON` | Service-account JSON (single-line) |
| `FIREBASE_PROJECT_ID` | Firebase project ID |
| `MONGODB_URI` | Atlas SRV string (mark as **secret**) |
| `MONGODB_DB` | `dleague` (or override) |

Optional overrides (have safe defaults):
- `DLEAGUE_ADDR` (`:8080`), `DLEAGUE_WEB` (`/app/web`).

## 7. Security checklist

- [ ] Atlas IP allowlist documented (`0.0.0.0/0` is beta-only — tighten
      before non-beta launch via static-IP NAT or PrivateLink M10+)
- [ ] `MONGODB_URI` marked secret in Coolify; never logged
- [ ] `.env` not committed (gitignored; double-check before pushing)
- [ ] `FIREBASE_CREDENTIALS_JSON` only as env var, never on disk in repo
- [ ] DB user `dleague-app` has `readWrite` on `dleague` only — not
      `dbAdmin`/`atlasAdmin`
- [ ] Atlas alerts enabled: connection-count breach, slow-query log

## 8. CI

The GitHub Actions workflow at `.github/workflows/ci.yml.disabled` is
disabled. Re-enabling is tracked in
[`phase-07-cleanup-and-docs.md`](../plans/260507-1648-mongodb-atlas-only-migration/phase-07-cleanup-and-docs.md);
the rewrite drops the WASM/MySQL stack assumptions and runs `go test ./...`,
`make grep-isolation`, `make web-build`.
