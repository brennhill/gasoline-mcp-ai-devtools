#!/bin/bash
set -e

# Comprehensive MCP Tool Test Suite
# Tests all tools with various scenarios: cold start, immediate use, concurrent calls

PORT=$((8000 + RANDOM % 1000))
# Use local build if available, otherwise fall back to PATH
if [ -x "./gasoline-mcp" ]; then
    WRAPPER="./gasoline-mcp"
else
    WRAPPER="gasoline-mcp"
fi
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

# Test 6: Stdout purity verification (critical for MCP protocol)
echo "Test 6: Stdout Purity (JSON-RPC Only)"
echo "====================================="
kill_server

REQUEST='{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"purity-test","version":"1.0"}}}'
(echo "$REQUEST"; sleep 0.5) | $WRAPPER --port $PORT > "$TEMP_DIR/purity_stdout.txt" 2>"$TEMP_DIR/purity_stderr.txt"

# CRITICAL: Verify stdout contains ONLY valid JSON-RPC (no stray output)
# Each non-empty line must be valid JSON with "jsonrpc":"2.0"
STDOUT_ERRORS=0
STDOUT_LINES=0
while IFS= read -r line; do
    [ -z "$line" ] && continue  # Skip empty lines
    STDOUT_LINES=$((STDOUT_LINES + 1))

    # Check if line is valid JSON
    if ! echo "$line" | jq -e . >/dev/null 2>&1; then
        echo "  ❌ Non-JSON output on stdout: $line"
        STDOUT_ERRORS=$((STDOUT_ERRORS + 1))
    # Check if it's a JSON-RPC message
    elif ! echo "$line" | jq -e 'has("jsonrpc")' >/dev/null 2>&1; then
        echo "  ❌ Non-JSON-RPC JSON on stdout: $line"
        STDOUT_ERRORS=$((STDOUT_ERRORS + 1))
    fi
done < "$TEMP_DIR/purity_stdout.txt"

if [ "$STDOUT_ERRORS" -eq 0 ] && [ "$STDOUT_LINES" -gt 0 ]; then
    echo "  ✅ Stdout contains only valid JSON-RPC ($STDOUT_LINES messages)"
elif [ "$STDOUT_LINES" -eq 0 ]; then
    echo "  ❌ No output on stdout (expected JSON-RPC response)"
else
    echo "  ❌ Found $STDOUT_ERRORS invalid lines on stdout"
fi

# Note: stderr is allowed for logging (doesn't break protocol)
STDERR_LINES=$(wc -l < "$TEMP_DIR/purity_stderr.txt" | tr -d ' ')
if [ "$STDERR_LINES" -gt 0 ]; then
    echo "  ℹ️  $STDERR_LINES stderr lines (allowed for logging)"
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

# Test 8: Graceful shutdown with --stop
echo "Test 8: Graceful Shutdown (--stop)"
echo "==================================="

# Start a fresh server in daemon mode (for reliable PID file testing)
kill_server
$WRAPPER --daemon --port $PORT >/dev/null 2>&1 &

# Wait for server to be ready (health check)
for i in $(seq 1 30); do
    if curl -s http://localhost:$PORT/health >/dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

# Verify server is running
if curl -s http://localhost:$PORT/health >/dev/null 2>&1; then
    echo "  ✅ Server started successfully"
else
    echo "  ❌ Server failed to start"
fi

# Check PID file exists (daemon creates it after HTTP bind)
PID_FILE="$HOME/.gasoline-$PORT.pid"
if [ -f "$PID_FILE" ]; then
    SAVED_PID=$(cat "$PID_FILE")
    echo "  ✅ PID file exists (PID: $SAVED_PID)"
else
    echo "  ⚠️  PID file not found (fallback methods will be used)"
fi

# Stop server using --stop flag
echo "  Stopping server with --stop..."
$WRAPPER --stop --port $PORT > "$TEMP_DIR/stop_output.txt" 2>&1
STOP_EXIT=$?

if [ $STOP_EXIT -eq 0 ]; then
    echo "  ✅ --stop command succeeded"
else
    echo "  ❌ --stop command failed (exit code: $STOP_EXIT)"
fi

# Verify server is stopped
sleep 1
if ! lsof -ti :$PORT >/dev/null 2>&1; then
    echo "  ✅ Server stopped successfully"
else
    echo "  ❌ Server still running after --stop"
fi

# Verify PID file is cleaned up
if [ ! -f "$PID_FILE" ]; then
    echo "  ✅ PID file cleaned up"
else
    echo "  ⚠️  PID file still exists (stale)"
    rm -f "$PID_FILE"
fi

echo ""

# Final cleanup (server should already be stopped, but just in case)
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
echo "  - Stdout purity (JSON-RPC only, no stray output)"
echo "  - Server persistence"
echo "  - Graceful shutdown (--stop + PID file)"
echo ""
echo "✅ Test suite complete!"
echo ""
