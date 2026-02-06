#!/usr/bin/env bash
# Test fast-start bridge mode: instant responses during daemon boot
# Validates the UX improvements from v5.7.4+

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
    sleep 0.3
}

# Send MCP request and capture response with timeout
# Usage: mcp_request <timeout_sec> <json_request>
mcp_request() {
    local timeout_sec=$1
    local request=$2
    local tmp_out=$(mktemp)
    local max_iterations=$((timeout_sec * 10))  # 0.1s per iteration

    (echo "$request"; sleep $timeout_sec) | "$BINARY" --port "$PORT" > "$tmp_out" 2>/dev/null &
    local pid=$!

    # Wait for response (don't wait for process to finish - just check output)
    local count=0
    while [ $count -lt $max_iterations ]; do
        if [ -s "$tmp_out" ]; then
            # Got response - read it and kill process
            local result=$(cat "$tmp_out")
            kill $pid 2>/dev/null || true
            # Don't wait - let it die in background
            rm -f "$tmp_out"
            echo "$result"
            return 0
        fi
        sleep 0.1
        count=$((count + 1))
    done

    kill $pid 2>/dev/null || true
    cat "$tmp_out"
    rm -f "$tmp_out"
    return 1
}

# Ensure clean state
cleanup

echo "╔══════════════════════════════════════════╗"
echo "║  Fast-Start Bridge Mode Tests            ║"
echo "╚══════════════════════════════════════════╝"
echo ""

# ═══════════════════════════════════════════════════════════════
# Test 1: Bridge mode responds to initialize immediately
# ═══════════════════════════════════════════════════════════════
info "Test 1: Bridge responds to initialize immediately (< 1s)"

START=$(python3 -c 'import time; print(int(time.time()*1000))')
RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}')
END=$(python3 -c 'import time; print(int(time.time()*1000))')

DURATION=$((END - START))

if [ -z "$RESULT" ]; then
  fail "Test 1: No response from bridge"
fi

if ! echo "$RESULT" | jq -e '.result.protocolVersion' > /dev/null 2>&1; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 1: Response missing protocolVersion"
fi

if [ $DURATION -gt 1000 ]; then
  fail "Test 1: Response took ${DURATION}ms (> 1000ms)"
fi

pass "Test 1: initialize responded in ${DURATION}ms"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 2: Bridge mode responds to tools/list immediately
# ═══════════════════════════════════════════════════════════════
info "Test 2: Bridge responds to tools/list immediately (< 1s)"

START=$(python3 -c 'import time; print(int(time.time()*1000))')
RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}')
END=$(python3 -c 'import time; print(int(time.time()*1000))')

DURATION=$((END - START))

if [ -z "$RESULT" ]; then
  fail "Test 2: No response from bridge"
fi

if ! echo "$RESULT" | jq -e '.result.tools | length > 0' > /dev/null 2>&1; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 2: Response missing tools"
fi

# Verify expected tools are present
TOOLS=$(echo "$RESULT" | jq -r '.result.tools[].name' | sort | tr '\n' ' ')
if ! echo "$TOOLS" | grep -q "configure"; then
  fail "Test 2: Missing 'configure' tool"
fi
if ! echo "$TOOLS" | grep -q "observe"; then
  fail "Test 2: Missing 'observe' tool"
fi

if [ $DURATION -gt 1000 ]; then
  fail "Test 2: Response took ${DURATION}ms (> 1000ms)"
fi

pass "Test 2: tools/list in ${DURATION}ms (tools: $(echo $TOOLS | tr ' ' ','))"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 3: Version is included in initialize response
# ═══════════════════════════════════════════════════════════════
info "Test 3: Version included in initialize response"

RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":3,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}')

VERSION=$(echo "$RESULT" | jq -r '.result.serverInfo.version // empty')

if [ -z "$VERSION" ]; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 3: Version not in response"
fi

