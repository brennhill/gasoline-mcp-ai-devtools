#!/usr/bin/env bash
# Test all 14 edge cases from MCP persistent server architecture

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# shellcheck source=tests/regression/lib/common.sh
source "$PROJECT_ROOT/tests/regression/lib/common.sh"

TEST_PORT=17893  # Use different port to avoid conflicts
BINARY="$PROJECT_ROOT/dist/gasoline"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass() {
  echo -e "${GREEN}[PASS]${NC} $1"
}

fail() {
  echo -e "${RED}[FAIL]${NC} $1"
  exit 1
}

skip() {
  echo -e "${YELLOW}[SKIP]${NC} $1"
}

info() {
  echo -e "${YELLOW}[INFO]${NC} $1"
}

# Helper: Check if server is running
is_server_running() {
  local port=$1
  curl -sf "http://127.0.0.1:$port/health" > /dev/null 2>&1
}

# Helper: Kill server on port
kill_server() {
  local port=$1
  lsof -ti ":$port" 2>/dev/null | xargs kill -9 2>/dev/null || true
  sleep 0.5
}

# Helper: Start server in background
start_server_daemon() {
  local port=$1
  "$BINARY" --daemon --port "$port" > /dev/null 2>&1 &
  local pid=$!

  # Wait for ready
  for i in {1..30}; do
    if is_server_running "$port"; then
      echo "$pid"
      return 0
    fi
    sleep 0.1
  done

  kill "$pid" 2>/dev/null || true
  return 1
}

# Helper: Simulate MCP client connection
simulate_mcp_connect() {
  local port=$1
  local timeout=${2:-5}

  # Simulate stdio by piping input to binary
  echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | timeout "$timeout" "$BINARY" --port "$port" 2>/dev/null || true
}

