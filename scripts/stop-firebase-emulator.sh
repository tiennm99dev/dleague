#!/usr/bin/env bash
# Stop any running Firebase emulator process.
# Sends SIGTERM to all firebase/java processes started by start-firebase-emulator.sh.
set -euo pipefail

echo "Stopping Firebase Auth emulator..."

# firebase emulators:start spawns a Java process; kill by matching the emulator port.
if pgrep -f "firebase emulators" > /dev/null 2>&1; then
  pkill -f "firebase emulators" && echo "Stopped firebase process."
else
  echo "No firebase emulator process found."
fi

# Also kill the underlying Java emulator if still running.
if pgrep -f "cloud-firestore-emulator\|firebase-auth-emulator" > /dev/null 2>&1; then
  pkill -f "cloud-firestore-emulator\|firebase-auth-emulator" && echo "Stopped Java emulator process."
fi
