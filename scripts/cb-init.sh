#!/usr/bin/env bash
# Idempotent first-run init for the Couchbase service in docker-compose.
# Safe to re-run: each step skips on "already exists".
#
# Prerequisites:
#   - `docker compose up -d couchbase` already ran and the container is healthy.
#   - The following env vars are set (sourced from .env):
#       COUCHBASE_USERNAME, COUCHBASE_PASSWORD, COUCHBASE_BUCKET
#   - The cluster admin password defaults to $COUCHBASE_PASSWORD here, but for
#     stricter setups, set CB_ADMIN_USER + CB_ADMIN_PASSWORD before running.
#
# Usage:
#   ./scripts/cb-init.sh

set -euo pipefail

CONTAINER="${COUCHBASE_CONTAINER:-dleague-couchbase}"
CB_HOST="${CB_HOST:-127.0.0.1}"
CB_PORT="${CB_PORT:-8091}"
CB_ADMIN_USER="${CB_ADMIN_USER:-Administrator}"
CB_ADMIN_PASSWORD="${CB_ADMIN_PASSWORD:-${COUCHBASE_PASSWORD:?missing COUCHBASE_PASSWORD}}"

APP_USER="${COUCHBASE_USERNAME:?missing COUCHBASE_USERNAME}"
APP_PASSWORD="${COUCHBASE_PASSWORD:?missing COUCHBASE_PASSWORD}"
BUCKET="${COUCHBASE_BUCKET:?missing COUCHBASE_BUCKET}"

# Memory quotas — generous on a 24 GB ARM VM; tune later if needed.
DATA_MB="${CB_DATA_QUOTA_MB:-2048}"
INDEX_MB="${CB_INDEX_QUOTA_MB:-1024}"
BUCKET_MB="${CB_BUCKET_QUOTA_MB:-512}"

cb() { docker exec "$CONTAINER" couchbase-cli "$@"; }
cbq() { docker exec "$CONTAINER" cbq -u "$CB_ADMIN_USER" -p "$CB_ADMIN_PASSWORD" -e "http://localhost:$CB_PORT" -script "$1"; }

echo "==> Waiting for Couchbase to respond on $CB_HOST:$CB_PORT…"
until docker exec "$CONTAINER" curl -fsS "http://localhost:$CB_PORT/pools" >/dev/null 2>&1; do
  sleep 2
done

echo "==> Initializing cluster (skipped if already initialized)…"
cb cluster-init \
  --cluster "http://localhost:$CB_PORT" \
  --cluster-name "dleague-beta" \
  --cluster-username "$CB_ADMIN_USER" \
  --cluster-password "$CB_ADMIN_PASSWORD" \
  --cluster-ramsize "$DATA_MB" \
  --cluster-index-ramsize "$INDEX_MB" \
  --services data,index,query \
  --index-storage-setting default \
  || echo "  (cluster already initialized)"

echo "==> Creating bucket '$BUCKET' (skipped if exists)…"
cb bucket-create \
  --cluster "http://localhost:$CB_PORT" \
  --username "$CB_ADMIN_USER" \
  --password "$CB_ADMIN_PASSWORD" \
  --bucket "$BUCKET" \
  --bucket-type couchbase \
  --bucket-ramsize "$BUCKET_MB" \
  --bucket-replica 0 \
  --enable-flush 0 \
  || echo "  (bucket already exists)"

echo "==> Creating collections (users, puzzles, matches, attempts)…"
for col in users puzzles matches attempts; do
  cb collection-manage \
    --cluster "http://localhost:$CB_PORT" \
    --username "$CB_ADMIN_USER" \
    --password "$CB_ADMIN_PASSWORD" \
    --bucket "$BUCKET" \
    --create-collection "_default.$col" \
    || echo "  (collection $col already exists)"
done

echo "==> Creating app user '$APP_USER' with bucket-scoped access…"
cb user-manage \
  --cluster "http://localhost:$CB_PORT" \
  --username "$CB_ADMIN_USER" \
  --password "$CB_ADMIN_PASSWORD" \
  --set \
  --rbac-username "$APP_USER" \
  --rbac-password "$APP_PASSWORD" \
  --roles "bucket_full_access[$BUCKET]" \
  --auth-domain local \
  || echo "  (user already exists)"

echo "==> Creating primary indexes on each collection…"
for col in users puzzles matches attempts; do
  cbq "CREATE PRIMARY INDEX IF NOT EXISTS ON \`$BUCKET\`.\`_default\`.\`$col\`;"
done

echo "==> Done. Verify: curl -u \"$APP_USER:$APP_PASSWORD\" http://$CB_HOST:$CB_PORT/pools/default"