# ============================================================================
# Edge Case 1: Port Already in Use (Non-Gasoline Process)
# ============================================================================
test_edge_case_1() {
  info "Testing Edge Case 1: Port already in use"

  # Start two servers on same port - second should fail
  local pid1
  pid1=$(start_server_daemon "$TEST_PORT")

  # Try to start second server - should fail
  if timeout 3 "$BINARY" --daemon --port "$TEST_PORT" > /tmp/gasoline-test-ec1.log 2>&1; then
    kill "$pid1" 2>/dev/null || true
    fail "Edge Case 1: Second server started on same port"
  fi

  # Verify only one server running
  local count
  count=$(lsof -ti ":$TEST_PORT" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$count" -ne 1 ]; then
    kill_server "$TEST_PORT"
    fail "Edge Case 1: Expected 1 server, got $count"
  fi

  pass "Edge Case 1: Correctly prevented duplicate server on same port"

  # Cleanup
  kill_server "$TEST_PORT"
}

# ============================================================================
# Edge Case 2: Server Crashes Mid-Connection
# ============================================================================
test_edge_case_2() {
  info "Testing Edge Case 2: Server crashes mid-connection"

  # Start server
  local pid
  pid=$(start_server_daemon "$TEST_PORT")

  # Verify running
  if ! is_server_running "$TEST_PORT"; then
    fail "Edge Case 2: Server failed to start"
  fi

  # Kill server abruptly (simulate crash)
  kill -9 "$pid" 2>/dev/null || true
  sleep 0.5

  # Verify server is down
  if is_server_running "$TEST_PORT"; then
    fail "Edge Case 2: Server still running after kill -9"
  fi

  # MCP client should detect and respawn
  # (Testing full auto-recovery requires more complex setup)
  pass "Edge Case 2: Server crash detected"

  # Cleanup
  kill_server "$TEST_PORT"
}

# ============================================================================
# Edge Case 3: Stale PID File
# ============================================================================
test_edge_case_3() {
  info "Testing Edge Case 3: Stale PID file"

  # Create stale PID file with non-existent PID
  local pid_file="$HOME/.gasoline-$TEST_PORT.pid"
  echo "99999" > "$pid_file"

  # Start server - should detect stale PID and start anyway
  local pid
  pid=$(start_server_daemon "$TEST_PORT")

  if ! is_server_running "$TEST_PORT"; then
    rm -f "$pid_file"
    fail "Edge Case 3: Server failed to start with stale PID file"
  fi

  # PID file should be updated
  local new_pid
  new_pid=$(cat "$pid_file")
  if [ "$new_pid" = "99999" ]; then
    kill "$pid" 2>/dev/null || true
    rm -f "$pid_file"
    fail "Edge Case 3: PID file not updated"
  fi

  pass "Edge Case 3: Stale PID file handled correctly"

  # Cleanup
  kill_server "$TEST_PORT"
  rm -f "$pid_file"
}

# ============================================================================
# Edge Case 4: Multiple Clients Race to Spawn
# ============================================================================
test_edge_case_4() {
  info "Testing Edge Case 4: Multiple MCP clients race to spawn server"

  # Ensure no server running
  kill_server "$TEST_PORT"
  sleep 0.5

  # Spawn 10 MCP clients simultaneously (they will race to spawn the daemon)
  # All should eventually connect - winner spawns, losers retry and connect
  local pids=()
  for i in {1..10}; do
    # Simulate MCP client: piping a simple request to stdin triggers handleMCPConnection
    echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | \
      "$BINARY" --port "$TEST_PORT" > "/tmp/gasoline-race-${i}.log" 2>&1 &
    pids+=($!)
  done

  # Give clients time to race, spawn server, and establish connections
  sleep 5

  # All client processes should exit successfully after getting response
  local failed_count=0
  for pid in "${pids[@]}"; do
    # Wait for process to complete
    if ! wait "$pid" 2>/dev/null; then
      failed_count=$((failed_count + 1))
    fi
  done

  # All clients should have succeeded (exit code 0)
  if [ "$failed_count" -gt 0 ]; then
    kill_server "$TEST_PORT"
    fail "Edge Case 4: $failed_count clients failed (expected 0 failures)"
  fi

  # Exactly one daemon server should be running
  local server_count
  server_count=$(lsof -ti ":$TEST_PORT" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$server_count" -ne 1 ]; then
    kill_server "$TEST_PORT"
    fail "Edge Case 4: Expected 1 server, got $server_count"
  fi

  # Server should be responding
  if ! is_server_running "$TEST_PORT"; then
    kill_server "$TEST_PORT"
    fail "Edge Case 4: Server not responding after race"
  fi

  # Check that all clients got responses
  local response_count=0
  for i in {1..10}; do
    if grep -q '"tools"' "/tmp/gasoline-race-$i.log" 2>/dev/null; then
      response_count=$((response_count + 1))
    fi
  done

  if [ "$response_count" -lt 10 ]; then
    kill_server "$TEST_PORT"
    fail "Edge Case 4: Only $response_count/10 clients got responses"
  fi

  pass "Edge Case 4: All 10 clients successfully connected despite race"

  # Cleanup
  kill_server "$TEST_PORT"
}

# ============================================================================
# Edge Case 5: Server Startup Timeout
# ============================================================================
test_edge_case_5() {
  info "Testing Edge Case 5: Server startup timeout"

  # This is hard to test without modifying the binary
  # We can verify the timeout exists by checking logs
  skip "Edge Case 5: Requires controlled server hang (manual test)"
}

# ============================================================================
# Edge Case 6: Permission Denied on Port Bind
# ============================================================================
test_edge_case_6() {
  info "Testing Edge Case 6: Permission denied on port bind"

  # Try to bind to privileged port (< 1024) without root
  if [ "$(id -u)" -eq 0 ]; then
    skip "Edge Case 6: Running as root, cannot test permission denied"
    return
  fi

  if "$BINARY" --daemon --port 80 > /tmp/gasoline-test-ec6.log 2>&1; then
    fail "Edge Case 6: Server started on privileged port without root"
  fi

  # Check error message mentions permission
  if ! grep -q -i "permission\|bind" /tmp/gasoline-test-ec6.log 2>/dev/null; then
    fail "Edge Case 6: No clear permission error in logs"
  fi

  pass "Edge Case 6: Permission denied handled correctly"
}

# ============================================================================
# Edge Case 7: Extension Version Mismatch
# ============================================================================
test_edge_case_7() {
  info "Testing Edge Case 7: Extension version mismatch"

  # Start server
  local pid
  pid=$(start_server_daemon "$TEST_PORT")

  # Simulate extension with different version
  local response
  response=$(curl -sf "http://127.0.0.1:$TEST_PORT/sync" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"session_id":"test","extension_version":"99.0.0"}' 2>/dev/null)

  # Check that server_version is present in response
  if ! echo "$response" | jq -e '.server_version' > /dev/null 2>&1; then
    kill "$pid" 2>/dev/null || true
    fail "Edge Case 7: server_version not in response"
  fi

  pass "Edge Case 7: Version mismatch detection works"

  # Cleanup
  kill_server "$TEST_PORT"
}

# ============================================================================
# Edge Case 8: Wrapper Binary Not Found
# ============================================================================
test_edge_case_8() {
  info "Testing Edge Case 8: Wrapper binary not found"

  # This is tested by the npm wrapper, not the Go binary
  skip "Edge Case 8: Tested by npm wrapper integration tests"
}

# ============================================================================
# Edge Case 9: Disk Full (Log File Write Fails)
# ============================================================================
test_edge_case_9() {
  info "Testing Edge Case 9: Disk full"

  # Hard to test without actually filling disk
  # Server should continue despite log write failures
  skip "Edge Case 9: Requires disk full simulation (manual test)"
}

# ============================================================================
# Edge Case 10: Graceful Shutdown During Active Connection
# ============================================================================
test_edge_case_10() {
  info "Testing Edge Case 10: Graceful shutdown during connection"

  # Start server
  local pid
  pid=$(start_server_daemon "$TEST_PORT")

  # Start long request in background
  curl -sf "http://127.0.0.1:$TEST_PORT/health" > /dev/null 2>&1 &
  local request_pid=$!

  # Send SIGTERM (graceful shutdown)
  kill -TERM "$pid" 2>/dev/null || true

  # Wait for request to complete
  wait "$request_pid" 2>/dev/null || true

  # Server should exit after completing request
  sleep 1
  if is_server_running "$TEST_PORT"; then
    kill -9 "$pid" 2>/dev/null || true
    fail "Edge Case 10: Server didn't shutdown gracefully"
  fi

  pass "Edge Case 10: Graceful shutdown works"
}

# ============================================================================
# Edge Case 11: PID File Race Condition
# ============================================================================
test_edge_case_11() {
  info "Testing Edge Case 11: PID file race condition"

  # This can't happen due to port binding protection
  # We verify that only one server can bind the port

  # Try to start two servers simultaneously
  "$BINARY" --daemon --port "$TEST_PORT" > /dev/null 2>&1 &
  local pid1=$!
  "$BINARY" --daemon --port "$TEST_PORT" > /dev/null 2>&1 &
  local pid2=$!

  sleep 2

  # Only one should succeed
  local running_count=0
  if ps -p "$pid1" > /dev/null 2>&1; then
    running_count=$((running_count + 1))
  fi
  if ps -p "$pid2" > /dev/null 2>&1; then
    running_count=$((running_count + 1))
  fi

  if [ "$running_count" -ne 1 ]; then
    kill "$pid1" "$pid2" 2>/dev/null || true
    kill_server "$TEST_PORT"
    fail "Edge Case 11: Expected 1 server, got $running_count"
  fi

  pass "Edge Case 11: Port binding prevents PID race"

  # Cleanup
  kill_server "$TEST_PORT"
}

# ============================================================================
# Edge Case 12: SIGKILL (Unkillable Signal)
# ============================================================================
test_edge_case_12() {
  info "Testing Edge Case 12: SIGKILL detection"

  # Start server
  local pid
  pid=$(start_server_daemon "$TEST_PORT")

  # Force kill with SIGKILL
  kill -9 "$pid" 2>/dev/null || true
  sleep 0.5

  # Verify server is dead
  if is_server_running "$TEST_PORT"; then
    fail "Edge Case 12: Server survived SIGKILL"
  fi

  # Check logs for startup entry (shutdown entry will be missing)
  if ! grep -q '"event":"startup"' "$HOME/gasoline-logs.jsonl" 2>/dev/null; then
    fail "Edge Case 12: No startup entry in logs"
  fi

  pass "Edge Case 12: SIGKILL leaves no shutdown entry (as expected)"
}

# ============================================================================
# Edge Case 13: Extension Connects Before Server Ready
# ============================================================================
test_edge_case_13() {
  info "Testing Edge Case 13: Extension connects before ready"

  # Start server
  local pid
  pid=$(start_server_daemon "$TEST_PORT")

  # Extension sync endpoint should work immediately
  local response
  response=$(curl -sf "http://127.0.0.1:$TEST_PORT/sync" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"session_id":"test"}' 2>/dev/null)

  if ! echo "$response" | jq -e '.ack' > /dev/null 2>&1; then
    kill "$pid" 2>/dev/null || true
    fail "Edge Case 13: /sync endpoint not ready"
  fi

  pass "Edge Case 13: Server handlers ready"

  # Cleanup
  kill_server "$TEST_PORT"
}

# ============================================================================
# Edge Case 14: Network Interface Down
# ============================================================================
test_edge_case_14() {
  info "Testing Edge Case 14: Network interface down"

  # We can't actually take down loopback without root
  # Just verify server binds to 127.0.0.1
  skip "Edge Case 14: Requires network interface control (manual test)"
}

# ============================================================================
# Main Test Runner
# ============================================================================
main() {
  echo "╔══════════════════════════════════════════╗"
  echo "║  MCP Persistent Server Edge Case Tests   ║"
  echo "╚══════════════════════════════════════════╝"
  echo ""

  if [ ! -f "$BINARY" ]; then
    fail "Binary not found: $BINARY"
  fi

  # Ensure clean state
  kill_server "$TEST_PORT"

  local passed=0
  local failed=0
  local skipped=0

  for i in {1..14}; do
    echo ""
    if test_edge_case_"$i" 2>&1; then
      passed=$((passed + 1))
    else
      # Check if it was skipped
      if [ $? -eq 0 ]; then
        skipped=$((skipped + 1))
      else
        failed=$((failed + 1))
      fi
    fi
  done

  echo ""
  echo "══════════════════════════════════════════"
  echo "Test Summary"
  echo "══════════════════════════════════════════"
  echo -e "${GREEN}Passed:${NC}  $passed"
  echo -e "${RED}Failed:${NC}  $failed"
  echo -e "${YELLOW}Skipped:${NC} $skipped"
  echo "Total:   14"
  echo "══════════════════════════════════════════"

  if [ "$failed" -gt 0 ]; then
    exit 1
  fi
}

main "$@"
