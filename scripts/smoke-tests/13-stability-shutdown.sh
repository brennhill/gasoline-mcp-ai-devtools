#!/bin/bash
# 13-stability-shutdown.sh — S.14-S.16: Post-barrage stability and graceful shutdown.
# MUST BE LAST — S.16 kills the daemon.
set -eo pipefail

begin_category "13" "Stability & Shutdown" "3"

# ── Test S.14: observe(page) still works after all actions ─
begin_test "S.14" "Page state survives action barrage" \
    "After navigate + JS execution + clicks + forms + WS, observe(page) still returns valid data" \
    "Verifies no corruption from heavy interaction"

run_test_s14() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qE 'https?://'; then
        pass "observe(page) still returns a valid URL after all actions."
    else
        fail "observe(page) broken after actions. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s14

# ── Test S.15: Health still OK after everything ──────────
begin_test "S.15" "Health still OK after everything" \
    "Verify daemon is healthy after all the interaction and observation" \
    "Detects memory leaks, crashes, or degraded state"

run_test_s15() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)
    if [ "$status_val" != "ok" ]; then
        fail "Health status='$status_val' after test barrage. Body: $(truncate "$body")"
        return
    fi

    pass "Daemon still healthy after full smoke test. status='ok'."
}
run_test_s15

# ── Test S.16: Graceful shutdown ─────────────────────────
begin_test "S.16" "Graceful shutdown via --stop" \
    "Run --stop, verify port is freed and PID file is cleaned up" \
    "Ungraceful shutdown leaves orphan processes and stale PID files"

run_test_s16() {
    local stop_output
    stop_output=$("$WRAPPER" --stop --port "$PORT" 2>&1)
    local stop_exit=$?

    if [ $stop_exit -ne 0 ]; then
        fail "--stop exited with code $stop_exit. Output: $(truncate "$stop_output")"
        return
    fi

    sleep 1

    if lsof -ti :"$PORT" >/dev/null 2>&1; then
        fail "Port $PORT still occupied after --stop."
        return
    fi

    local pid_file="$HOME/.gasoline-${PORT}.pid"
    if [ -f "$pid_file" ]; then
        fail "PID file $pid_file still exists after --stop."
        rm -f "$pid_file"
        return
    fi

    pass "Graceful shutdown: --stop exited 0, port freed, PID file cleaned."
}
run_test_s16
