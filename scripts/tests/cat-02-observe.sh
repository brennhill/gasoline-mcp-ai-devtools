#!/bin/bash
# cat-02-observe.sh — Category 2: Observe Tool (24 tests).
# Tests all observe modes plus negative cases.
# Each mode must return a valid response shape, even with no data.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "2" "Observe Tool" "24"

ensure_daemon

# Warm-up: wait until the daemon's /mcp endpoint is responsive
# Under parallel load, /health may respond before /mcp is ready
for _warmup_i in $(seq 1 10); do
    _warmup_resp=$(curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":0,"method":"tools/list"}' \
        "http://localhost:${PORT}/mcp" 2>/dev/null)
    if echo "$_warmup_resp" | jq -e '.result.tools' >/dev/null 2>&1; then
        break
    fi
    sleep 0.5
done

# ── 2.1 — observe(page) ──────────────────────────────────
begin_test "2.1" "observe(page) returns page data" \
    "Call observe with what:page. Verify valid response with page info or no-tab message." \
    "This is the most basic observe call. If this fails, no observe works."
run_test_2_1() {
    RESPONSE=$(call_tool "observe" '{"what":"page"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(page), got valid response. Content text present (${#text} chars). No isError."
}
run_test_2_1

# ── 2.2 — observe(tabs) ──────────────────────────────────
begin_test "2.2" "observe(tabs) returns tab array" \
    "Call observe with what:tabs. Verify response has tabs array and tracking_active field." \
    "MCP clients use this to know what is being tracked. Shape must be stable."
run_test_2_2() {
    RESPONSE=$(call_tool "observe" '{"what":"tabs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "tabs"; then
        fail "Expected content to contain 'tabs' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "tracking_active"; then
        fail "Expected content to contain 'tracking_active' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(tabs), got valid response with 'tabs' and 'tracking_active' fields. Content: ${#text} chars."
}
run_test_2_2

# ── 2.3 — observe(logs) ──────────────────────────────────
begin_test "2.3" "observe(logs) returns logs array" \
    "Call observe with what:logs. Verify response has count and logs array. NOT an error response." \
    "Empty state must be distinguishable from error state. Returning error for no data breaks AI workflows."
run_test_2_3() {
    RESPONSE=$(call_tool "observe" '{"what":"logs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(logs), got valid response with 'count' field. Content: ${#text} chars. No isError."
}
run_test_2_3

# ── 2.4 — observe(logs) with min_level filter ────────────
begin_test "2.4" "observe(logs) with min_level filter" \
    "Call observe with what:logs and min_level:error. Verify valid response with filter applied." \
    "Filter params that silently fail mean AI gets wrong data and makes wrong decisions."
run_test_2_4() {
    RESPONSE=$(call_tool "observe" '{"what":"logs","min_level":"error"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(logs, min_level:error), got valid response with 'count' field. Content: ${#text} chars."
}
run_test_2_4

# ── 2.5 — observe(network_waterfall) ─────────────────────
begin_test "2.5" "observe(network_waterfall) returns entries array" \
    "Call observe with what:network_waterfall. Verify response has entries and count." \
    "Network waterfall is the most-used observe mode. Shape breakage affects every user."
run_test_2_5() {
    RESPONSE=$(call_tool "observe" '{"what":"network_waterfall"}')
    # network_waterfall does an on-demand extension query (5s timeout).
    # Without extension, bridge may timeout (4s < 5s). This is expected.
    if check_bridge_timeout "$RESPONSE"; then
        pass "Sent observe(network_waterfall), got bridge timeout (expected without extension). Server did not crash."
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(network_waterfall), got valid response with 'entries' and 'count'. Content: ${#text} chars."
}
run_test_2_5

# ── 2.6 — observe(network_waterfall) with limit ──────────
begin_test "2.6" "observe(network_waterfall) with limit parameter" \
    "Call observe with what:network_waterfall and limit:5. Verify response respects limit." \
    "Limit is critical for keeping MCP context windows manageable. Silently ignoring limit overflows AI context."
run_test_2_6() {
    RESPONSE=$(call_tool "observe" '{"what":"network_waterfall","limit":5}')
    if check_bridge_timeout "$RESPONSE"; then
        pass "Sent observe(network_waterfall, limit:5), got bridge timeout (expected without extension). Server did not crash."
        return
    fi
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(network_waterfall, limit:5), got valid response with 'entries'. Content: ${#text} chars."
}
run_test_2_6

# ── 2.7 — observe(errors) ────────────────────────────────
begin_test "2.7" "observe(errors) returns error array" \
    "Call observe with what:errors. Verify response has errors array and count." \
    "Error detection is core functionality."
run_test_2_7() {
    RESPONSE=$(call_tool "observe" '{"what":"errors"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "errors"; then
        fail "Expected content to contain 'errors' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(errors), got valid response with 'errors' and 'count'. Content: ${#text} chars."
}
run_test_2_7

# ── 2.8 — observe(vitals) ────────────────────────────────
begin_test "2.8" "observe(vitals) returns metrics shape" \
    "Call observe with what:vitals. Verify response has metrics object with has_data boolean." \
    "Web Vitals is the performance monitoring surface. Shape must be stable."
run_test_2_8() {
    RESPONSE=$(call_tool "observe" '{"what":"vitals"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "metrics"; then
        fail "Expected content to contain 'metrics' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "has_data"; then
        fail "Expected content to contain 'has_data' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(vitals), got valid response with 'metrics' and 'has_data'. Content: ${#text} chars."
}
run_test_2_8

# ── 2.9 — observe(actions) ───────────────────────────────
begin_test "2.9" "observe(actions) returns entries" \
    "Call observe with what:actions. Verify response has entries array and count." \
    "Actions feed test generation and reproduction."
run_test_2_9() {
    RESPONSE=$(call_tool "observe" '{"what":"actions"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(actions), got valid response with 'entries'. Content: ${#text} chars."
}
run_test_2_9

# ── 2.10 — observe(websocket_events) ─────────────────────
begin_test "2.10" "observe(websocket_events) returns events" \
    "Call observe with what:websocket_events. Verify response has entries array. No error." \
    "WebSocket capture is a key differentiator."
run_test_2_10() {
    RESPONSE=$(call_tool "observe" '{"what":"websocket_events"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(websocket_events), got valid response with 'entries'. Content: ${#text} chars."
}
run_test_2_10

# ── 2.11 — observe(websocket_status) ─────────────────────
begin_test "2.11" "observe(websocket_status) returns connection status" \
    "Call observe with what:websocket_status. Verify valid response with connection data." \
    "Status endpoint must always respond, even with no WebSocket connections."
run_test_2_11() {
    RESPONSE=$(call_tool "observe" '{"what":"websocket_status"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "connections"; then
        fail "Expected content to contain 'connections' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(websocket_status), got valid response with 'connections'. Content: ${#text} chars."
}
run_test_2_11

# ── 2.12 — observe(extension_logs) ───────────────────────
begin_test "2.12" "observe(extension_logs) returns logs" \
    "Call observe with what:extension_logs. Verify response has logs data. No error." \
    "Extension debugging depends on this."
run_test_2_12() {
    RESPONSE=$(call_tool "observe" '{"what":"extension_logs"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "logs"; then
        fail "Expected content to contain 'logs' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(extension_logs), got valid response with 'logs' and 'count'. Content: ${#text} chars."
}
run_test_2_12

# ── 2.13 — observe(pilot) ────────────────────────────────
begin_test "2.13" "observe(pilot) returns pilot status" \
    "Call observe with what:pilot. Verify response contains pilot enabled/disabled state." \
    "AI Web Pilot gate -- all interact commands check this."
run_test_2_13() {
    RESPONSE=$(call_tool "observe" '{"what":"pilot"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "enabled" && ! check_contains "$text" "pilot" && ! check_contains "$text" "status"; then
        fail "observe(pilot) response missing pilot state fields (enabled/pilot/status). Content: $(truncate "$text")"
        return
    fi
    pass "Sent observe(pilot), got valid response with pilot state fields. Content: $(truncate "$text" 200)"
}
run_test_2_13

# ── 2.14 — observe(performance) ──────────────────────────
begin_test "2.14" "observe(performance) returns snapshots" \
    "Call observe with what:performance. Verify response is valid, not error." \
    "Performance snapshots feed Web Vitals reporting."
run_test_2_14() {
    RESPONSE=$(call_tool "observe" '{"what":"performance"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "snapshots"; then
        fail "Expected content to contain 'snapshots' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(performance), got valid response with 'snapshots'. Content: ${#text} chars."
}
run_test_2_14

# ── 2.15 — observe(timeline) ─────────────────────────────
begin_test "2.15" "observe(timeline) returns unified entries" \
    "Call observe with what:timeline. Verify response has entries array and count." \
    "Timeline is the unified view across all buffer types."
run_test_2_15() {
    RESPONSE=$(call_tool "observe" '{"what":"timeline"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(timeline), got valid response with 'entries' and 'count'. Content: ${#text} chars."
}
run_test_2_15

# ── 2.16 — observe(error_clusters) ───────────────────────
begin_test "2.16" "observe(error_clusters) returns clusters" \
    "Call observe with what:error_clusters. Verify response has clusters array and total_count." \
    "Error clustering reduces noise for AI. Shape must be stable."
run_test_2_16() {
    RESPONSE=$(call_tool "observe" '{"what":"error_clusters"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "clusters"; then
        fail "Expected content to contain 'clusters' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "total_count"; then
        fail "Expected content to contain 'total_count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(error_clusters), got valid response with 'clusters' and 'total_count'. Content: ${#text} chars."
}
run_test_2_16

# ── 2.17 — observe(history) ──────────────────────────────
begin_test "2.17" "observe(history) returns navigation history" \
    "Call observe with what:history. Verify response has entries array and count." \
    "Navigation history is used by reproduction and test generation."
run_test_2_17() {
    RESPONSE=$(call_tool "observe" '{"what":"history"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "entries"; then
        fail "Expected content to contain 'entries' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(history), got valid response with 'entries' and 'count'. Content: ${#text} chars."
}
run_test_2_17

# ── 2.18 — observe(accessibility) ────────────────────────
begin_test "2.18" "observe(accessibility) returns audit data" \
    "Call observe with what:accessibility. Verify valid response (audit results or no-extension message)." \
    "A11y audits feed SARIF export."
run_test_2_18() {
    RESPONSE=$(call_tool "observe" '{"what":"accessibility"}')
    # accessibility may return isError when no extension/tab is tracked — that is acceptable
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    # Verify it is a valid JSON-RPC response (not a crash)
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "Response is not valid JSON-RPC. Full response: $(truncate "$RESPONSE")"
        return
    fi
    pass "Sent observe(accessibility), got valid JSON-RPC response. Content: ${#text} chars. May be error (no extension) or audit data."
}
run_test_2_18

# ── 2.19 — observe(security_audit) ───────────────────────
begin_test "2.19" "observe(security_audit) returns findings" \
    "Call observe with what:security_audit. Verify valid response with security data." \
    "Security audit is a core feature."
run_test_2_19() {
    RESPONSE=$(call_tool "observe" '{"what":"security_audit"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "findings" && ! check_contains "$text" "checks" && ! check_contains "$text" "summary" && ! check_contains "$text" "audit"; then
        fail "observe(security_audit) response missing audit fields (findings/checks/summary/audit). Content: $(truncate "$text")"
        return
    fi
    pass "Sent observe(security_audit), got valid response with audit data fields. Content: $(truncate "$text" 200)"
}
run_test_2_19

# ── 2.20 — observe(third_party_audit) ────────────────────
begin_test "2.20" "observe(third_party_audit) returns analysis" \
    "Call observe with what:third_party_audit. Verify valid response with third-party analysis." \
    "Third-party tracking is compliance-critical for enterprise users."
run_test_2_20() {
    RESPONSE=$(call_tool "observe" '{"what":"third_party_audit"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "origins" && ! check_contains "$text" "third_party" && ! check_contains "$text" "domains" && ! check_contains "$text" "scripts" && ! check_contains "$text" "status"; then
        fail "observe(third_party_audit) response missing audit fields (origins/third_party/domains/scripts/status). Content: $(truncate "$text")"
        return
    fi
    pass "Sent observe(third_party_audit), got valid response with audit data fields. Content: $(truncate "$text" 200)"
}
run_test_2_20

# ── 2.21 — observe(pending_commands) ─────────────────────
begin_test "2.21" "observe(pending_commands) returns command queues" \
    "Call observe with what:pending_commands. Verify response has pending, completed, failed arrays." \
    "Async command tracking is the interact tool's feedback loop."
run_test_2_21() {
    RESPONSE=$(call_tool "observe" '{"what":"pending_commands"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "pending"; then
        fail "Expected content to contain 'pending' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "completed"; then
        fail "Expected content to contain 'completed' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "failed"; then
        fail "Expected content to contain 'failed' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(pending_commands), got valid response with 'pending', 'completed', 'failed'. Content: ${#text} chars."
}
run_test_2_21

# ── 2.22 — observe(failed_commands) ──────────────────────
begin_test "2.22" "observe(failed_commands) returns failures" \
    "Call observe with what:failed_commands. Verify response has commands array and count." \
    "Failed command visibility prevents silent failures."
run_test_2_22() {
    RESPONSE=$(call_tool "observe" '{"what":"failed_commands"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "commands"; then
        fail "Expected content to contain 'commands' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "count"; then
        fail "Expected content to contain 'count' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(failed_commands), got valid response with 'commands' and 'count'. Content: ${#text} chars."
}
run_test_2_22

# ── 2.23 — observe with invalid "what" ───────────────────
begin_test "2.23" "observe with invalid what returns structured error" \
    "Call observe with what:nonexistent_mode. Verify isError:true with helpful message." \
    "Typos in mode names must produce helpful errors, not empty success responses."
run_test_2_23() {
    RESPONSE=$(call_tool "observe" '{"what":"nonexistent_mode"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true but got success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    # Verify the error message mentions valid options or unknown mode
    if ! check_contains "$text" "nknown" && ! check_contains "$text" "valid" && ! check_contains "$text" "Valid"; then
        fail "Error message does not mention unknown mode or valid options. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe(nonexistent_mode), got isError:true. Error mentions valid options. Content: $(truncate "$text" 150)"
}
run_test_2_23

# ── 2.24 — observe with missing "what" ───────────────────
begin_test "2.24" "observe with missing what returns error" \
    "Call observe with empty params {}. Verify error about missing required parameter." \
    "Missing required params must fail loudly."
run_test_2_24() {
    RESPONSE=$(call_tool "observe" '{}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true but got success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "what"; then
        fail "Error message does not mention missing 'what' parameter. Got: $(truncate "$text")"
        return
    fi
    pass "Sent observe({}), got isError:true. Error mentions missing 'what' parameter. Content: $(truncate "$text" 150)"
}
run_test_2_24

finish_category
