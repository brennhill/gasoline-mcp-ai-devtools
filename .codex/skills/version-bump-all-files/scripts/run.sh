#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: bash .codex/skills/version-bump-all-files/scripts/run.sh <OLD_VERSION> <NEW_VERSION>

Example:
  bash .codex/skills/version-bump-all-files/scripts/run.sh 0.7.10 0.7.11
EOF
}

if [ "${1:-}" = "" ] || [ "${2:-}" = "" ]; then
  usage
  exit 1
fi

OLD_VERSION="$1"
NEW_VERSION="$2"

if ! [[ "$OLD_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "❌ OLD_VERSION is not strict semver: $OLD_VERSION"
  exit 1
fi

if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "❌ NEW_VERSION is not strict semver: $NEW_VERSION"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../" && pwd)"
cd "$REPO_ROOT"

CURRENT_VERSION="$(tr -d '[:space:]' < VERSION)"
echo "Current VERSION: $CURRENT_VERSION"
echo "Target VERSION:  $NEW_VERSION"

if [ "$CURRENT_VERSION" != "$NEW_VERSION" ]; then
  echo "==> Running bump-version.js"
  node scripts/bump-version.js "$NEW_VERSION"

  if grep -q '^sync-version:' Makefile; then
    echo "==> Running make sync-version"
    make sync-version
  else
    echo "⚠️  sync-version target not found; skipping"
  fi
else
  echo "==> VERSION already $NEW_VERSION; skipping bump-version.js"
fi

echo "==> Running validate-versions gate"
bash scripts/validate-versions.sh

echo "==> Running stale reference sweep for $OLD_VERSION"
if STALE_MATCHES="$(rg -F -n "$OLD_VERSION" --glob '!dist/**' --glob '!node_modules/**' --glob '!.git/**' || true)" && [ -n "$STALE_MATCHES" ]; then
  echo "❌ Stale version references remain:"
  echo "$STALE_MATCHES"
  exit 1
fi

echo "==> Running installer regression guard"
node --test tests/extension/install-script-extension-source.test.js

echo "✅ Version bump workflow complete. No stale $OLD_VERSION references found."
