#!/bin/bash
# cat-04-configure.sh — UAT tests for the configure tool (11 tests).
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "4" "Configure Tool" "11"
ensure_daemon

# ── 4.1 — configure(health) returns server health ─────────
begin_test "4.1" "configure(health) returns server health" \
    "Verify health action returns status, version, and key health fields" \
    "Health is the liveness probe. Every field is consumed by monitoring."
run_test_4_1() {
    RESPONSE=$(call_tool "configure" '{"action":"health"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if ! check_contains "$text" "version"; then
        fail "Response missing 'version' field. Content: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "uptime_seconds"; then
        fail "Response missing 'uptime_seconds' field. Content: $(truncate "$text")"
        return
    fi
    pass "Health response contains 'version' and 'uptime_seconds'. Content: $(truncate "$text" 200)"
}
run_test_4_1

# ── 4.2 — configure(clear) resets buffers ─────────────────
begin_test "4.2" "configure(clear) resets buffers" \
    "Clear all buffers, then verify observe(logs) returns empty" \
    "Clear is used between test runs. Two-step verification proves the clear worked."
run_test_4_2() {
    local CLEAR_RESP
    CLEAR_RESP=$(call_tool "configure" '{"action":"clear"}')
    if ! check_not_error "$CLEAR_RESP"; then
        fail "Clear returned error. Content: $(truncate "$(extract_content_text "$CLEAR_RESP")")"
        return
    fi
    local OBS_RESP
    OBS_RESP=$(call_tool "observe" '{"what":"logs"}')
    if ! check_not_error "$OBS_RESP"; then
        fail "observe(logs) after clear returned error. Content: $(truncate "$(extract_content_text "$OBS_RESP")")"
        return
    fi
    local obs_text
    obs_text=$(extract_content_text "$OBS_RESP")
    # After clear, expect count:0 or empty logs
    if check_contains "$obs_text" '"count":0' || check_contains "$obs_text" '"count": 0' || check_contains "$obs_text" '"entries":[]' || check_contains "$obs_text" '"entries": []'; then
        pass "Clear succeeded. observe(logs) returned empty/zero count after clear. Content: $(truncate "$obs_text" 200)"
    else
        fail "Clear did not empty logs buffer. Expected count:0 or entries:[], got: $(truncate "$obs_text" 200)"
    fi
}
run_test_4_2

# ── 4.3 — configure(clear) with specific buffer ───────────
begin_test "4.3" "configure(clear) with specific buffer" \
    "Clear only the network buffer, verify response indicates success" \
    "Selective clear is used for targeted debugging. Must not clear everything."
run_test_4_3() {
    RESPONSE=$(call_tool "configure" '{"action":"clear","buffer":"network"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Clear network buffer returned error. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    pass "Selective clear of network buffer succeeded. Content: $(truncate "$text" 200)"
}
run_test_4_3

# ── 4.4 — configure(store) save and load roundtrip ────────
begin_test "4.4" "configure(store) save and load roundtrip" \
    "Save data with key, load it back, verify contents match" \
    "Roundtrip proves serialization/deserialization works. Catches silent corruption."
run_test_4_4() {
    # Save
    local SAVE_RESP
    SAVE_RESP=$(call_tool "configure" '{"action":"store","store_action":"save","namespace":"uat","key":"roundtrip_test","data":{"foo":"bar","num":42}}')
    if ! check_not_error "$SAVE_RESP"; then
        fail "Store save returned error. Content: $(truncate "$(extract_content_text "$SAVE_RESP")")"
        return
    fi
    # Load
    local LOAD_RESP
    LOAD_RESP=$(call_tool "configure" '{"action":"store","store_action":"load","namespace":"uat","key":"roundtrip_test"}')
    if ! check_not_error "$LOAD_RESP"; then
        fail "Store load returned error. Content: $(truncate "$(extract_content_text "$LOAD_RESP")")"
        return
    fi
    local load_text
    load_text=$(extract_content_text "$LOAD_RESP")
    if ! check_contains "$load_text" "foo"; then
        fail "Loaded data missing 'foo'. Content: $(truncate "$load_text")"
        return
    fi
    if ! check_contains "$load_text" "bar"; then
        fail "Loaded data missing 'bar'. Content: $(truncate "$load_text")"
        return
    fi
    pass "Store save/load roundtrip succeeded. Loaded data contains 'foo' and 'bar'. Content: $(truncate "$load_text" 200)"
}
run_test_4_4

# ── 4.5 — configure(store) list shows saved keys ──────────
begin_test "4.5" "configure(store) list shows saved keys" \
    "After saving in 4.4, list keys and verify roundtrip_test appears" \
    "List must reflect actual state, not cached/stale data."
run_test_4_5() {
    local LIST_RESP
    LIST_RESP=$(call_tool "configure" '{"action":"store","store_action":"list","namespace":"uat"}')
    if ! check_not_error "$LIST_RESP"; then
        fail "Store list returned error. Content: $(truncate "$(extract_content_text "$LIST_RESP")")"
        return
    fi
    local list_text
    list_text=$(extract_content_text "$LIST_RESP")
    if ! check_contains "$list_text" "roundtrip_test"; then
        fail "Store list does not contain 'roundtrip_test'. Content: $(truncate "$list_text")"
        return
    fi
    pass "Store list includes 'roundtrip_test' key. Content: $(truncate "$list_text" 200)"
}
run_test_4_5

# ── 4.6 — configure(noise_rule) add and list roundtrip ────
begin_test "4.6" "configure(noise_rule) add and list roundtrip" \
    "Add a noise rule, list rules, verify our rule appears" \
    "Noise rules affect all observe responses. If add silently fails, data is noisy."
run_test_4_6() {
    # Add
    local ADD_RESP
    ADD_RESP=$(call_tool "configure" '{"action":"noise_rule","noise_action":"add","rules":[{"category":"network","match_spec":{"url_regex":"uat_noise_test_pattern"},"classification":"infrastructure"}]}')
    if ! check_not_error "$ADD_RESP"; then
        fail "Noise rule add returned error. Content: $(truncate "$(extract_content_text "$ADD_RESP")")"
        return
    fi
    # List
    local LIST_RESP
    LIST_RESP=$(call_tool "configure" '{"action":"noise_rule","noise_action":"list"}')
    if ! check_not_error "$LIST_RESP"; then
        fail "Noise rule list returned error. Content: $(truncate "$(extract_content_text "$LIST_RESP")")"
        return
    fi
    local list_text
    list_text=$(extract_content_text "$LIST_RESP")
    if ! check_contains "$list_text" "uat_noise_test_pattern"; then
        fail "Noise rule list does not contain 'uat_noise_test_pattern'. Content: $(truncate "$list_text")"
        return
    fi
    pass "Noise rule add/list roundtrip succeeded. List contains 'uat_noise_test_pattern'. Content: $(truncate "$list_text" 200)"
}
run_test_4_6

# ── 4.7 — configure(audit_log) returns entries ────────────
begin_test "4.7" "configure(audit_log) returns entries" \
    "After several tool calls, verify audit log has entries" \
    "Audit trail is compliance-critical. If empty after tool calls, auditing is broken."
run_test_4_7() {
    RESPONSE=$(call_tool "configure" '{"action":"audit_log"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Audit log returned error. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    # Audit log should contain some entries from our prior tool calls
    if [ -z "$text" ]; then
        fail "Audit log response is empty."
        return
    fi
    pass "Audit log returned data. Content: $(truncate "$text" 200)"
}
run_test_4_7

# ── 4.8 — configure(streaming) enable and status ──────────
begin_test "4.8" "configure(streaming) enable and status" \
    "Enable streaming for errors, then check status reflects enabled state" \
    "Streaming state must persist within a session."
run_test_4_8() {
    # Enable
    local ENABLE_RESP
    ENABLE_RESP=$(call_tool "configure" '{"action":"streaming","streaming_action":"enable","events":["errors"]}')
    if ! check_not_error "$ENABLE_RESP"; then
        fail "Streaming enable returned error. Content: $(truncate "$(extract_content_text "$ENABLE_RESP")")"
        return
    fi
    # Status
    local STATUS_RESP
    STATUS_RESP=$(call_tool "configure" '{"action":"streaming","streaming_action":"status"}')
    if ! check_not_error "$STATUS_RESP"; then
        fail "Streaming status returned error. Content: $(truncate "$(extract_content_text "$STATUS_RESP")")"
        return
    fi
    local status_text
    status_text=$(extract_content_text "$STATUS_RESP")
    if ! check_contains "$status_text" "enabled"; then
        fail "Streaming status missing 'enabled' field. Content: $(truncate "$status_text" 200)"
        return
    fi
    pass "Streaming enable/status roundtrip succeeded. Status contains 'enabled' field. Content: $(truncate "$status_text" 200)"
}
run_test_4_8

# ── 4.9 — configure(test_boundary) start and end ──────────
begin_test "4.9" "configure(test_boundary) start and end" \
    "Start and end a test boundary, verify both succeed" \
    "Test boundaries isolate CI test runs. If they error, CI tests can't use the feature."
run_test_4_9() {
    # Start
    local START_RESP
    START_RESP=$(call_tool "configure" '{"action":"test_boundary_start","test_id":"uat-boundary-1","label":"UAT test"}')
    if ! check_not_error "$START_RESP"; then
        fail "test_boundary_start returned error. Content: $(truncate "$(extract_content_text "$START_RESP")")"
        return
    fi
    # End
    local END_RESP
    END_RESP=$(call_tool "configure" '{"action":"test_boundary_end","test_id":"uat-boundary-1"}')
    if ! check_not_error "$END_RESP"; then
        fail "test_boundary_end returned error. Content: $(truncate "$(extract_content_text "$END_RESP")")"
        return
    fi
    pass "test_boundary start and end both succeeded without error."
}
run_test_4_9

# ── 4.10 — configure(query_dom) with selector ─────────────
begin_test "4.10" "configure(query_dom) with selector" \
    "Send query_dom for 'body' — may timeout or return no-extension, but must not crash" \
    "DOM queries are sent to extension via pending queries. Must not crash without extension."
run_test_4_10() {
    RESPONSE=$(call_tool "configure" '{"action":"query_dom","selector":"body"}')
    # We don't care if it's an error or success — just that we got a valid JSON-RPC response
    if ! check_valid_jsonrpc "$RESPONSE"; then
        fail "query_dom did not return valid JSON-RPC. Raw response: $(truncate "$RESPONSE")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    pass "query_dom returned valid JSON-RPC response (may be error or success). Content: $(truncate "$text" 200)"
}
run_test_4_10

# ── 4.11 — configure with invalid action ──────────────────
begin_test "4.11" "configure with invalid action returns error" \
    "Send an unknown action, verify isError:true" \
    "Invalid actions must not silently succeed."
run_test_4_11() {
    RESPONSE=$(call_tool "configure" '{"action":"destroy_everything"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for invalid action 'destroy_everything'. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "Invalid action 'destroy_everything' correctly returned isError:true."
}
run_test_4_11

# ── Cleanup noise rule from 4.6 ───────────────────────────
# Best effort cleanup: remove the noise rule we added so it doesn't affect other tests
call_tool "configure" '{"action":"noise_rule","noise_action":"reset"}' >/dev/null 2>&1

# Disable streaming from 4.8
call_tool "configure" '{"action":"streaming","streaming_action":"disable"}' >/dev/null 2>&1

finish_category
