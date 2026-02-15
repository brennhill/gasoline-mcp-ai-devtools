#!/bin/bash
# framework-smoke.sh — Smoke test overrides for the base framework.
# Sources framework.sh and adds human-interactive helpers:
#   pause_for_human()    — wait for Enter between tests
#   interact_and_wait()  — fire interact + poll for completion
#   log_diagnostic()     — append raw responses to diagnostics file
#   Overridden pass()/fail() that call pause_for_human
set -eo pipefail

# ── Source base framework ─────────────────────────────────
SMOKE_FRAMEWORK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SMOKE_FRAMEWORK_DIR/../tests/framework.sh"

# ── Shared mutable state (set by modules, read by later modules) ──
EXTENSION_CONNECTED=false
PILOT_ENABLED=false
SMOKE_MARKER="GASOLINE_SMOKE_$(date +%s)_$$"
SKIP_COUNT=0
CURRENT_TEST_ID=""

# ── Color support ─────────────────────────────────────────
# Use ANSI colors only when stdout is a TTY
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    _CLR_PASS='\033[0;32m'   # green
    _CLR_FAIL='\033[0;31m'   # red
    _CLR_SKIP='\033[0;33m'   # yellow
    _CLR_RESET='\033[0m'
    _INTERACTIVE=true
else
    _CLR_PASS=''
    _CLR_FAIL=''
    _CLR_SKIP=''
    _CLR_RESET=''
    _INTERACTIVE=false
fi

# ── Composable cleanup system ─────────────────────────────
# Modules register cleanup functions instead of overwriting the EXIT trap.
# _smoke_cleanup_handlers is a space-separated list of function names.
_smoke_cleanup_handlers=""

# Register a cleanup function. Called by modules instead of `trap ... EXIT`.
register_cleanup() {
    _smoke_cleanup_handlers="$_smoke_cleanup_handlers $1"
}

# Master cleanup: runs all registered handlers, then base cleanup.
_smoke_master_cleanup() {
    for handler in $_smoke_cleanup_handlers; do
        "$handler" 2>/dev/null || true
    done
    # Base cleanup: kill daemon, remove temp dir
    kill_server 2>/dev/null || true
    pkill -f "upload-server.py" 2>/dev/null || true
    [ -n "$TEMP_DIR" ] && rm -rf "$TEMP_DIR" 2>/dev/null || true
}

# ── Persistent output files ──────────────────────────────
# Output and diagnostics go to well-known persistent locations so they
# survive crashes and are easy to find. Printed at start of every run.
SMOKE_OUTPUT_DIR="${HOME}/.gasoline/smoke-results"
mkdir -p "$SMOKE_OUTPUT_DIR"
DIAGNOSTICS_FILE="$SMOKE_OUTPUT_DIR/diagnostics.log"
SMOKE_OUTPUT_FILE="$SMOKE_OUTPUT_DIR/output.log"

init_smoke() {
    local port="${1:-7890}"
    init_framework "$port" "/dev/null"

    # Override OUTPUT_FILE to use persistent location instead of TEMP_DIR
    OUTPUT_FILE="$SMOKE_OUTPUT_FILE"

    # Initialize output files
    echo "Smoke Test Output — $(date)" > "$OUTPUT_FILE"
    echo "Smoke Test Diagnostics — $(date)" > "$DIAGNOSTICS_FILE"
    echo "Port: $port" >> "$DIAGNOSTICS_FILE"
    echo "======================================" >> "$DIAGNOSTICS_FILE"

    # Print file locations and mode upfront so they're always visible
    echo ""
    echo "  Output:      $OUTPUT_FILE"
    echo "  Diagnostics: $DIAGNOSTICS_FILE"
    if [ "$_INTERACTIVE" = "true" ]; then
        echo "  Mode:        interactive (pauses between tests)"
    else
        echo "  Mode:        non-interactive (CI/piped — no pauses)"
    fi
    echo ""

    # Set the master EXIT trap (never overwritten by modules)
    trap _smoke_master_cleanup EXIT

    # Trap ERR so crashes under set -e are immediately visible.
    # Logs the failing command, line, and function to both stderr and diagnostics.
    trap '_smoke_on_error $LINENO "${FUNCNAME[0]:-main}" "${BASH_COMMAND}"' ERR
}

