#!/bin/bash
# cat-23-draw-mode.sh — UAT tests for Draw Mode (8 tests).
# Tests draw_mode_start (interact), annotations/annotation_detail (analyze),
# and POST /draw-mode/complete HTTP endpoint.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "23" "Draw Mode" "8"
ensure_daemon

# ── 23.1 — interact(draw_mode_start) without pilot returns error ──
begin_test "23.1" "interact(draw_mode_start) without pilot returns error" \
    "draw_mode_start requires pilot enabled; without extension it should return isError" \
    "Pilot-gated action must fail clearly when pilot is off."
run_test_23_1() {
    RESPONSE=$(call_tool "interact" '{"action":"draw_mode_start"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true for draw_mode_start without pilot. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "pilot\|Pilot\|disabled"; then
        pass "draw_mode_start correctly returned pilot disabled error."
    else
        fail "draw_mode_start error should mention pilot. Content: $(truncate "$text")"
    fi
}
run_test_23_1

# ── 23.2 — draw_mode_start appears in tools/list schema ──────
begin_test "23.2" "draw_mode_start in tools/list interact action enum" \
    "Verify tools/list includes draw_mode_start in interact action enum" \
    "Schema correctness: LLMs discover available actions via tools/list."
run_test_23_2() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    # Extract interact tool's action enum
    local has_draw_mode
    has_draw_mode=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "interact")
        | .inputSchema.properties.action.enum[]
        | select(. == "draw_mode_start")
    ' 2>/dev/null)
    if [ "$has_draw_mode" = "draw_mode_start" ]; then
        pass "draw_mode_start found in interact action enum."
    else
        fail "draw_mode_start NOT found in interact action enum. Response: $(truncate "$TOOLS_RESP" 300)"
    fi
}
run_test_23_2

# ── 23.3 — annotations and annotation_detail in analyze schema ──
begin_test "23.3" "annotations/annotation_detail in analyze schema" \
    "Verify tools/list includes annotations and annotation_detail in analyze what enum" \
    "Schema correctness: LLMs discover draw mode analysis via tools/list."
run_test_23_3() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    local has_annotations has_detail
    has_annotations=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "analyze")
        | .inputSchema.properties.what.enum[]
        | select(. == "annotations")
    ' 2>/dev/null)
    has_detail=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "analyze")
        | .inputSchema.properties.what.enum[]
        | select(. == "annotation_detail")
    ' 2>/dev/null)
    local failed=""
    if [ "$has_annotations" != "annotations" ]; then
        failed="$failed annotations"
    fi
    if [ "$has_detail" != "annotation_detail" ]; then
        failed="$failed annotation_detail"
    fi
    if [ -n "$failed" ]; then
        fail "Missing from analyze what enum:$failed"
    else
        pass "Both 'annotations' and 'annotation_detail' found in analyze what enum."
    fi
}
run_test_23_3

# ── 23.4 — analyze(annotations) with no session ─────────────
begin_test "23.4" "analyze(annotations) with no session returns no-session response" \
    "Call analyze({what: annotations}) with no draw mode session; expect structured response" \
    "Clean state: annotations query without session must not crash."
run_test_23_4() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotations"}')
    if check_is_error "$RESPONSE"; then
        # isError is acceptable if the message explains no session
        local text
        text=$(extract_content_text "$RESPONSE")
        if check_contains "$text" "annotation\|session\|draw"; then
            pass "analyze(annotations) with no session returned error with helpful message. Content: $(truncate "$text" 200)"
        else
            fail "analyze(annotations) returned error without helpful message. Content: $(truncate "$text" 200)"
        fi
        return
    fi
    # Non-error response is also acceptable (could return empty annotations)
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "annotation\|count\|No annotation"; then
        pass "analyze(annotations) with no session returned clean response. Content: $(truncate "$text" 200)"
    else
        fail "analyze(annotations) response missing expected fields. Content: $(truncate "$text" 200)"
    fi
}
run_test_23_4

