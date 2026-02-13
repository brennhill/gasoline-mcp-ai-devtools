#!/bin/bash
# 01-bootstrap.sh — S.1-S.4: Cold start, health, extension gate, navigate.
# Sets: EXTENSION_CONNECTED, PILOT_ENABLED
set -eo pipefail

begin_category "1" "Bootstrap" "4"

# ── Test S.1: Cold start auto-spawn ──────────────────────
begin_test "S.1" "Cold start auto-spawn" \
    "Kill any running daemon, send an MCP call, verify the daemon spawns automatically" \
    "This is the most critical path — if cold start fails, nothing works"

run_test_s1() {
    kill_server
    sleep 0.5

    if lsof -ti :"$PORT" >/dev/null 2>&1; then
        fail "Port $PORT still occupied after kill. Cannot test cold start."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"page"}')

    if [ -z "$response" ]; then
        fail "No response at all. Daemon failed to auto-spawn."
        return
    fi

    if ! check_valid_jsonrpc "$response"; then
        fail "Response is not valid JSON-RPC: $(truncate "$response")"
        return
    fi

    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -q "starting up"; then
        pass "Cold start: daemon spawned (got 'retry in 2s' message)."
    elif check_not_error "$response"; then
        pass "Cold start: daemon spawned and responded immediately."
    else
        fail "Cold start: daemon spawned but returned tool-level error. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s1

# ── Test S.2: Health + version ───────────────────────────
begin_test "S.2" "Health endpoint and version" \
    "Verify /health returns status=ok and version matches VERSION file" \
    "Version mismatch means the wrong binary is running"

run_test_s2() {
    sleep 2
    if ! wait_for_health 50; then
        fail "Daemon not healthy after 50 attempts. Cannot check version."
        return
    fi

    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)
    if [ "$status_val" != "ok" ]; then
        fail "Health status='$status_val', expected 'ok'. Body: $(truncate "$body")"
        return
    fi

    local health_version
    health_version=$(echo "$body" | jq -r '.version // empty' 2>/dev/null)
    if [ "$health_version" != "$VERSION" ]; then
        fail "Version mismatch: health='$health_version', VERSION file='$VERSION'."
        return
    fi

    pass "Health OK: status='ok', version='$health_version' matches VERSION file."
}
run_test_s2

# ── Test S.3: Extension gate ─────────────────────────────
begin_test "S.3" "Extension connected" \
    "Check /health for capture.available=true" \
    "All browser tests require extension. Stops here if not connected."

run_test_s3() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local capture_available
    capture_available=$(echo "$body" | jq -r '.capture.available // false' 2>/dev/null)

    if [ "$capture_available" = "true" ]; then
        EXTENSION_CONNECTED=true
        pass "Extension connected: capture.available=true."
    else
        fail "Extension NOT connected. Open Chrome with Gasoline extension and track a tab."
        echo "" | tee -a "$OUTPUT_FILE"
        echo "  >>> 1. Open Chrome with the Gasoline extension installed" | tee -a "$OUTPUT_FILE"
        echo "  >>> 2. Click the Gasoline icon > 'Track This Tab' on any page" | tee -a "$OUTPUT_FILE"
        echo "  >>> 3. Enable 'AI Web Pilot' toggle in the extension popup" | tee -a "$OUTPUT_FILE"
        echo "  >>> 4. Re-run: bash scripts/smoke-test.sh" | tee -a "$OUTPUT_FILE"
        echo "" | tee -a "$OUTPUT_FILE"
    fi
}
run_test_s3

# ── Test S.4: Navigate to test page ──────────────────────
begin_test "S.4" "Navigate to a page" \
    "Use interact(navigate) to open example.com, verify observe(page) reflects it" \
    "Tests the full interact pipeline: MCP > daemon > extension > browser"

run_test_s4() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    call_tool "configure" '{"action":"clear","buffer":"all"}' >/dev/null

    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Initial page load"}'

    if echo "$INTERACT_RESULT" | grep -qi "pilot.*disabled\|not enabled\|web pilot"; then
        skip "AI Web Pilot is disabled. Enable it in the extension popup and re-run."
        return
    fi

    PILOT_ENABLED=true
    sleep 3

    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "example.com"; then
        pass "Navigated to example.com. observe(page) confirms URL."
    else
        fail "Navigate did not work. observe(page) still shows: $(truncate "$content_text" 200)"
    fi
}
run_test_s4
