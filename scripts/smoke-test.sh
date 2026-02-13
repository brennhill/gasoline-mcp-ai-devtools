#!/bin/bash
# smoke-test.sh — Human smoke test for Gasoline MCP.
# Sources modular test modules sequentially. Shared mutable state
# (EXTENSION_CONNECTED, PILOT_ENABLED, SMOKE_MARKER) flows across modules.
#
# Usage:
#   bash scripts/smoke-test.sh                    # default port 7890
#   bash scripts/smoke-test.sh 7890               # explicit port
#   bash scripts/smoke-test.sh --start-from 12    # resume from module 12
#   bash scripts/smoke-test.sh 7890 --start-from 07
set -euo pipefail

RUNNER_DIR="$(cd "$(dirname "$0")" && pwd)"
SMOKE_DIR="$RUNNER_DIR/smoke-tests"
PORT="7890"
START_FROM=""

# Parse args: positional port + optional --start-from
while [ $# -gt 0 ]; do
    case "$1" in
        --start-from)
            START_FROM="${2:-}"
            shift 2
            ;;
        *)
            PORT="$1"
            shift
            ;;
    esac
done

# ── Source framework (initializes globals) ────────────────
# shellcheck source=/dev/null
source "$SMOKE_DIR/framework-smoke.sh"
init_smoke "$PORT"

echo ""
echo "============================================================"
echo "  GASOLINE SMOKE TEST SUITE"
echo "  Port: $PORT | $(date)"
echo "  80 tests across 14 modules"
echo "============================================================"
echo ""

# ── Resume support ───────────────────────────────────────
# --start-from skips all modules before the target.
# Instead of running full bootstrap, does a quick health probe
# to set EXTENSION_CONNECTED and PILOT_ENABLED.
SKIP_UNTIL_FOUND="${START_FROM:+true}"

if [ -n "$START_FROM" ]; then
    echo "  Resuming from module matching: $START_FROM"
    echo ""

    # Quick state init — replaces full bootstrap when resuming.
    # Assumes daemon is already running from the previous run.
    if ! wait_for_health 30; then
        echo "  Daemon not healthy. Starting fresh..."
        call_tool "observe" '{"what":"page"}' >/dev/null 2>&1 || true
        sleep 2
        wait_for_health 30 || true
    fi

    health_body=$(get_http_body "http://localhost:${PORT}/health" 2>/dev/null || echo "{}")
    if echo "$health_body" | jq -e '.capture.available == true' >/dev/null 2>&1; then
        EXTENSION_CONNECTED=true
        echo "  Extension: connected"
    else
        echo "  Extension: NOT connected (some tests will skip)"
    fi
    # Probe pilot by checking if interact responds without "disabled"
    PILOT_ENABLED=true
    echo "  Pilot: assumed enabled (from previous run)"
    echo ""
fi

# ── Run modules in order ─────────────────────────────────
MODULES=(
    "01-bootstrap.sh"
    "02-core-telemetry.sh"
    "03-observe-modes.sh"
    "04-network-websocket.sh"
    "05-interact-dom.sh"
    "06-interact-state.sh"
    "07-generate-formats.sh"
    "08-configure-features.sh"
    "09-perf-analysis.sh"
    "10-recording.sh"
    "11-subtitle-screenshot.sh"
    "12-cross-cutting.sh"
    "13-draw-mode.sh"
    "14-stability-shutdown.sh"
)

for module in "${MODULES[@]}"; do
    # Skip modules until START_FROM match
    if [ "$SKIP_UNTIL_FOUND" = "true" ]; then
        if [[ "$module" == *"$START_FROM"* ]]; then
            SKIP_UNTIL_FOUND=""
        else
            continue
        fi
    fi

    module_path="$SMOKE_DIR/$module"
    if [ ! -f "$module_path" ]; then
        echo "WARNING: Module $module not found, skipping."
        continue
    fi
    # shellcheck source=/dev/null
    source "$module_path"
done

# ── Summary ──────────────────────────────────────────────
{
    echo ""
    echo "============================================================"
    echo "SMOKE TEST SUMMARY"
    echo "============================================================"
    echo "  Passed:  $PASS_COUNT"
    echo "  Failed:  $FAIL_COUNT"
    echo "  Skipped: $SKIP_COUNT"
    echo ""
    if [ "$FAIL_COUNT" -eq 0 ]; then
        if [ "$SKIP_COUNT" -gt 0 ]; then
            echo "  Result: PASSED (with $SKIP_COUNT skipped)"
        else
            echo "  Result: ALL PASSED"
        fi
    else
        echo "  Result: FAILED"
    fi
    echo ""
    echo "Diagnostics saved to: $DIAGNOSTICS_FILE"
    echo "View with: cat $DIAGNOSTICS_FILE"
    echo ""
} | tee -a "$OUTPUT_FILE"

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