# ── 23.5 — analyze(annotation_detail) missing correlation_id ──
begin_test "23.5" "analyze(annotation_detail) missing correlation_id returns error" \
    "annotation_detail requires correlation_id param; omitting it should return isError" \
    "Required param validation."
run_test_23_5() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotation_detail"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for annotation_detail without correlation_id. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "correlation_id"; then
        pass "annotation_detail without correlation_id correctly returned error mentioning 'correlation_id'."
    else
        fail "annotation_detail error should mention 'correlation_id'. Content: $(truncate "$text")"
    fi
}
run_test_23_5

# ── 23.6 — analyze(annotation_detail) nonexistent ID ─────────
begin_test "23.6" "analyze(annotation_detail) nonexistent ID returns not-found" \
    "annotation_detail with non-existent correlation_id should return isError with not-found message" \
    "Graceful handling of invalid correlation IDs."
run_test_23_6() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotation_detail","correlation_id":"nonexistent_uat_id"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for nonexistent correlation_id. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "not found\|expired\|no detail"; then
        pass "annotation_detail with nonexistent ID returned appropriate not-found error."
    else
        fail "Expected not-found/expired message. Content: $(truncate "$text")"
    fi
}
run_test_23_6

# ── 23.7 — POST /draw-mode/complete end-to-end ───────────────
begin_test "23.7" "POST /draw-mode/complete stores annotations" \
    "POST valid draw mode completion payload via HTTP; verify 200 + stored response" \
    "Core integration: extension sends draw mode results to server."
run_test_23_7() {
    # Build payload with inline base64 (avoid sed which chokes on base64 chars)
    local payload='{"screenshot_data_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg==","annotations":[{"id":"uat_ann_001","text":"make this darker","element_summary":"button.primary Submit","correlation_id":"uat_detail_001","rect":{"x":100,"y":200,"width":150,"height":50},"page_url":"https://example.com","timestamp":1700000000000}],"element_details":{"uat_detail_001":{"selector":"button#submit-btn","tag":"button","text_content":"Submit","classes":["primary"],"computed_styles":{"background-color":"rgb(59, 130, 246)"},"bounding_rect":{"x":100,"y":200,"width":150,"height":50}}},"page_url":"https://example.com","tab_id":9999}'

    local status body
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension-uat" \
        -w "\n%{http_code}" \
        -d "$payload" \
        "http://localhost:${PORT}/draw-mode/complete" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "POST /draw-mode/complete returned HTTP $status. Body: $(truncate "$body")"
        return
    fi

    # Verify response JSON
    if ! echo "$body" | jq -e '.' >/dev/null 2>&1; then
        fail "Response is not valid JSON. Body: $(truncate "$body")"
        return
    fi

    local resp_status ann_count
    resp_status=$(echo "$body" | jq -r '.status' 2>/dev/null)
    ann_count=$(echo "$body" | jq -r '.annotation_count' 2>/dev/null)

    if [ "$resp_status" != "stored" ]; then
        fail "Expected status 'stored', got '$resp_status'. Body: $(truncate "$body")"
        return
    fi
    if [ "$ann_count" != "1" ]; then
        fail "Expected annotation_count 1, got '$ann_count'. Body: $(truncate "$body")"
        return
    fi

    pass "POST /draw-mode/complete returned 200 with status=stored, annotation_count=1."
}
run_test_23_7

# ── 23.8 — POST /draw-mode/complete invalid JSON returns 400 ──
begin_test "23.8" "POST /draw-mode/complete invalid JSON returns 400" \
    "Send malformed JSON to /draw-mode/complete; verify 400 Bad Request" \
    "Error handling: invalid input must not crash server."
run_test_23_8() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension-uat" \
        -o /dev/null -w "%{http_code}" \
        -d '{invalid json' \
        "http://localhost:${PORT}/draw-mode/complete" 2>/dev/null)

    if [ "$status" = "400" ]; then
        pass "POST /draw-mode/complete with invalid JSON returned HTTP 400."
    else
        fail "Expected HTTP 400 for invalid JSON, got HTTP $status."
    fi
}
run_test_23_8

finish_category
