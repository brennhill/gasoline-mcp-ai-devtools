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
ONLY_MODULE=""

# Parse args: positional port + optional --start-from
while [ $# -gt 0 ]; do
    case "$1" in
        --help|-h)
            echo "Usage: bash scripts/smoke-test.sh [PORT] [OPTIONS]"
            echo ""
            echo "  PORT           Daemon port (default: 7890)"
            echo "  --start-from   Skip modules until MODULE matches (substring)"
            echo "  --only         Run only the matching module, then stop"
            echo "                 Examples: --only 15, --only upload"
            echo ""
            echo "Modules: 01-bootstrap, 02-core-telemetry, 03-observe-modes,"
            echo "  04-network-websocket, 05-interact-dom, 06-interact-state,"
            echo "  07-generate-formats, 08-configure-features, 09-perf-analysis,"
            echo "  10-recording, 11-subtitle-screenshot, 12-cross-cutting,"
            echo "  13-draw-mode, 15-file-upload, 14-stability-shutdown"
            exit 0
            ;;
        --start-from)
            if [ -z "${2:-}" ]; then
                echo "Error: --start-from requires a module name or number." >&2
                echo "Example: --start-from 05" >&2
                exit 1
            fi
            START_FROM="$2"
            shift 2
            ;;
        --only)
            if [ -z "${2:-}" ]; then
                echo "Error: --only requires a module name or number." >&2
                echo "Example: --only 15" >&2
                exit 1
            fi
            # --only is implemented as --start-from + stop after match
            START_FROM="$2"
            ONLY_MODULE="$2"
            shift 2
            ;;
        *)
            PORT="$1"
            shift
            ;;
    esac
done

# ── Dependency checks ─────────────────────────────────────
_smoke_check_deps() {
    local missing=""
    for cmd in jq curl python3 lsof; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            missing="$missing $cmd"
        fi
    done
    if ! command -v timeout >/dev/null 2>&1 && ! command -v gtimeout >/dev/null 2>&1; then
        missing="$missing timeout(brew install coreutils)"
    fi
    if [ -n "$missing" ]; then
        echo "FATAL: Missing dependencies:$missing" >&2
        exit 1
    fi
}
_smoke_check_deps

# ── Source framework (initializes globals) ────────────────
# shellcheck source=/dev/null
source "$SMOKE_DIR/framework-smoke.sh"
init_smoke "$PORT"
# Note: init_smoke sets the EXIT trap (_smoke_master_cleanup).
# Do NOT set another EXIT trap here — use register_cleanup instead.

# ── Module list ──────────────────────────────────────────
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
    "15-file-upload.sh"       # 15 runs before 14: upload needs a live daemon
    "21-macro-recording.sh"
    "22-log-aggregation.sh"
    "14-stability-shutdown.sh" # 14 must be last: it kills the daemon
)

# ── Port conflict check ───────────────────────────────────
if lsof -ti :"$PORT" >/dev/null 2>&1; then
    echo "WARNING: Port $PORT is already in use. Killing existing process..." >&2
    lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 0.5
fi

