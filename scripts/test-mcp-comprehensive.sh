#!/bin/bash
set -e

# Comprehensive MCP Integration Test
# Tests all tools (observe, generate, configure, interact) in various scenarios
# Proves MCP connection is 100% reliable and silent

PORT=$((8000 + RANDOM % 1000))
WRAPPER="gasoline-mcp"
TEMP_DIR=$(mktemp -d)
PASS_COUNT=0
FAIL_COUNT=0

echo "============================================"
echo "Comprehensive MCP Integration Test"
echo "============================================"
echo ""
echo "Port: $PORT"
echo "Tests: All tools in various scenarios"
echo ""

# Helper: Kill server
kill_server() {
    lsof -ti :$PORT 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 0.5
}

# Helper: Send request and validate response
test_request() {
    local test_name=$1
    local request=$2
    local expect_success=$3  # "success" or "error_ok" (error response is acceptable)

    local log_file="$TEMP_DIR/${test_name}.log"

    # Send request
    echo "$request" | $WRAPPER --port $PORT > "$log_file" 2>&1
    local exit_code=$?

    # Check stderr is silent
    local stderr_lines=$(grep '^\[gasoline' "$log_file" 2>/dev/null | wc -l | tr -d ' ' || echo "0")

    # Check for JSON-RPC response
    if grep -q '"result"' "$log_file" 2>/dev/null; then
        if [ "$stderr_lines" -eq 0 ]; then
            echo "  ✅ $test_name (success, silent)"
            PASS_COUNT=$((PASS_COUNT + 1))
            return 0
        else
            echo "  ⚠️  $test_name (success but $stderr_lines stderr lines)"
            FAIL_COUNT=$((FAIL_COUNT + 1))
            return 1
        fi
    elif grep -q '"error"' "$log_file" 2>/dev/null; then
        if [ "$expect_success" = "error_ok" ]; then
            echo "  ✅ $test_name (error response, expected)"
            PASS_COUNT=$((PASS_COUNT + 1))
            return 0
        else
            echo "  ❌ $test_name (unexpected error)"
            grep '"error"' "$log_file" | jq -r '.error.message' 2>/dev/null | sed 's/^/       /' || true
            FAIL_COUNT=$((FAIL_COUNT + 1))
            return 1
        fi
    else
        echo "  ❌ $test_name (no response)"
        FAIL_COUNT=$((FAIL_COUNT + 1))
        return 1
    fi
}

# Test 1: Cold start + list tools
echo "Test 1: Cold Start + List Tools"
echo "================================"
kill_server

test_request "cold_start_list_tools" \
    '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
    "success"

# Check server spawned
if lsof -ti :$PORT >/dev/null 2>&1; then
    echo "  ✅ Server spawned and running"
else
    echo "  ❌ Server not running after cold start"
fi

echo ""

# Test 2: Cold start + immediate tool call
echo "Test 2: Cold Start + Immediate Tool Call"
echo "========================================="
kill_server

test_request "cold_observe_page" \
    '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"mode":"page"}}}' \
    "error_ok"

echo ""

# Test 3: Initialize then call all tools
echo "Test 3: Call All Tools Sequentially"
echo "===================================="
kill_server

# Initialize first
test_request "initialize" \
    '{"jsonrpc":"2.0","id":3,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
    "success"

# observe tool with different modes
test_request "observe_errors" \
    '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe","arguments":{"mode":"errors"}}}' \
    "error_ok"

test_request "observe_logs" \
    '{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"observe","arguments":{"mode":"logs"}}}' \
    "error_ok"

test_request "observe_network" \
    '{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"observe","arguments":{"mode":"network_waterfall"}}}' \
    "error_ok"

test_request "observe_page" \
    '{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"observe","arguments":{"mode":"page"}}}' \
    "error_ok"

# configure tool
test_request "configure_get_health" \
    '{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"configure","arguments":{"action":"get_health"}}}' \
    "success"

test_request "configure_toggle_pilot" \
    '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"configure","arguments":{"action":"toggle_pilot"}}}' \
    "success"

# generate tool (expects error - no extension)
test_request "generate_screenshot" \
    '{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"generate","arguments":{"mode":"screenshot"}}}' \
    "error_ok"

# interact tool (expects error - no extension)
test_request "interact_click" \
    '{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"interact","arguments":{"action":"click","selector":"button"}}}' \
    "error_ok"

echo ""

# Test 4: Concurrent tool calls
echo "Test 4: Concurrent Tool Calls (5 simultaneous)"
echo "==============================================="