# Check version looks reasonable (starts with digit)
if ! echo "$VERSION" | grep -qE '^[0-9]'; then
  fail "Test 3: Version doesn't look valid: $VERSION"
fi

pass "Test 3: Version is $VERSION"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 4: ping responds immediately
# ═══════════════════════════════════════════════════════════════
info "Test 4: ping responds immediately (< 1s)"

START=$(python3 -c 'import time; print(int(time.time()*1000))')
RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":5,"method":"ping"}')
END=$(python3 -c 'import time; print(int(time.time()*1000))')

DURATION=$((END - START))

if [ -z "$RESULT" ]; then
  fail "Test 4: No response to ping"
fi

if ! echo "$RESULT" | jq -e '.result' > /dev/null 2>&1; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 4: Ping response has no result"
fi

if [ $DURATION -gt 1000 ]; then
  fail "Test 4: Response took ${DURATION}ms (> 1000ms)"
fi

pass "Test 4: ping in ${DURATION}ms"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 5: prompts/list responds immediately
# ═══════════════════════════════════════════════════════════════
info "Test 5: prompts/list responds immediately (< 1s)"

START=$(python3 -c 'import time; print(int(time.time()*1000))')
RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":6,"method":"prompts/list"}')
END=$(python3 -c 'import time; print(int(time.time()*1000))')
DURATION=$((END - START))

if ! echo "$RESULT" | jq -e '.result.prompts' > /dev/null 2>&1; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 5: prompts/list missing prompts array"
fi

if [ $DURATION -gt 1000 ]; then
  fail "Test 5: Response took ${DURATION}ms (> 1000ms)"
fi

pass "Test 5: prompts/list in ${DURATION}ms"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 6: resources/list responds immediately
# ═══════════════════════════════════════════════════════════════
info "Test 6: resources/list responds immediately (< 1s)"

START=$(python3 -c 'import time; print(int(time.time()*1000))')
RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":7,"method":"resources/list"}')
END=$(python3 -c 'import time; print(int(time.time()*1000))')
DURATION=$((END - START))

if ! echo "$RESULT" | jq -e '.result.resources' > /dev/null 2>&1; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 6: resources/list missing resources array"
fi

if [ $DURATION -gt 1000 ]; then
  fail "Test 6: Response took ${DURATION}ms (> 1000ms)"
fi

pass "Test 6: resources/list in ${DURATION}ms"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 7: resources/templates/list responds immediately
# ═══════════════════════════════════════════════════════════════
info "Test 7: resources/templates/list responds immediately (< 1s)"

START=$(python3 -c 'import time; print(int(time.time()*1000))')
RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":8,"method":"resources/templates/list"}')
END=$(python3 -c 'import time; print(int(time.time()*1000))')
DURATION=$((END - START))

if ! echo "$RESULT" | jq -e '.result.resourceTemplates' > /dev/null 2>&1; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 7: resources/templates/list missing resourceTemplates array"
fi

if [ $DURATION -gt 1000 ]; then
  fail "Test 7: Response took ${DURATION}ms (> 1000ms)"
fi

pass "Test 7: resources/templates/list in ${DURATION}ms"

cleanup

# ═══════════════════════════════════════════════════════════════
# Test 8: Server name is 'gasoline' in response
# ═══════════════════════════════════════════════════════════════
info "Test 8: Server name is 'gasoline' in response"

RESULT=$(mcp_request 5 '{"jsonrpc":"2.0","id":8,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}')

SERVER_NAME=$(echo "$RESULT" | jq -r '.result.serverInfo.name // empty')

if [ "$SERVER_NAME" != "gasoline" ]; then
  echo "DEBUG: Got response: $RESULT"
  fail "Test 8: Server name is '$SERVER_NAME', expected 'gasoline'"
fi

pass "Test 8: Server name is '$SERVER_NAME'"

cleanup

echo ""
echo "══════════════════════════════════════════"
echo "  All fast-start tests passed!"
echo "══════════════════════════════════════════"
