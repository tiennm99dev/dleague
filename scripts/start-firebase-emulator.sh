#!/usr/bin/env bash
# Start the Firebase Auth emulator in the background.
# Requires: npm install -g firebase-tools
# The emulator listens on 127.0.0.1:9099 as configured in firebase.json.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${REPO_ROOT}"

echo "Starting Firebase Auth emulator (project=dleague-dev, port=9099)..."
firebase emulators:start --only auth --project dleague-dev &

echo "Firebase emulator PID=$!"
echo "Set FIREBASE_AUTH_EMULATOR_HOST=127.0.0.1:9099 before running the server or tests."