BINARY_VERSION=$("$WRAPPER" --version 2>/dev/null || echo "unknown")
echo ""
echo "============================================================"
echo "  GASOLINE SMOKE TEST SUITE"
echo "  Port: $PORT | $(date)"
echo "  Binary: $BINARY_VERSION"
echo "  Expected: $(cat "$RUNNER_DIR/../VERSION" 2>/dev/null || echo "?")"
echo "  ${#MODULES[@]} modules"
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
        echo "  Daemon not healthy. Starting fresh with --enable-os-upload-automation..."
        start_daemon_with_flags --enable-os-upload-automation || true
        # Give daemon extra time to fully initialize (MCP readiness lags HTTP readiness)
        sleep 2
    fi

    # Verify daemon is actually responding to MCP calls
    probe_resp=$(call_tool "observe" '{"what":"page"}' 2>/dev/null)
    if echo "$probe_resp" | grep -q "starting up" 2>/dev/null; then
        echo "  Daemon still starting, waiting 3s..."
        sleep 3
    fi

    health_body=$(get_http_body "http://localhost:${PORT}/health" 2>/dev/null || echo "{}")
    daemon_ver=$(echo "$health_body" | jq -r '.version // "unknown"' 2>/dev/null || echo "unknown")
    echo "  Daemon version: v${daemon_ver}"
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
MODULE_NUM=0
for module in "${MODULES[@]}"; do
    # Skip modules until START_FROM match (substring: "05" matches "05-interact-dom.sh")
    if [ "$SKIP_UNTIL_FOUND" = "true" ]; then
        if [[ "$module" == *"$START_FROM"* ]]; then
            SKIP_UNTIL_FOUND=""
        else
            continue
        fi
    fi

    MODULE_NUM=$((MODULE_NUM + 1))

    module_path="$SMOKE_DIR/$module"
    if [ ! -f "$module_path" ]; then
        echo "WARNING: Module $module not found, skipping."
        continue
    fi
    echo "── Module $MODULE_NUM/${#MODULES[@]}: $module ──"
    # Run the module in a subshell so its `set -eo pipefail` can't kill the runner.
    # Shared state (EXTENSION_CONNECTED, PILOT_ENABLED, etc.) is exported via a
    # temp file since subshell variables don't propagate to the parent.
    _state_file="$TEMP_DIR/smoke_state_$$"
    (
        # shellcheck source=/dev/null
        source "$module_path"
        # Export mutable state back to parent
        cat > "$_state_file" <<STATE_EOF
EXTENSION_CONNECTED=$EXTENSION_CONNECTED
PILOT_ENABLED=$PILOT_ENABLED
PASS_COUNT=$PASS_COUNT
FAIL_COUNT=$FAIL_COUNT
SKIP_COUNT=$SKIP_COUNT
DAEMON_PID=$DAEMON_PID
STATE_EOF
    ) || true
    # Import state from subshell
    if [ -f "$_state_file" ]; then
        # shellcheck source=/dev/null
        source "$_state_file"
        rm -f "$_state_file"
    fi

    # --only: stop after running the matched module
    if [ -n "$ONLY_MODULE" ] && [[ "$module" == *"$ONLY_MODULE"* ]]; then
        echo ""
        echo "  (--only $ONLY_MODULE: stopping after this module)"
        break
    fi
done

# ── Detect --start-from / --only with no match ────────────
if [ "$SKIP_UNTIL_FOUND" = "true" ]; then
    echo "ERROR: No module matched '$START_FROM'." >&2
    echo "  Available modules: ${MODULES[*]}" >&2
    exit 1
fi

if [ "$MODULE_NUM" -eq 0 ] && [ -z "$START_FROM" ]; then
    echo "ERROR: No modules were run." >&2
    exit 1
fi

# ── Cleanup: kill any orphaned test servers ───────────────
pkill -f "upload-server.py" 2>/dev/null || true
kill_server 2>/dev/null || true
if [ -f "$RUNNER_DIR/cleanup-test-daemons.sh" ]; then
    bash "$RUNNER_DIR/cleanup-test-daemons.sh" --quiet >/dev/null 2>&1 || true
fi

# ── Summary ──────────────────────────────────────────────
ELAPSED=$(( $(date +%s) - START_TIME ))
{
    echo ""
    echo "============================================================"
    echo "SMOKE TEST SUMMARY"
    echo "============================================================"
    echo "  Passed:  $PASS_COUNT"
    echo "  Failed:  $FAIL_COUNT"
    echo "  Skipped: $SKIP_COUNT"
    echo "  Time:    ${ELAPSED}s"
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
    echo "  Output:      $OUTPUT_FILE"
    echo "  Diagnostics: $DIAGNOSTICS_FILE"
    echo ""
} | tee -a "$OUTPUT_FILE"

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
