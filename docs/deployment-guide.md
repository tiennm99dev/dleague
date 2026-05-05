# Deployment Guide

This is the dev/beta deploy walkthrough for the dleague stack:
**Firebase Auth + self-hosted Couchbase 8.0 + Redis 8.4 + Go server**, all
behind one `docker-compose.yml`. Authoritative plan:
[`plans/260505-1604-firebase-couchbase-redis-pivot/plan.md`](../plans/260505-1604-firebase-couchbase-redis-pivot/plan.md).

> **Beta posture:** data loss is acceptable on this stack; backups are not in
> scope. The `cmd/dleague-export` CLI (Phase 12) is the migration escape hatch.

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
   `FIREBASE_PROJECT_ID`. The web `apiKey` / `authDomain` / `appId` are for the
   Svelte client config (Phase 7), not the server.

## 2. Verify ARM64 image manifests (on the OCI VM)

The OCI Always-Free Ampere A1 Flex VM is **ARM64**. All images must publish a
`linux/arm64` manifest.

```bash
docker manifest inspect couchbase/server-community:8.0.0 | grep -i 'arm64\|aarch64'
docker manifest inspect redis:8.4-alpine | grep -i 'arm64\|aarch64'
```

If `couchbase/server-community:8.0.0` has **no** ARM64 manifest, fall back to
`couchbase/server-community:7.6.x` (confirmed ARM64). Update the `image:` line
in `docker-compose.yml`. Same fallback rule for Redis: `redis:8.0-alpine`.

Confirm pulls succeed:

```bash
docker pull --platform linux/arm64 couchbase/server-community:8.0.0
docker pull --platform linux/arm64 redis:8.4-alpine
```

## 3. Bring up the stack

```bash
cp .env.example .env
# Fill in FIREBASE_*, COUCHBASE_PASSWORD, REDIS_PASSWORD with strong values.

# Start data services first.
docker compose up -d couchbase redis
docker compose ps  # both should be (healthy) within ~60s
```

## 4. First-run Couchbase init

The Couchbase Web UI lives on port 8091. The compose file leaves that port
**unexposed by default**. For first-run init you have two options:

### Option A — automated (recommended)

```bash
# Sources .env into the script.
set -a && source .env && set +a
./scripts/cb-init.sh
```

This is idempotent: cluster init, bucket `dleague`, collections
(`users` / `puzzles` / `matches` / `attempts`), the bucket-scoped app user,
and primary indexes — each step skips on "already exists".

### Option B — manual via Web UI

Temporarily uncomment the `8091:8091` port mapping in `docker-compose.yml`,
`docker compose up -d couchbase`, then point a browser at
`http://<vm-host>:8091`:

1. **Setup new cluster**. Name `dleague-beta`. Services: Data + Query + Index.
   Memory quotas: Data **2048 MB**, Index **1024 MB**.
2. **Bucket → Add bucket**. Name `dleague`, RAM `512 MB`.
3. **Bucket → Scopes & Collections** under `_default`: add `users`,
   `puzzles`, `matches`, `attempts`.
4. **Security → Users → Add user**. Username `dleague_app`,
   role **Application Access** on bucket `dleague`.
5. **Query → Workbench**: run
   ```sql
   CREATE PRIMARY INDEX ON `dleague`.`_default`.`users`;
   CREATE PRIMARY INDEX ON `dleague`.`_default`.`puzzles`;
   CREATE PRIMARY INDEX ON `dleague`.`_default`.`matches`;
   CREATE PRIMARY INDEX ON `dleague`.`_default`.`attempts`;
   ```
6. **Re-comment the `8091:8091` line** and `docker compose up -d couchbase`.

## 5. Verify

```bash
# Redis
docker compose exec redis redis-cli -a "$REDIS_PASSWORD" PING
# → PONG

# Couchbase (from inside the dleague net)
docker compose exec couchbase \
  curl -u "$COUCHBASE_USERNAME:$COUCHBASE_PASSWORD" \
  http://localhost:8091/pools/default | head

# Firebase reachability (no auth required for this endpoint shape).
curl -fsS "https://identitytoolkit.googleapis.com/v1/projects/$FIREBASE_PROJECT_ID:lookup" \
  -H "Content-Type: application/json" -d '{}' || true
```

The Go server gets started by Coolify in production, or with
`docker compose up -d dleague` for local end-to-end. Health check at
`http://localhost:8080/health`.

## 6. Security checklist

- [ ] `8091` (Couchbase Web UI) **not exposed to host** in steady state
- [ ] `6379` (Redis) **not exposed to host**
- [ ] Only `dleague:8080` published
- [ ] `.env` not committed (it is gitignored; double-check before pushing)
- [ ] `FIREBASE_CREDENTIALS_JSON` only as env var, never on disk in repo
- [ ] Couchbase app user has bucket-scoped role, not cluster admin

## 7. CI

The GitHub Actions workflow at `.github/workflows/ci.yml` is **disabled
during the pivot** — the file is renamed `ci.yml.disabled` because it
targeted the old Go 1.26 + WASM stack. Re-enabling is tracked in
[`phase-12-supersession-cleanup.md`](../plans/260505-1604-firebase-couchbase-redis-pivot/phase-12-supersession-cleanup.md).
