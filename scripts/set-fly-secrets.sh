#!/usr/bin/env bash
# set-fly-secrets.sh — documents the fly secrets set commands for dleague production.
#
# DO NOT EXECUTE THIS SCRIPT DIRECTLY FROM CI OR MAKE.
# A human operator runs these commands manually after creating the Fly app.
#
# Prerequisites:
#   1. fly CLI installed and authenticated (fly auth login)
#   2. Fly app created: fly apps create dleague
#   3. Atlas M0 cluster provisioned; prod user created with readWrite@dleague role.
#   4. Firebase project created; service account JSON downloaded locally.
#
# Usage (run each line individually, substituting real values):
#
# MONGO_URI  ─────────────────────────────────────────────────────────────────
# fly secrets set MONGO_URI="mongodb+srv://dleague_prod:<PASSWORD>@<CLUSTER>.mongodb.net/dleague?retryWrites=true&w=majority"
#
# FIREBASE_PROJECT_ID  ───────────────────────────────────────────────────────
# fly secrets set FIREBASE_PROJECT_ID="dleague-prod"
#
# FIREBASE_SERVICE_ACCOUNT_B64  ──────────────────────────────────────────────
# Base64-encode the service account JSON (single line, no newlines):
#   fly secrets set FIREBASE_SERVICE_ACCOUNT_B64="$(base64 -w0 /path/to/serviceAccount-dleague-prod.json)"
#
# DLEAGUE_WS_ORIGINS  ────────────────────────────────────────────────────────
# Comma-separated list of origins allowed to open WebSocket connections.
# Must be non-empty in production (boot-time assertion will refuse to start otherwise).
#   fly secrets set DLEAGUE_WS_ORIGINS="https://dleague.fly.dev,https://dleague.gg"
#
# DLEAGUE_TRUSTED_PROXIES  ───────────────────────────────────────────────────
# Fly.io proxy CIDR ranges for X-Forwarded-For trust.
# See https://fly.io/docs/networking/static-egress-ip/ for current ranges.
#   fly secrets set DLEAGUE_TRUSTED_PROXIES="<fly_proxy_cidr_1>,<fly_proxy_cidr_2>"
#
# DLEAGUE_ENV  ───────────────────────────────────────────────────────────────
# Already set in fly.toml [env]; only override if you need a non-default value.
#   fly secrets set DLEAGUE_ENV="production"
#
# After setting secrets, deploy:
#   fly deploy --remote-only
#
# Verify:
#   curl -s https://dleague.fly.dev/health | jq .

echo "This script is documentation-only. Do not execute it directly."
echo "Copy and run the fly secrets set commands above manually."
exit 1
