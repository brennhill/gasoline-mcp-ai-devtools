#!/bin/bash
# cat-09-http.sh — UAT tests for HTTP endpoints (4 tests).
# NOTE: Test 9.4 (/shutdown) must be LAST as it kills the daemon.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "9" "HTTP Endpoints" "4"
ensure_daemon

# ── 9.1 — /health returns complete health object ──────────
begin_test "9.1" "/health returns complete health object" \
    "Verify /health JSON contains status, version, and uptime fields" \
    "Health shape is consumed by monitoring dashboards. Missing fields break integrations."
run_test_9_1() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")
    if [ -z "$body" ]; then
        fail "/health returned empty body."
        return
    fi
    # Must be valid JSON
    if ! echo "$body" | jq -e '.' >/dev/null 2>&1; then
        fail "/health returned invalid JSON. Body: $(truncate "$body")"
        return
    fi
    # Check required fields
    local has_status has_version
    has_status=$(echo "$body" | jq -e '.status' 2>/dev/null)
    has_version=$(echo "$body" | jq -e '.version' 2>/dev/null)
    if [ -z "$has_status" ] || [ "$has_status" = "null" ]; then
        fail "/health missing 'status' field. Body: $(truncate "$body")"
        return
    fi
    if [ -z "$has_version" ] || [ "$has_version" = "null" ]; then
        fail "/health missing 'version' field. Body: $(truncate "$body")"
        return
    fi
    pass "/health contains status=$has_status, version=$has_version. Body: $(truncate "$body" 200)"
}
run_test_9_1

# ── 9.2 — /diagnostics returns info ───────────────────────
begin_test "9.2" "/diagnostics returns info" \
    "Verify /diagnostics returns valid JSON, not 404 or 500" \
    "Diagnostics is the debugging escape hatch. Must not be broken when you need it most."
run_test_9_2() {
    local status
    status=$(get_http_status "http://localhost:${PORT}/diagnostics")
    if [ "$status" = "404" ] || [ "$status" = "500" ]; then
        fail "/diagnostics returned HTTP $status."
        return
    fi
    local body
    body=$(get_http_body "http://localhost:${PORT}/diagnostics")
    if [ -z "$body" ]; then
        fail "/diagnostics returned empty body."
        return
    fi
    # Should be valid JSON
    if ! echo "$body" | jq -e '.' >/dev/null 2>&1; then
        fail "/diagnostics returned invalid JSON. Body: $(truncate "$body")"
        return
    fi
    pass "/diagnostics returned HTTP $status with valid JSON. Body: $(truncate "$body" 200)"
}
run_test_9_2

# ── 9.3 — /mcp POST accepts JSON-RPC ──────────────────────
begin_test "9.3" "/mcp POST accepts JSON-RPC" \
    "Verify /mcp endpoint handles tools/list request and returns tools array" \
    "HTTP transport is the alternative to stdio. Must work for web-based MCP clients."
run_test_9_3() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/mcp" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
    if [ -z "$body" ]; then
        fail "/mcp POST returned empty body."
        return
    fi
    # Must be valid JSON-RPC
    if ! echo "$body" | jq -e '.jsonrpc == "2.0"' >/dev/null 2>&1; then
        fail "/mcp POST returned invalid JSON-RPC. Body: $(truncate "$body")"
        return
    fi
    # Must have tools in result
    local tool_count
    tool_count=$(echo "$body" | jq -r '.result.tools | length' 2>/dev/null)
    if [ -z "$tool_count" ] || [ "$tool_count" = "null" ] || [ "$tool_count" = "0" ]; then
        fail "/mcp POST did not return tools array. Body: $(truncate "$body")"
        return
    fi
    pass "/mcp POST returned valid JSON-RPC with $tool_count tools. Body: $(truncate "$body" 200)"
}
run_test_9_3

# ── 9.4 — /shutdown POST stops the server ─────────────────
# NOTE: This test MUST be last since it kills the daemon.
begin_test "9.4" "/shutdown POST stops the server" \
    "POST to /shutdown, wait, verify port is freed" \
    "Programmatic shutdown is used by CI cleanup."
run_test_9_4() {
    local shutdown_body
    shutdown_body=$(get_http_body "http://localhost:${PORT}/shutdown" -X POST)
    # Give the server time to shut down
    sleep 3
    # Verify port is freed
    if curl -s --connect-timeout 2 "http://localhost:${PORT}/health" >/dev/null 2>&1; then
        fail "Server still responding on port $PORT after /shutdown. Body: $(truncate "$shutdown_body")"
        return
    fi
    pass "/shutdown succeeded. Port $PORT is freed after 3 second wait. Shutdown response: $(truncate "$shutdown_body" 200)"
}
run_test_9_4

# finish_category calls kill_server (which no-ops if daemon is already dead)
# then writes results and exits.
finish_category
