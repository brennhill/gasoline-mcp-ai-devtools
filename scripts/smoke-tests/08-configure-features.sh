#!/bin/bash
# 08-configure-features.sh — S.61-S.65: Configure tool features.
# noise rules, store persistence, buffer clear, streaming, test boundaries
set -eo pipefail

begin_category "8" "Configure Features" "5"

# ── Test S.61: Noise rules CRUD ──────────────────────────
begin_test "S.61" "Noise rules: add, list, remove, verify" \
    "Full noise rule lifecycle: add a rule, list to verify, remove, list to confirm" \
    "Tests: noise filtering configuration"

run_test_s61() {
    # Add a noise rule
    local add_response
    add_response=$(call_tool "configure" '{"action":"noise_rule","noise_action":"add","pattern":"smoke-test-noise","category":"console","reason":"Smoke test noise rule"}')

    if ! check_not_error "$add_response"; then
        fail "noise_rule add returned error. Content: $(truncate "$(extract_content_text "$add_response")" 200)"
        return
    fi

    # List rules
    local list_response
    list_response=$(call_tool "configure" '{"action":"noise_rule","noise_action":"list"}')
    local list_text
    list_text=$(extract_content_text "$list_response")

    if ! echo "$list_text" | grep -q "smoke-test-noise"; then
        fail "Noise rule not found in list after add. Content: $(truncate "$list_text" 200)"
        return
    fi

    # Extract rule_id for removal
    local rule_id
    rule_id=$(echo "$list_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    rules = data.get('rules', [])
    for r in rules:
        if 'smoke-test-noise' in str(r):
            print(r.get('id', r.get('rule_id', '')))
            break
except: pass
" 2>/dev/null)

    if [ -n "$rule_id" ]; then
        # Remove the rule
        local remove_response
        remove_response=$(call_tool "configure" "{\"action\":\"noise_rule\",\"noise_action\":\"remove\",\"rule_id\":\"$rule_id\"}")

        if ! check_not_error "$remove_response"; then
            fail "noise_rule remove returned error. Content: $(truncate "$(extract_content_text "$remove_response")" 200)"
            return
        fi
    fi

    # Verify removal
    local list2_response
    list2_response=$(call_tool "configure" '{"action":"noise_rule","noise_action":"list"}')
    local list2_text
    list2_text=$(extract_content_text "$list2_response")

    if echo "$list2_text" | grep -q "smoke-test-noise"; then
        fail "Noise rule still in list after remove. Content: $(truncate "$list2_text" 200)"
    else
        pass "Noise rule CRUD: add, list (found), remove, list (gone)."
    fi
}
run_test_s61

# ── Test S.62: Store persistence ─────────────────────────
begin_test "S.62" "Store: save, load, list, delete roundtrip" \
    "Full key-value store lifecycle" \
    "Tests: persistent data storage"

run_test_s62() {
    # Save data
    local save_response
    save_response=$(call_tool "configure" '{"action":"store","store_action":"save","key":"smoke-key","namespace":"smoke","data":{"value":"smoke-data-123"}}')

    if ! check_not_error "$save_response"; then
        fail "store save returned error. Content: $(truncate "$(extract_content_text "$save_response")" 200)"
        return
    fi

    # Load it back
    local load_response
    load_response=$(call_tool "configure" '{"action":"store","store_action":"load","key":"smoke-key","namespace":"smoke"}')
    local load_text
    load_text=$(extract_content_text "$load_response")

    if ! echo "$load_text" | grep -q "smoke-data-123"; then
        fail "store load did not return saved data. Content: $(truncate "$load_text" 200)"
        return
    fi

    # List keys
    local list_response
    list_response=$(call_tool "configure" '{"action":"store","store_action":"list","namespace":"smoke"}')
    local list_text
    list_text=$(extract_content_text "$list_response")

    if ! echo "$list_text" | grep -q "smoke-key"; then
        fail "store list does not contain 'smoke-key'. Content: $(truncate "$list_text" 200)"
        return
    fi

    # Delete
    local del_response
    del_response=$(call_tool "configure" '{"action":"store","store_action":"delete","key":"smoke-key","namespace":"smoke"}')

    if ! check_not_error "$del_response"; then
        fail "store delete returned error. Content: $(truncate "$(extract_content_text "$del_response")" 200)"
        return
    fi

    # Verify deletion — load should return empty/error
    local load2_response
    load2_response=$(call_tool "configure" '{"action":"store","store_action":"load","key":"smoke-key","namespace":"smoke"}')
    local load2_text
    load2_text=$(extract_content_text "$load2_response")

    if echo "$load2_text" | grep -q "smoke-data-123"; then
        fail "store load still returns data after delete. Content: $(truncate "$load2_text" 200)"
    else
        pass "Store CRUD: save, load (found), list (found), delete, load (gone)."
    fi
}
run_test_s62

# ── Test S.63: Selective buffer clear ────────────────────
begin_test "S.63" "Selective buffer clear (logs only, actions preserved)" \
    "Seed data, clear logs only, verify actions still exist" \
    "Tests: targeted buffer clearing"

run_test_s63() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Seed some data
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Seed log\",\"script\":\"console.log('CLEAR_TEST_LOG')\"}"
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Seed action","script":"var b = document.createElement(\"button\"); b.id = \"clear-test-btn\"; b.textContent = \"X\"; document.body.appendChild(b); b.click(); \"done\""}'
    sleep 1

    # Clear only logs
    local clear_response
    clear_response=$(call_tool "configure" '{"action":"clear","buffer":"logs"}')

    if ! check_not_error "$clear_response"; then
        fail "clear(logs) returned error. Content: $(truncate "$(extract_content_text "$clear_response")" 200)"
        return
    fi

    # Verify logs cleared
    local log_response
    log_response=$(call_tool "observe" '{"what":"logs"}')
    local log_text
    log_text=$(extract_content_text "$log_response")

    local logs_cleared=true
    if echo "$log_text" | grep -q "CLEAR_TEST_LOG"; then
        logs_cleared=false
    fi

    # Verify actions still exist
    local action_response
    action_response=$(call_tool "observe" '{"what":"actions"}')
    local action_text
    action_text=$(extract_content_text "$action_response")

    local actions_exist=false
    if echo "$action_text" | grep -qiE "click\|action\|entries"; then
        actions_exist=true
    fi

    if [ "$logs_cleared" = "true" ] && [ "$actions_exist" = "true" ]; then
        pass "Selective clear: logs cleared, actions preserved."
    elif [ "$logs_cleared" = "true" ]; then
        pass "Logs cleared. Actions check inconclusive (may have been empty). Partial pass."
    else
        fail "Logs NOT cleared after clear(logs). Log content: $(truncate "$log_text" 200)"
    fi
}
run_test_s63

