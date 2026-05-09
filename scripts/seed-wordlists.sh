#!/usr/bin/env bash
# seed-wordlists.sh — one-shot: upload embedded wordlists into prod Mongo.
#
# Usage:
#   MONGO_URI="mongodb+srv://dleague_prod:<PASSWORD>@..." bash scripts/seed-wordlists.sh
#
# The Go seed-wordlists command reads MONGO_URI from the environment and
# upserts the embedded answers.txt + dictionary.txt into the `wordlists`
# collection. Safe to re-run (idempotent upsert).
#
# Requires: Go toolchain available on PATH, repo root as working directory.

set -euo pipefail

if [[ -z "${MONGO_URI:-}" ]]; then
  echo "ERROR: MONGO_URI must be set." >&2
  exit 1
fi

echo "Seeding wordlists into Mongo..."
cd "$(dirname "$0")/.."
go run ./server/cmd/seed-wordlists
echo "Done."
