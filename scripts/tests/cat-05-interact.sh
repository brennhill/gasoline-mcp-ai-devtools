#!/bin/bash
# cat-05-interact.sh — UAT tests for the interact tool (7 tests).

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "5" "Interact Tool" "7"
ensure_daemon

# ── 5.1 — interact(list_states) returns array ─────────────
begin_test "5.1" "interact(list_states) returns array" \
    "Verify list_states returns states array and count" \
    "list_states doesn't require pilot. Must always work."
run_test_5_1() {
    RESPONSE=$(call_tool "interact" '{"action":"list_states"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "states" || check_contains "$text" "count"; then
        pass "list_states returned valid response with states/count. Content: $(truncate "$text" 200)"
    else
        fail "list_states response missing 'states' or 'count'. Content: $(truncate "$text")"
    fi
}
run_test_5_1

# ── 5.2 — interact save/load/delete roundtrip ─────────────
begin_test "5.2" "interact save/load/delete state roundtrip" \
    "Full CRUD: save state, list to verify, load it, delete it, list to confirm deletion" \
    "Full CRUD roundtrip. If any step fails, state management is broken."
run_test_5_2() {
    # Step 1: Save state
    local SAVE_RESP
    SAVE_RESP=$(call_tool "interact" '{"action":"save_state","snapshot_name":"uat-state-test"}')
    if ! check_not_error "$SAVE_RESP"; then
        fail "save_state returned error. Content: $(truncate "$(extract_content_text "$SAVE_RESP")")"
        return
    fi

    # Step 2: List and verify it appears
    local LIST_RESP
    LIST_RESP=$(call_tool "interact" '{"action":"list_states"}')
    if ! check_not_error "$LIST_RESP"; then
        fail "list_states after save returned error. Content: $(truncate "$(extract_content_text "$LIST_RESP")")"
        return
    fi
    local list_text
    list_text=$(extract_content_text "$LIST_RESP")
    if ! check_contains "$list_text" "uat-state-test"; then
        fail "list_states does not contain 'uat-state-test' after save. Content: $(truncate "$list_text")"
        return
    fi

    # Step 3: Load the state
    local LOAD_RESP
    LOAD_RESP=$(call_tool "interact" '{"action":"load_state","snapshot_name":"uat-state-test"}')
    if ! check_not_error "$LOAD_RESP"; then
        fail "load_state returned error. Content: $(truncate "$(extract_content_text "$LOAD_RESP")")"
        return
    fi

    # Step 4: Delete the state
    local DEL_RESP
    DEL_RESP=$(call_tool "interact" '{"action":"delete_state","snapshot_name":"uat-state-test"}')
    if ! check_not_error "$DEL_RESP"; then
        fail "delete_state returned error. Content: $(truncate "$(extract_content_text "$DEL_RESP")")"
        return
    fi

    # Step 5: List again and verify it's gone
    local LIST2_RESP
    LIST2_RESP=$(call_tool "interact" '{"action":"list_states"}')
    if ! check_not_error "$LIST2_RESP"; then
        fail "list_states after delete returned error. Content: $(truncate "$(extract_content_text "$LIST2_RESP")")"
        return
    fi
    local list2_text
    list2_text=$(extract_content_text "$LIST2_RESP")
    if check_contains "$list2_text" "uat-state-test"; then
        fail "list_states still contains 'uat-state-test' after delete. Content: $(truncate "$list2_text")"
        return
    fi

    pass "Full CRUD roundtrip: save, list (found), load, delete, list (gone). All steps succeeded."
}
run_test_5_2

# ── 5.3 — interact(execute_js) without pilot returns error ─
begin_test "5.3" "interact(execute_js) without pilot returns error" \
    "execute_js requires pilot enabled; without extension it should return isError" \
    "Pilot-gated actions must fail clearly when pilot is off."
run_test_5_3() {
    RESPONSE=$(call_tool "interact" '{"action":"execute_js","script":"return 1+1"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for execute_js without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "execute_js correctly returned isError:true without pilot enabled."
}
run_test_5_3

# ── 5.4 — interact(navigate) without pilot returns error ──
begin_test "5.4" "interact(navigate) without pilot returns error" \
    "navigate requires pilot enabled; without extension it should return isError" \
    "Pilot-gated actions must fail clearly when pilot is off."
run_test_5_4() {
    RESPONSE=$(call_tool "interact" '{"action":"navigate","url":"https://example.com"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for navigate without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "navigate correctly returned isError:true without pilot enabled."
}
run_test_5_4

# ── 5.5 — interact(highlight) without pilot returns error ──
begin_test "5.5" "interact(highlight) without pilot returns error" \
    "highlight requires pilot enabled; without extension it should return isError" \
    "Pilot-gated actions must fail clearly when pilot is off."
run_test_5_5() {
    RESPONSE=$(call_tool "interact" '{"action":"highlight","selector":"body"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for highlight without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "highlight correctly returned isError:true without pilot enabled."
}
run_test_5_5

# ── 5.6 — interact with invalid action ────────────────────
begin_test "5.6" "interact with invalid action returns error" \
    "Send an unknown action, verify isError:true" \
    "Invalid actions must not crash."
run_test_5_6() {
    RESPONSE=$(call_tool "interact" '{"action":"fly_to_moon"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for invalid action 'fly_to_moon'. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "Invalid action 'fly_to_moon' correctly returned isError:true."
}
run_test_5_6

# ── 5.7 — interact(save_state) without name returns error ──
begin_test "5.7" "interact(save_state) without snapshot_name returns error" \
    "save_state requires snapshot_name parameter; omitting it should return isError" \
    "Required param validation."
run_test_5_7() {
    RESPONSE=$(call_tool "interact" '{"action":"save_state"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for save_state without snapshot_name. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    pass "save_state without snapshot_name correctly returned isError:true."
}
run_test_5_7

finish_category