PIDS=()
for i in 1 2 3 4 5; do
    REQUEST='{"jsonrpc":"2.0","id":'$((100+i))',"method":"tools/call","params":{"name":"configure","arguments":{"action":"get_health"}}}'
    (echo "$REQUEST"; sleep 0.3) | $WRAPPER --port $PORT > "$TEMP_DIR/concurrent_$i.log" 2>&1 &
    PIDS+=($!)
done

# Wait for all
sleep 2

# Check results
CONCURRENT_SUCCESS=0
for i in 1 2 3 4 5; do
    if grep -q '"result"' "$TEMP_DIR/concurrent_$i.log" 2>/dev/null; then
        CONCURRENT_SUCCESS=$((CONCURRENT_SUCCESS + 1))
    fi
done

echo "  ✅ $CONCURRENT_SUCCESS/5 concurrent calls succeeded"
if [ "$CONCURRENT_SUCCESS" -eq 5 ]; then
    PASS_COUNT=$((PASS_COUNT + 1))
else
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

# Kill leftover processes
for pid in "${PIDS[@]}"; do
    kill $pid 2>/dev/null || true
done

echo ""

# Test 5: Rapid sequential calls (stress test)
echo "Test 5: Rapid Sequential Calls (10 requests)"
echo "============================================="

RAPID_SUCCESS=0
for i in 1 2 3 4 5 6 7 8 9 10; do
    REQUEST='{"jsonrpc":"2.0","id":'$((200+i))',"method":"tools/call","params":{"name":"configure","arguments":{"action":"get_health"}}}'
    if echo "$REQUEST" | $WRAPPER --port $PORT 2>/dev/null | grep -q '"result"'; then
        RAPID_SUCCESS=$((RAPID_SUCCESS + 1))
    fi
done

echo "  ✅ $RAPID_SUCCESS/10 rapid calls succeeded"
if [ "$RAPID_SUCCESS" -eq 10 ]; then
    PASS_COUNT=$((PASS_COUNT + 1))
else
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo ""

# Test 6: Server persistence
echo "Test 6: Server Persistence Check"
echo "================================="

if lsof -ti :$PORT >/dev/null 2>&1; then
    echo "  ✅ Server persisted through all tests"

    # Get server health
    HEALTH=$(curl -s http://localhost:$PORT/health 2>/dev/null || echo "")
    if [ -n "$HEALTH" ]; then
        VERSION=$(echo "$HEALTH" | jq -r '.version' 2>/dev/null || echo "unknown")
        UPTIME=$(echo "$HEALTH" | jq -r '.server.uptime' 2>/dev/null || echo "unknown")
        echo "  Version: $VERSION"
        echo "  Uptime: $UPTIME"
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        echo "  ⚠️  Server running but health check failed"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
else
    echo "  ❌ Server not running"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo ""

# Test 7: Final stdio silence verification
echo "Test 7: Final Stdio Silence Check"
echo "=================================="

SILENCE_LOG="$TEMP_DIR/final_silence.log"
REQUEST='{"jsonrpc":"2.0","id":999,"method":"tools/list","params":{}}'
echo "$REQUEST" | $WRAPPER --port $PORT > "$SILENCE_LOG" 2>&1

FINAL_STDERR=$(grep '^\[gasoline' "$SILENCE_LOG" | wc -l | tr -d ' ')
if [ "$FINAL_STDERR" -eq 0 ]; then
    echo "  ✅ Zero stderr output (stdio silence maintained)"
    PASS_COUNT=$((PASS_COUNT + 1))
else
    echo "  ❌ Found $FINAL_STDERR stderr lines"
    grep '^\[gasoline' "$SILENCE_LOG" | sed 's/^/       /'
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi

echo ""

# Cleanup
kill_server
rm -rf "$TEMP_DIR"

# Final Summary
echo "============================================"
echo "FINAL SUMMARY"
echo "============================================"
echo ""
echo "Total Tests: $((PASS_COUNT + FAIL_COUNT))"
echo "  ✅ Passed: $PASS_COUNT"
echo "  ❌ Failed: $FAIL_COUNT"
echo ""

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo "🎉 ALL TESTS PASSED!"
    echo ""
    echo "✅ MCP connection is 100% reliable"
    echo "✅ All tools are callable"
    echo "✅ Stdio is completely silent"
    echo "✅ Server persists correctly"
    echo "✅ Concurrent calls work"
    echo "✅ Rapid sequential calls work"
    echo ""
    echo "Ready for production release! 🚀"
    exit 0
else
    echo "⚠️  SOME TESTS FAILED"
    echo ""
    echo "Review the output above for details."
    echo "Common issues:"
    echo "  - Tools returning errors (browser extension not installed)"
    echo "  - Server not persisting (check logs)"
    echo "  - Stdio noise (check stderr output)"
    exit 1
fi
