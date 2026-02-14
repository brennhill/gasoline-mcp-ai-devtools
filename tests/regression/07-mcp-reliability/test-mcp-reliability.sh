#!/bin/bash
# MCP Connection Reliability Tests
# Verifies server persistence and recovery across MCP client connections

set -e

PORT=17899
BINARY="${GASOLINE_BINARY:-./dist/gasoline}"
PID_FILE="$HOME/.gasoline-$PORT.pid"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

cleanup() {
    pkill -9 -f "gasoline.*$PORT" 2>/dev/null || true
    rm -f "$PID_FILE"
    sleep 1
}

# Simulate MCP client: send initialize, get response
# Important: Keep stdin open long enough for full connection setup to avoid context cancellation
mcp_connect() {
    local timeout_sec=${1:-10}
    local tmp_out
    tmp_out="$(mktemp)"

    # Run in background - keep stdin open for 5s to allow full connection setup
    (
        echo '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
        sleep 5  # Keep stdin open - bridge shuts down when stdin closes
    ) | "$BINARY" --port "$PORT" > "$tmp_out" 2>/dev/null || true &
    local mcp_pid=$!

    # Wait for response (but don't wait for stdin to close)
    local count=0
    while [ "$count" -lt "$timeout_sec" ]; do
        if grep -q '"protocolVersion"' "$tmp_out" 2>/dev/null; then
            # Response received - wait a bit for connection to stabilize
            sleep 1
            kill "$mcp_pid" 2>/dev/null || true
            wait "$mcp_pid" 2>/dev/null || true
            cat "$tmp_out"
            rm -f "$tmp_out"
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done

    kill "$mcp_pid" 2>/dev/null || true
    wait "$mcp_pid" 2>/dev/null || true
    cat "$tmp_out"
    rm -f "$tmp_out"
    return 1
}

get_server_pid() {
    cat "$PID_FILE" 2>/dev/null || echo ""
}

is_server_healthy() {
    curl -s "http://localhost:$PORT/health" 2>/dev/null | grep -q '"status":"ok"'
}

echo "╔══════════════════════════════════════════╗"
echo "║     MCP Connection Reliability Tests     ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# Test 1: Cold Start
info "Test 1: Cold Start - Server spawns on first MCP connection"
cleanup

set +e
RESPONSE=$(mcp_connect 5)
set -e
if echo "$RESPONSE" | grep -q '"protocolVersion"'; then
    # With fast-start, daemon spawns asynchronously - wait for it to be ready
    for i in 1 2 3 4 5 6 7 8 9 10; do
        if is_server_healthy; then
            break
        fi
        sleep 0.5
    done
    if is_server_healthy; then
        PID1=$(get_server_pid)
        pass "Cold start successful (server PID: $PID1)"
    else
        fail "Initialize succeeded but server not healthy after 5s"
    fi
else
    fail "No initialize response"
fi

# Test 2: Warm Reconnect
info "Test 2: Warm Reconnect - New client connects to existing server"

# Server should still be running from Test 1
if ! is_server_healthy; then
    fail "Server died after first connection"
fi

# Connect again
set +e
RESPONSE=$(mcp_connect 5)
set -e
if echo "$RESPONSE" | grep -q '"protocolVersion"'; then
    PID2=$(get_server_pid)
    if [ "$PID1" = "$PID2" ]; then
        pass "Warm reconnect successful (same server PID: $PID2)"
    else
        fail "Server restarted unexpectedly (was $PID1, now $PID2)"
    fi
else
    fail "No initialize response on warm reconnect"
fi

# Test 3: Server Survives Rapid Connections
info "Test 3: Server survives 5 rapid MCP connections"

set +e
for i in 1 2 3 4 5; do
    RESPONSE=$(mcp_connect 3)
    if ! echo "$RESPONSE" | grep -q '"protocolVersion"'; then
        set -e
        fail "Connection $i failed"
    fi
done
set -e

PID3=$(get_server_pid)
if [ "$PID1" = "$PID3" ]; then
    pass "Server survived 5 rapid connections (PID unchanged: $PID3)"
else
    fail "Server restarted during rapid connections (was $PID1, now $PID3)"
fi

# Test 4: Recovery from killed server
info "Test 4: Recovery from killed server"

# Kill the server
pkill -9 -f "gasoline.*--daemon.*$PORT" 2>/dev/null || true
rm -f "$PID_FILE"
sleep 1

if is_server_healthy; then
    fail "Server should be dead after kill"
fi

# New connection should spawn fresh server
set +e
RESPONSE=$(mcp_connect 5)
set -e
if echo "$RESPONSE" | grep -q '"protocolVersion"'; then
    PID4=$(get_server_pid)
    if [ -n "$PID4" ] && [ "$PID4" != "$PID1" ]; then
        pass "Recovery successful (new server PID: $PID4)"
    else
        fail "Server didn't respawn properly"
    fi
else
    fail "Failed to recover from killed server"
fi

# Test 5: Fast-start responds immediately even when port blocked
# With fast-start mode, initialize/tools/list respond immediately from bridge
# without needing the daemon. Only tools/call checks daemon availability.
info "Test 5: Fast fail when port blocked by external process"

# Kill our server
pkill -9 -f "gasoline.*$PORT" 2>/dev/null || true
rm -f "$PID_FILE"
sleep 1

# Start a simple HTTP server to block the port
python3 -m http.server "$PORT" --bind 127.0.0.1 >/dev/null 2>&1 &
BLOCKER_PID=$!
sleep 1

# Verify blocker is running
if ! lsof -ti :"$PORT" >/dev/null 2>&1; then
    kill "$BLOCKER_PID" 2>/dev/null || true
    fail "Could not start port blocker"
fi

# With fast-start, initialize should succeed immediately even with port blocked
# The daemon spawn will fail, but we still get the fast-start response
tmp_out=$(mktemp)
START=$(date +%s)
(echo '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'; sleep 5) | \
    "$BINARY" --port "$PORT" > "$tmp_out" 2>/dev/null &
BIN_PID=$!

# Wait for response to appear in file
for i in 1 2 3 4 5 6 7 8 9 10; do
    if [ -s "$tmp_out" ] && grep -q '"protocolVersion"' "$tmp_out" 2>/dev/null; then
        break
    fi
    sleep 0.5
done
END=$(date +%s)
ELAPSED=$((END - START))

# Check we got initialize response quickly
if grep -q '"protocolVersion"' "$tmp_out"; then
    pass "Fast fail on blocked port (${ELAPSED}s < 5s)"
else
    cat "$tmp_out"  # Debug: show what we got
    fail "Did not get initialize response (expected fast-start behavior)"
fi

# Clean up
kill "$BIN_PID" 2>/dev/null || true
kill "$BLOCKER_PID" 2>/dev/null || true
rm -f "$tmp_out"

# Final cleanup
cleanup

echo ""
echo "════════════════════════════════════════════"
echo "  All MCP reliability tests passed!"
echo "════════════════════════════════════════════"
