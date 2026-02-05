#!/usr/bin/env bash
# Common functions for regression tests
# This file is sourced by test scripts

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
export GASOLINE_PORT="${GASOLINE_PORT:-17890}"
export GASOLINE_URL="http://127.0.0.1:${GASOLINE_PORT}"
export GASOLINE_BINARY="${GASOLINE_BINARY:-./dist/gasoline}"
export TEST_TIMEOUT="${TEST_TIMEOUT:-30}"

# Track server PID and temp log file for cleanup
GASOLINE_PID=""
GASOLINE_TEST_LOG=""

# Find a free port
find_free_port() {
    local port
    for port in $(seq 17890 17999); do
        if ! nc -z 127.0.0.1 "$port" 2>/dev/null; then
            echo "$port"
            return 0
        fi
    done
    echo "ERROR: No free port found" >&2
    return 1
}

# Start the gasoline server
start_server() {
    # Always find a fresh port to avoid conflicts between tests
    local port
    port=$(find_free_port)

    # Check if binary exists
    if [[ ! -x "$GASOLINE_BINARY" ]]; then
        echo -e "${RED}ERROR: Binary not found at $GASOLINE_BINARY${NC}" >&2
        echo "Run: go build -o dist/gasoline ./cmd/dev-console" >&2
        return 1
    fi

    # Use a temporary log file to ensure clean state for each test
    GASOLINE_TEST_LOG="/tmp/gasoline-test-$$.jsonl"

    # Start server in background
    # Note: We must NOT redirect stdin to /dev/null as that triggers MCP client mode
    # which causes the server to exit after 5 seconds when no stdin is available.
    # Instead, we use a pipe (via process substitution) which keeps stdin open.
    "$GASOLINE_BINARY" --port "$port" --log-file "$GASOLINE_TEST_LOG" >/dev/null 2>&1 &
    GASOLINE_PID=$!
    export GASOLINE_PID

    # Wait for server to be ready
    if ! wait_for_server "$port" 5; then
        echo -e "${RED}ERROR: Server failed to start${NC}" >&2
        return 1
    fi

    export GASOLINE_PORT="$port"
    export GASOLINE_URL="http://127.0.0.1:${port}"
}

# Wait for server to be ready
wait_for_server() {
    local port="${1:-$GASOLINE_PORT}"
    local timeout="${2:-10}"
    local elapsed=0

    while [[ $elapsed -lt $timeout ]]; do
        if curl -s --max-time 1 "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
            # Extra delay to ensure server is fully ready
            sleep 0.5
            return 0
        fi
        sleep 0.5
        elapsed=$((elapsed + 1))
    done

    return 1
}

# Stop the server
stop_server() {
    if [[ -n "${GASOLINE_PID:-}" ]]; then
        kill "$GASOLINE_PID" 2>/dev/null || true
        wait "$GASOLINE_PID" 2>/dev/null || true
        GASOLINE_PID=""
    fi
}

# Cleanup on exit
cleanup() {
    stop_server
    # Remove temp log file if it exists
    if [[ -n "${GASOLINE_TEST_LOG:-}" && -f "$GASOLINE_TEST_LOG" ]]; then
        rm -f "$GASOLINE_TEST_LOG"
    fi
}
trap cleanup EXIT

# Check if server is running
is_server_running() {
    local port="${1:-$GASOLINE_PORT}"
    curl -s "http://127.0.0.1:${port}/health" >/dev/null 2>&1
}

# Make an MCP request
mcp_call() {
    local method="$1"
    local params="$2"
    local id="${3:-1}"

    # Default params to empty object if not provided
    if [[ -z "$params" ]]; then
        params='{}'
    fi

    curl -s -X POST "${GASOLINE_URL}/mcp" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"id\":${id},\"method\":\"${method}\",\"params\":${params}}"
}

# Make an MCP tool call
mcp_tool() {
    local tool="$1"
    local args="$2"

    # Default args to empty object if not provided
    if [[ -z "$args" ]]; then
        args='{}'
    fi

    local params
    params=$(printf '{"name":"%s","arguments":%s}' "$tool" "$args")

    curl -s --max-time 30 -X POST "${GASOLINE_URL}/mcp" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":${params}}"
}

# POST data to an endpoint (simulating extension)
post_data() {
    local endpoint="$1"
    local data="$2"

    curl -s -X POST "${GASOLINE_URL}${endpoint}" \
        -H "Content-Type: application/json" \
        -d "$data"
}

# GET data from an endpoint
get_data() {
    local endpoint="$1"

    curl -s "${GASOLINE_URL}${endpoint}"
}

# Log test result
log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

log_info() {
    echo -e "[INFO] $1"
}

# Run a test function and track result
run_test() {
    local name="$1"
    shift

    if "$@"; then
        log_pass "$name"
        return 0
    else
        log_fail "$name"
        return 1
    fi
}
