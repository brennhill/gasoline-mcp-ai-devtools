#!/usr/bin/env bash
# Test: Cold start timing
#
# Verifies:
# - Server starts within 2 seconds (cold start requirement)
# - Server accepts connections immediately after health check passes

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

echo "Testing cold start timing..."

# Test 1: Server starts within 2 seconds
test_cold_start_timing() {
    local port
    port=$(find_free_port)

    local start_time
    start_time=$(date +%s%N)

    # Start server
    "$GASOLINE_BINARY" --port "$port" < /dev/null &
    local pid=$!

    # Wait for server (max 2 seconds)
    local elapsed=0
    local max_wait=20  # 2 seconds in 100ms increments

    while [[ $elapsed -lt $max_wait ]]; do
        if curl -s "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
            break
        fi
        sleep 0.1
        ((elapsed++))
    done

    local end_time
    end_time=$(date +%s%N)

    # Cleanup
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true

    # Check if server started
    if [[ $elapsed -ge $max_wait ]]; then
        echo "Server failed to start within 2 seconds" >&2
        return 1
    fi

    # Calculate actual time
    local duration_ms=$(( (end_time - start_time) / 1000000 ))
    echo "Cold start completed in ${duration_ms}ms"

    if [[ $duration_ms -gt 2000 ]]; then
        echo "Cold start took too long: ${duration_ms}ms > 2000ms" >&2
        return 1
    fi

    return 0
}

# Test 2: Server accepts MCP requests immediately after health passes
test_immediate_mcp_after_health() {
    local port
    port=$(find_free_port)

    # Start server
    "$GASOLINE_BINARY" --port "$port" < /dev/null &
    local pid=$!

    # Wait for health
    if ! wait_for_server "$port" 5; then
        kill "$pid" 2>/dev/null || true
        echo "Server failed to start" >&2
        return 1
    fi

    # Immediately send MCP request
    local response
    response=$(curl -s -X POST "http://127.0.0.1:${port}/mcp" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}')

    # Cleanup
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true

    # Verify response
    assert_mcp_success "$response" "MCP should respond immediately after health check"
}

# Run tests
FAILED=0

run_test "Cold start within 2 seconds" test_cold_start_timing || ((FAILED++))
run_test "MCP works immediately after health" test_immediate_mcp_after_health || ((FAILED++))

exit $FAILED