_smoke_on_error() {
    local line="$1" func="$2" cmd="$3"
    local test_ctx="${CURRENT_TEST_ID:-unknown}"
    printf "\n  ${_CLR_FAIL}!!! CRASH${_CLR_RESET} [${test_ctx}] at line $line in $func(): $cmd\n" >&2
    echo "  !!! Diagnostics: $DIAGNOSTICS_FILE" >&2
    {
        echo ""
        echo "!!! CRASH [${test_ctx}] at $(date +%H:%M:%S)"
        echo "    Test:     $test_ctx"
        echo "    Line:     $line"
        echo "    Function: $func"
        echo "    Command:  $cmd"
        echo "    Pass=$PASS_COUNT Fail=$FAIL_COUNT Skip=$SKIP_COUNT"
    } >> "$DIAGNOSTICS_FILE" 2>/dev/null
    # Count crash as a failure but don't exit — let the runner continue
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

# ── Override begin_test to track current test ID ──────────
begin_test() {
    CURRENT_TEST_ID="$1"
    local name="$2"
    local purpose="${3:-}"
    local trust="${4:-}"
    {
        echo "============================================================"
        echo "TEST ${CURRENT_TEST_ID}: ${name}"
        echo "============================================================"
        echo "Purpose: ${purpose}"
        echo "Trust:   ${trust}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
    # Log test start to diagnostics immediately
    echo "--- $(date +%H:%M:%S) START ${CURRENT_TEST_ID}: ${name}" >> "$DIAGNOSTICS_FILE"
}

# ── Override pass/fail/skip to pause for human + color ────
pass() {
    local description="$1"
    PASS_COUNT=$((PASS_COUNT + 1))
    # Colorized to terminal, plain to output file
    printf "  ${_CLR_PASS}PASS${_CLR_RESET} [${CURRENT_TEST_ID}]: %s\n\n" "$description"
    echo "  PASS [${CURRENT_TEST_ID}]: ${description}" >> "$OUTPUT_FILE"
    echo "  $(date +%H:%M:%S) PASS [${CURRENT_TEST_ID}]: ${description}" >> "$DIAGNOSTICS_FILE"
    pause_for_human
}

fail() {
    local description="$1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    # Log to diagnostics FIRST (before display) so it survives crashes
    echo "  $(date +%H:%M:%S) FAIL [${CURRENT_TEST_ID}]: ${description}" >> "$DIAGNOSTICS_FILE"
    printf "  ${_CLR_FAIL}FAIL${_CLR_RESET} [${CURRENT_TEST_ID}]: %s\n\n" "$description"
    echo "  FAIL [${CURRENT_TEST_ID}]: ${description}" >> "$OUTPUT_FILE"
    pause_for_human
}

skip() {
    local description="$1"
    SKIP_COUNT=$((SKIP_COUNT + 1))
    echo "  $(date +%H:%M:%S) SKIP [${CURRENT_TEST_ID}]: ${description}" >> "$DIAGNOSTICS_FILE"
    printf "  ${_CLR_SKIP}SKIP${_CLR_RESET} [${CURRENT_TEST_ID}]: %s\n\n" "$description"
    echo "  SKIP [${CURRENT_TEST_ID}]: ${description}" >> "$OUTPUT_FILE"
}

pause_for_human() {
    # Skip interactive prompt in CI or non-TTY environments
    if [ -n "${CI:-}" ] || [ ! -t 0 ]; then
        return
    fi
    echo "  -- Press Enter to continue, Ctrl-C to abort --"
    read -r
    echo ""
}

# ── Log diagnostic data ──────────────────────────────────
log_diagnostic() {
    local test_name="$1"
    local action="$2"
    local response="$3"
    local result="${4:-}"
    {
        echo ""
        echo "======= $test_name — $action ======="
        echo "Response:"
        echo "$response" | head -100
        if [ -n "$result" ]; then
            echo ""
            echo "Result:"
            echo "$result" | head -100
        fi
    } >> "$DIAGNOSTICS_FILE"
}

# ── Interact helper ──────────────────────────────────────
# Fires an interact command and waits for completion via polling.
# Sets INTERACT_RESULT to the command result text (or empty on timeout).
# Always returns 0 — callers inspect $INTERACT_RESULT for pass/fail.
# (Returning non-zero under set -eo pipefail kills the entire script.)
interact_and_wait() {
    local action="$1"
    local args="$2"
    local max_polls="${3:-20}"

    local response
    response=$(call_tool "interact" "$args")
    local content_text
    content_text=$(extract_content_text "$response")

    {
        echo ""
        echo "- $(date +%H:%M:%S) interact($action) initial response:"
        echo "$content_text" | head -50
    } >> "$DIAGNOSTICS_FILE"

    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)

    if [ -z "$corr_id" ]; then
        INTERACT_RESULT="$content_text"
        return 0
    fi

    for i in $(seq 1 "$max_polls"); do
        sleep 0.5
        local poll_response
        poll_response=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")
        local poll_text
        poll_text=$(extract_content_text "$poll_response")

        if echo "$poll_text" | grep -q '"status":"complete"'; then
            INTERACT_RESULT="$poll_text"
            {
                echo "  Complete after poll $i"
                echo "$poll_text" | head -30
            } >> "$DIAGNOSTICS_FILE"
            return 0
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            INTERACT_RESULT="$poll_text"
            {
                echo "  Failed after poll $i"
                echo "$poll_text" | head -30
            } >> "$DIAGNOSTICS_FILE"
            return 0
        fi
    done

    INTERACT_RESULT="timeout waiting for $action"
    {
        echo "  Timeout after $max_polls polls"
    } >> "$DIAGNOSTICS_FILE"
    return 0
}
