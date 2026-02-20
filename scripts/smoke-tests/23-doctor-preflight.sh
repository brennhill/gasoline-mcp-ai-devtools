#!/bin/bash
# 23-doctor-preflight.sh — 23.1-23.3: Doctor/check mode preflight tests.
# Verifies --doctor and --check output format, exit codes, and diagnostic checks.
set -eo pipefail

begin_category "23" "Doctor Preflight" "3"

# ── Test 23.1: --doctor exits with structured output ─────
begin_test "23.1" "[DAEMON ONLY] Doctor mode produces structured output" \
    "gasoline-mcp --doctor should run diagnostics and exit" \
    "Tests: doctor preflight runs and produces readable output"

run_test_23_1() {
    local output exit_code
    output=$("$WRAPPER" --doctor --port "$PORT" 2>&1) || exit_code=$?

    if [ -z "$output" ]; then
        fail "Doctor mode produced no output."
        return
    fi

    log_diagnostic "23.1" "doctor" "$output"

    # Doctor output should mention version and port
    if echo "$output" | grep -qi "version\|port\|gasoline"; then
        pass "Doctor mode produced structured output (${#output} chars). Exit code: ${exit_code:-0}"
    else
        fail "Doctor output missing expected fields. Output: $(truncate "$output" 300)"
    fi
}
run_test_23_1

# ── Test 23.2: --check is an alias of --doctor ──────────
begin_test "23.2" "[DAEMON ONLY] Check mode is a working alias of doctor" \
    "gasoline-mcp --check should produce similar output to --doctor" \
    "Tests: --check flag works"

run_test_23_2() {
    local output
    output=$("$WRAPPER" --check --port "$PORT" 2>&1) || true

    if [ -z "$output" ]; then
        fail "Check mode produced no output."
        return
    fi

    if echo "$output" | grep -qi "version\|port\|gasoline"; then
        pass "Check mode produced structured output. (${#output} chars)"
    else
        fail "Check output missing expected fields. Output: $(truncate "$output" 300)"
    fi
}
run_test_23_2

# ── Test 23.3: Doctor checks port availability ──────────
begin_test "23.3" "[DAEMON ONLY] Doctor reports port status" \
    "Doctor should indicate whether the configured port is available or in use" \
    "Tests: port availability preflight check"

run_test_23_3() {
    # Start daemon on PORT first so doctor detects it
    ensure_server_running

    local output
    output=$("$WRAPPER" --doctor --port "$PORT" 2>&1) || true

    log_diagnostic "23.3" "doctor-port" "$output"

    # Should mention port status (available, in use, running, etc.)
    if echo "$output" | grep -qi "port\|running\|pid\|listening\|available\|in.use"; then
        pass "Doctor reports port status information."
    else
        fail "Doctor output missing port status. Output: $(truncate "$output" 300)"
    fi
}
run_test_23_3
