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
SMOKE_MARKER="GASOLINE_SMOKE_$(date +%s)"
SKIPPED_COUNT=0

# ── Diagnostic log file ──────────────────────────────────
DIAGNOSTICS_FILE="/tmp/gasoline-smoke-diagnostics-$$.log"

init_smoke() {
    local port="${1:-7890}"
    init_framework "$port" "/dev/null"
    echo "Smoke Test Diagnostics — $(date)" > "$DIAGNOSTICS_FILE"
    echo "Port: $port" >> "$DIAGNOSTICS_FILE"
    echo "======================================" >> "$DIAGNOSTICS_FILE"
}

# ── Override pass/fail/skip to pause for human ───────────
pass() {
    local description="$1"
    PASS_COUNT=$((PASS_COUNT + 1))
    {
        echo "  PASS: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
    pause_for_human
}

fail() {
    local description="$1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    {
        echo "  FAIL: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
    pause_for_human
}

skip() {
    local description="$1"
    SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
    {
        echo "  SKIP: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

pause_for_human() {
    echo "  -- Press Enter to continue, Ctrl-C to abort --"
    read -r
    echo ""
}

# ── Log diagnostic data ──────────────────────────────────
log_diagnostic() {
    local test_name="$1"
    local action="$2"
    local response="$3"
    local result="$4"
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
interact_and_wait() {
    local action="$1"
    local args="$2"
    local max_polls="${3:-15}"

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
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//')

    if [ -z "$corr_id" ]; then
        INTERACT_RESULT="$content_text"
        return 1
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
            return 1
        fi
    done

    INTERACT_RESULT="timeout waiting for $action"
    {
        echo "  Timeout after $max_polls polls"
    } >> "$DIAGNOSTICS_FILE"
    return 1
}
