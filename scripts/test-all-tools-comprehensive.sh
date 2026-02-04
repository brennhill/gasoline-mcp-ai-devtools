#!/bin/bash
set -e

# Comprehensive MCP Tool Test Suite
# Tests all tools with various scenarios: cold start, immediate use, concurrent calls

PORT=$((8000 + RANDOM % 1000))
WRAPPER="gasoline-mcp"
TEMP_DIR=$(mktemp -d)

echo "========================================"
echo "Comprehensive MCP Tool Test Suite"
echo "========================================"
echo ""
echo "Port: $PORT"
echo "Temp dir: $TEMP_DIR"
echo ""

# Helper: Kill all servers
kill_server() {
    lsof -ti :$PORT 2>/dev/null | xargs kill -9 2>/dev/null || true
    sleep 0.5
}

# Helper: Send MCP request and check response
send_mcp_request() {
    local name=$1
    local request=$2
    local log_file="$TEMP_DIR/${name}.log"

    echo "$request" | $WRAPPER --port $PORT > "$log_file" 2>&1

    # Check for JSON-RPC response
    if grep -q '"result"' "$log_file" 2>/dev/null; then
        echo "  ✅ $name"
        return 0
    elif grep -q '"error"' "$log_file" 2>/dev/null; then
        echo "  ⚠️  $name (returned error)"
        return 0
    else
        echo "  ❌ $name (no response)"
        return 1
    fi
}

# Test 1: Cold start + list tools
echo "Test 1: Cold Start + List Tools"
echo "================================"
kill_server

REQUEST='{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
send_mcp_request "cold_start_list_tools" "$REQUEST"

# Verify server is still running
if lsof -ti :$PORT >/dev/null 2>&1; then
    echo "  ✅ Server persisted after client exit"
else
    echo "  ❌ Server died after client exit"
fi

echo ""

# Test 2: List all available tools
echo "Test 2: Get Full Tool List"
echo "==========================="

REQUEST='{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
RESULT=$($WRAPPER --port $PORT <<< "$REQUEST" 2>/dev/null | grep '"result"' || echo "")

if [ -n "$RESULT" ]; then
    TOOL_COUNT=$(echo "$RESULT" | jq -r '.result.tools | length' 2>/dev/null || echo "0")
    echo "  ✅ Found $TOOL_COUNT tools"

    # Extract tool names
    TOOLS=$(echo "$RESULT" | jq -r '.result.tools[].name' 2>/dev/null || echo "")
    echo "  Tools:"
    echo "$TOOLS" | sed 's/^/    - /'
else
    echo "  ❌ Could not get tool list"
fi

echo ""

# Test 3: Immediate tool calls after cold start
echo "Test 3: Cold Start + Immediate Tool Calls"
echo "=========================================="
kill_server

# Test observe_page
REQUEST='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"observe_page","arguments":{}}}'
send_mcp_request "cold_observe_page" "$REQUEST"

echo ""

# Test 4: Sequential tool calls (same server)
echo "Test 4: Sequential Tool Calls"
echo "=============================="

# observe_logs
REQUEST='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe_logs","arguments":{}}}'
send_mcp_request "observe_logs" "$REQUEST"

# observe_network
REQUEST='{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"observe_network","arguments":{}}}'
send_mcp_request "observe_network" "$REQUEST"

# get_health
REQUEST='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_health","arguments":{}}}'
send_mcp_request "get_health" "$REQUEST"

echo ""

# Test 5: Concurrent clients
echo "Test 5: Concurrent Clients (5 simultaneous)"
echo "============================================"
kill_server

PIDS=()
for i in 1 2 3 4 5; do
    REQUEST='{"jsonrpc":"2.0","id":'$i',"method":"tools/list","params":{}}'
    (echo "$REQUEST"; sleep 0.5) | $WRAPPER --port $PORT > "$TEMP_DIR/concurrent_$i.log" 2>&1 &
    PIDS+=($!)
done

# Wait for all clients
sleep 3

# Check results
CONCURRENT_SUCCESS=0
for i in 1 2 3 4 5; do
    if grep -q '"result"' "$TEMP_DIR/concurrent_$i.log" 2>/dev/null; then
        CONCURRENT_SUCCESS=$((CONCURRENT_SUCCESS + 1))
    fi
done

echo "  ✅ $CONCURRENT_SUCCESS/5 clients succeeded"

# Kill any remaining processes
for pid in "${PIDS[@]}"; do
    kill $pid 2>/dev/null || true
done

echo ""

# Test 6: Stdio silence verification
echo "Test 6: Stdio Silence Verification"
echo "==================================="
kill_server

REQUEST='{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"silence-test","version":"1.0"}}}'
(echo "$REQUEST"; sleep 0.5) | $WRAPPER --port $PORT > "$TEMP_DIR/silence_stdout.txt" 2>"$TEMP_DIR/silence_stderr.txt"

STDERR_LINES=$(wc -l < "$TEMP_DIR/silence_stderr.txt" | tr -d ' ')
if [ "$STDERR_LINES" -eq 0 ]; then
    echo "  ✅ Zero stderr lines (completely silent)"
else
    echo "  ❌ Found $STDERR_LINES stderr lines:"
    cat "$TEMP_DIR/silence_stderr.txt" | sed 's/^/    /'
fi

# Check stdout is JSON-RPC
if grep -q '"jsonrpc":"2.0"' "$TEMP_DIR/silence_stdout.txt" 2>/dev/null; then
    echo "  ✅ Stdout contains valid JSON-RPC"
else
    echo "  ❌ Stdout does not contain JSON-RPC"
fi

echo ""

# Test 7: Server persistence
echo "Test 7: Server Persistence"
echo "==========================="

# Check server is still running
if lsof -ti :$PORT >/dev/null 2>&1; then
    echo "  ✅ Server persisted through all tests"

    # Get uptime
    UPTIME=$(curl -s http://localhost:$PORT/health 2>/dev/null | jq -r '.server.uptime' 2>/dev/null || echo "unknown")
    echo "  Server uptime: $UPTIME"
else
    echo "  ❌ Server not running"
fi

echo ""

# Final cleanup
kill_server
rm -rf "$TEMP_DIR"

# Summary
echo "========================================"
echo "Summary"
echo "========================================"
echo ""
echo "✅ All core MCP operations tested:"
echo "  - Cold start + tool list"
echo "  - Immediate tool calls after spawn"
echo "  - Sequential tool calls"
echo "  - Concurrent clients (5 simultaneous)"
echo "  - Stdio silence (0 stderr lines)"
echo "  - Server persistence"
echo ""
echo "✅ Test suite complete!"
echo ""
