#!/bin/bash
# smoke-test-runner.sh — Orchestrator for modular smoke tests.
# Sources each module sequentially so shared mutable state
# (EXTENSION_CONNECTED, PILOT_ENABLED, SMOKE_MARKER) flows across modules.
#
# Usage:
#   bash scripts/smoke-test-runner.sh          # default port 7890
#   bash scripts/smoke-test-runner.sh 7890     # explicit port
set -euo pipefail

RUNNER_DIR="$(cd "$(dirname "$0")" && pwd)"
SMOKE_DIR="$RUNNER_DIR/smoke-tests"
PORT="${1:-7890}"

# ── Source framework (initializes globals) ────────────────
# shellcheck source=/dev/null
source "$SMOKE_DIR/framework-smoke.sh"
init_smoke "$PORT"

echo ""
echo "============================================================"
echo "  GASOLINE SMOKE TEST SUITE"
echo "  Port: $PORT | $(date)"
echo "  68 tests across 13 modules"
echo "============================================================"
echo ""

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
    "13-stability-shutdown.sh"
)

for module in "${MODULES[@]}"; do
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
    echo "  Skipped: $SKIPPED_COUNT"
    echo ""
    if [ "$FAIL_COUNT" -eq 0 ]; then
        if [ "$SKIPPED_COUNT" -gt 0 ]; then
            echo "  Result: PASSED (with $SKIPPED_COUNT skipped)"
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