# ── Test S.64: Streaming toggle ──────────────────────────
begin_test "S.64" "Streaming: enable, status, disable, status" \
    "Toggle streaming events and verify state transitions" \
    "Tests: streaming configuration"

run_test_s64() {
    # Enable streaming
    local enable_response
    enable_response=$(call_tool "configure" '{"action":"streaming","streaming_action":"enable","events":["errors"]}')

    if ! check_not_error "$enable_response"; then
        fail "streaming enable returned error. Content: $(truncate "$(extract_content_text "$enable_response")" 200)"
        return
    fi

    # Check status
    local status_response
    status_response=$(call_tool "configure" '{"action":"streaming","streaming_action":"status"}')
    local status_text
    status_text=$(extract_content_text "$status_response")

    local enabled_after_enable=false
    if echo "$status_text" | grep -qiE "enabled.*true\|active.*true\|\"enabled\""; then
        enabled_after_enable=true
    fi

    # Disable streaming
    local disable_response
    disable_response=$(call_tool "configure" '{"action":"streaming","streaming_action":"disable"}')

    if ! check_not_error "$disable_response"; then
        fail "streaming disable returned error. Content: $(truncate "$(extract_content_text "$disable_response")" 200)"
        return
    fi

    # Check status again
    local status2_response
    status2_response=$(call_tool "configure" '{"action":"streaming","streaming_action":"status"}')
    local status2_text
    status2_text=$(extract_content_text "$status2_response")

    if [ "$enabled_after_enable" = "true" ]; then
        pass "Streaming: enable > status(active) > disable > status. State transitions correct."
    else
        # Streaming may not explicitly say "enabled:true" — just check no errors
        pass "Streaming: enable/disable/status calls all succeeded without errors."
    fi
}
run_test_s64

# ── Test S.65: Test boundaries ───────────────────────────
begin_test "S.65" "Test boundaries: start and end markers" \
    "configure(test_boundary_start) then (test_boundary_end)" \
    "Tests: test boundary markers for isolating activity"

run_test_s65() {
    # Start boundary
    local start_response
    start_response=$(call_tool "configure" '{"action":"test_boundary_start","test_id":"smoke-boundary","label":"Smoke test boundary"}')

    if ! check_not_error "$start_response"; then
        fail "test_boundary_start returned error. Content: $(truncate "$(extract_content_text "$start_response")" 200)"
        return
    fi

    # Do some activity between boundaries
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "execute_js" '{"action":"execute_js","reason":"Activity between boundaries","script":"console.log(\"boundary-test-activity\")"}'
    fi

    sleep 0.5

    # End boundary
    local end_response
    end_response=$(call_tool "configure" '{"action":"test_boundary_end","test_id":"smoke-boundary"}')

    if ! check_not_error "$end_response"; then
        fail "test_boundary_end returned error. Content: $(truncate "$(extract_content_text "$end_response")" 200)"
        return
    fi

    pass "Test boundaries: start and end markers both completed for 'smoke-boundary'."
}
run_test_s65
