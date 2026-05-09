#!/usr/bin/env bash
# promote-admin.sh — one-shot: set admin custom claim on a Firebase user.
#
# Usage:
#   FIREBASE_PROJECT_ID=dleague-prod \
#   GOOGLE_APPLICATION_CREDENTIALS=/path/to/serviceAccount.json \
#   bash scripts/promote-admin.sh <firebase-uid>
#
# Uses cmd/admin CLI under the hood. Requires Go toolchain on PATH.

set -euo pipefail

UID_ARG="${1:-}"
if [[ -z "$UID_ARG" ]]; then
  echo "Usage: $0 <firebase-uid>" >&2
  exit 1
fi

if [[ -z "${FIREBASE_PROJECT_ID:-}" ]]; then
  echo "ERROR: FIREBASE_PROJECT_ID must be set." >&2
  exit 1
fi

cd "$(dirname "$0")/.."
go run ./server/cmd/admin --action=promote-admin --uid="$UID_ARG"
echo "Admin claim set for uid=$UID_ARG"
