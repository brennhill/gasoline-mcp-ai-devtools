#!/bin/bash
# check-sync-invariants.sh
# Checks for known regression patterns that caused production issues.
# Run in CI to catch regressions early.

set -e

echo "Checking sync client invariants..."

# INVARIANT 1: startSyncClient must NOT call resetConnection
# Background: Calling resetConnection in startSyncClient caused rapid
# connect/disconnect cycling (100+ logs per second).
# Fixed in commit 49250df.
if grep -n "resetConnection" src/background/index.ts | grep -v "export function resetSyncClientConnection" | grep -v "syncClient.resetConnection()" > /dev/null 2>&1; then
  # Check if it's inside startSyncClient function
  if awk '/function startSyncClient/,/^}/' src/background/index.ts | grep "resetConnection" > /dev/null 2>&1; then
    echo "❌ REGRESSION: resetConnection() found inside startSyncClient()"
    echo "   This causes rapid connect/disconnect cycling."
    echo "   See commit 49250df for the fix."
    exit 1
  fi
fi

echo "✅ Sync client invariants OK"
