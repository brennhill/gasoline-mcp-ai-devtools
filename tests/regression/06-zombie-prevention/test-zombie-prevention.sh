#!/usr/bin/env bash
set -euo pipefail

# Test zombie prevention and fast-fail behaviors

BINARY="${GASOLINE_BINARY:-dist/gasoline-darwin-arm64}"
TEST_PORT=17899

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }
info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# Cleanup function
cleanup() {
  pkill -9 -f "gasoline.*$TEST_PORT" 2>/dev/null || true
  rm -f ~/.gasoline-$TEST_PORT.pid
}

# Ensure clean state
cleanup

echo "╔══════════════════════════════════════════╗"
echo "║  Zombie Prevention & Fast-Fail Tests     ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# ============================================================================
# Test 1: Stale PID File Cleanup
# ============================================================================
info "Test 1: Stale PID file is removed and server starts"

# Create a stale PID file with non-existent PID
echo "99999" > ~/.gasoline-$TEST_PORT.pid

# Try to start server - should remove stale PID and start successfully
"$BINARY" --daemon --port "$TEST_PORT" >/dev/null 2>&1 &
SERVER_PID=$!

sleep 2

# Check if server is running
if curl -s "http://localhost:$TEST_PORT/health" >/dev/null 2>&1; then
  pass "Test 1: Stale PID file cleaned up, server started"
else
  fail "Test 1: Server failed to start after removing stale PID"
fi

cleanup
sleep 1

# ============================================================================
# Test 2: Port Conflict Fast-Fail
# ============================================================================
info "Test 2: Port conflict fails fast (< 4 seconds)"

# Start first server (must background with &)
"$BINARY" --daemon --port "$TEST_PORT" >/dev/null 2>&1 &
sleep 2

# Try to start second server - should fail fast
START_TIME=$(date +%s)
"$BINARY" --daemon --port "$TEST_PORT" >/tmp/port-conflict.log 2>&1 && exit_code=0 || exit_code=$?
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

if [ "$exit_code" -ne 0 ] && [ "$ELAPSED" -lt 4 ]; then
  pass "Test 2: Port conflict failed fast in ${ELAPSED}s (< 4s)"
else
  fail "Test 2: Port conflict took ${ELAPSED}s (expected < 4s)"
fi

# Check error message is helpful
if grep -q "already in use" /tmp/port-conflict.log; then
  pass "Test 2: Error message mentions 'already in use'"
else
  fail "Test 2: Error message not helpful"
fi

cleanup
sleep 1

# ============================================================================
# Test 3: PID File Prevents Duplicate
# ============================================================================
info "Test 3: Live PID file prevents duplicate server"

# Start server
"$BINARY" --daemon --port "$TEST_PORT" >/dev/null 2>&1 &
sleep 2

# Verify PID file exists
if [ -f ~/.gasoline-$TEST_PORT.pid ]; then
  pass "Test 3: PID file created"
else
  fail "Test 3: PID file not created"
fi

# Try to start duplicate - should fail immediately
"$BINARY" --daemon --port "$TEST_PORT" >/tmp/duplicate.log 2>&1 && exit_code=0 || exit_code=$?

if [ "$exit_code" -ne 0 ]; then
  pass "Test 3: Duplicate server prevented"
else
  fail "Test 3: Duplicate server not prevented"
fi

cleanup
sleep 1

# ============================================================================
# Test 4: MCP Client Cold Start (spawns server automatically)
# ============================================================================
info "Test 4: MCP client spawns server and connects fast (< 6 seconds)"

# No server running - MCP client should spawn one and connect
START_TIME=$(date +%s)
(
  echo '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
  sleep 2
) | "$BINARY" --port "$TEST_PORT" >/tmp/mcp-spawn.log 2>&1 &
MCP_PID=$!
sleep 4
kill $MCP_PID 2>/dev/null || true
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

# Check if server was spawned and is healthy
if curl -s "http://localhost:$TEST_PORT/health" | grep -q '"status":"ok"'; then
  pass "Test 4: MCP client spawned server in ${ELAPSED}s"
else
  fail "Test 4: MCP client failed to spawn server"
fi

cleanup
sleep 1

# ============================================================================
# Test 5: --stop Cleans Up PID File
# ============================================================================
info "Test 5: --stop removes PID file"

# Start server (must background with &)
"$BINARY" --daemon --port "$TEST_PORT" >/dev/null 2>&1 &
sleep 2

# Stop server
"$BINARY" --stop --port "$TEST_PORT" >/dev/null 2>&1
sleep 1

# Verify PID file removed
if [ ! -f ~/.gasoline-$TEST_PORT.pid ]; then
  pass "Test 5: PID file removed after --stop"
else
  fail "Test 5: PID file not removed"
fi

cleanup

# ============================================================================
# Test 6: Zombie Process Doesn't Block New Server
# ============================================================================
info "Test 6: After killing zombie, new server can start"

# Start server and get its PID (must background with &)
"$BINARY" --daemon --port "$TEST_PORT" >/dev/null 2>&1 &
sleep 2
SERVER_PID=$(lsof -ti :"$TEST_PORT")

# Kill server WITHOUT letting it clean up (simulating crash)
kill -9 "$SERVER_PID"
sleep 1

# PID file still exists (zombie scenario)
if [ -f ~/.gasoline-$TEST_PORT.pid ]; then
  pass "Test 6: PID file still exists after kill -9"
else
  info "Test 6: PID file was removed (unexpected but ok)"
fi

# Try to start new server - should detect dead process and start
"$BINARY" --daemon --port "$TEST_PORT" >/dev/null 2>&1 &
sleep 2

if curl -s "http://localhost:$TEST_PORT/health" >/dev/null 2>&1; then
  pass "Test 6: New server started after zombie cleanup"
else
  fail "Test 6: New server failed to start"
fi

cleanup

echo ""
echo "══════════════════════════════════════════"
echo "All zombie prevention tests complete!"
echo "══════════════════════════════════════════"
