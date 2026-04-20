#!/usr/bin/env bash
# check-openapi-server-conformance.sh — Boots the Kaboom daemon and runs
# Schemathesis against cmd/browser-agent/openapi.json to verify the server's
# responses match the spec. Catches schema-level drift that static type-gen
# and URL presence checks cannot (e.g., server renames a field, changes an
# enum, or drops a documented response code).
#
# Scope (intentionally conservative — first rollout):
#   - GET-only (POST bodies fuzz too hard without schemas everywhere)
#   - Excludes side-effect paths (/shutdown, /clear, /setup, HTML viewers)
#   - Excludes paths with known pre-existing drift (see baseline-skip block)
#   - Check: response_schema_conformance only
#
# Usage:
#   ./scripts/check-openapi-server-conformance.sh              # uses an ephemeral port
#   PORT=18000 ./scripts/check-openapi-server-conformance.sh   # explicit port

set -euo pipefail

PORT="${PORT:-18890}"
BASE_URL="http://127.0.0.1:${PORT}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="${REPO_ROOT}/cmd/browser-agent/openapi.json"
STATE_DIR="$(mktemp -d)"
DAEMON_LOG="$(mktemp)"

# Synthetic extension identity — bypasses extensionOnly middleware so the real
# endpoint logic runs instead of returning 403. The Origin is a valid-shape
# chrome extension ID; since no KABOOM_EXTENSION_ID env is set, the daemon
# accepts any extension Origin (see internal/upgrade/nonce.go Pin logic).
EXT_ORIGIN="chrome-extension://abcdefghijklmnopabcdefghijklmnop"
EXT_CLIENT="kaboom-extension/ci"

DAEMON_PID=""
cleanup() {
  if [ -n "$DAEMON_PID" ]; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  rm -rf "$STATE_DIR" "$DAEMON_LOG"
}
trap cleanup EXIT

# Start daemon in background.
echo "▶ Booting Kaboom daemon on :${PORT} (state-dir=${STATE_DIR})"
go run ./cmd/browser-agent --daemon --port "$PORT" --state-dir "$STATE_DIR" \
  > "$DAEMON_LOG" 2>&1 &
DAEMON_PID=$!

# Wait for /health to come up. 30s is generous — the daemon usually boots in <2s.
DEADLINE=$((SECONDS + 30))
until curl -sf "${BASE_URL}/health" -H "X-Kaboom-Client: ${EXT_CLIENT}" >/dev/null 2>&1; do
  if [ "$SECONDS" -ge "$DEADLINE" ]; then
    echo "✗ Daemon failed to come up within 30s. Log:"
    cat "$DAEMON_LOG"
    exit 1
  fi
  if ! kill -0 "$DAEMON_PID" 2>/dev/null; then
    echo "✗ Daemon exited during startup. Log:"
    cat "$DAEMON_LOG"
    exit 1
  fi
  sleep 0.3
done
echo "✓ Daemon healthy"

# Run schemathesis. Exclusions fall into three buckets:
#
# (1) Non-API paths — HTML viewers, websocket test endpoints, setup wizards.
#     These aren't JSON endpoints and can't meaningfully fuzz.
# (2) Side-effect paths — /shutdown kills the daemon, /upgrade/install fires
#     the installer, /clear wipes buffers. Running schemathesis against them
#     would either break the test run or trigger real state changes.
# (3) Baseline-skip — endpoints with pre-existing drift that we'd need to
#     either fix in the spec or fix in the Go handler. Track these as open
#     issues; remove from the exclusion list once fixed. Adding a NEW path
#     here requires justification.

SCHEMATHESIS_BIN="${SCHEMATHESIS_BIN:-schemathesis}"

"$SCHEMATHESIS_BIN" run \
  -u "$BASE_URL" \
  -H "Origin: ${EXT_ORIGIN}" \
  -H "X-Kaboom-Client: ${EXT_CLIENT}" \
  --include-method GET \
  --exclude-path-regex '^/tests' \
  --exclude-path '/logs.html' \
  --exclude-path '/docs' \
  --exclude-path '/openapi.json' \
  --exclude-path '/setup' \
  --exclude-path '/insecure-proxy' \
  --exclude-path '/shutdown' \
  --exclude-path '/clear' \
  --exclude-path '/api/status' \
  --exclude-path '/snapshot' \
  --exclude-path '/clients/{id}' \
  --checks response_schema_conformance \
  --max-examples 10 \
  --request-timeout 5 \
  "$SPEC"
