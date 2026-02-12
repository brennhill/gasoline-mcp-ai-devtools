#!/bin/bash
# cat-23-draw-mode.sh — UAT tests for Draw Mode (8 tests).
# Tests draw_mode_start (interact), annotations/annotation_detail (analyze),
# and POST /draw-mode/complete HTTP endpoint.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "23" "Draw Mode" "16"
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

# ── 23.9 — POST /draw-mode/complete with session_name ──────────
begin_test "23.9" "POST /draw-mode/complete with session_name stores named session" \
    "POST with session_name field; verify response includes session_name acknowledgment" \
    "Named sessions allow multi-page annotation workflows."
run_test_23_9() {
    local payload='{"screenshot_data_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg==","annotations":[{"id":"uat_sn_001","text":"fix nav spacing","element_summary":"nav.main Links","correlation_id":"uat_sn_d001","rect":{"x":0,"y":0,"width":400,"height":60},"page_url":"https://example.com/home","timestamp":1700000000000}],"element_details":{"uat_sn_d001":{"selector":"nav.main","tag":"nav","text_content":"Links","classes":["main"]}},"page_url":"https://example.com/home","tab_id":8888,"session_name":"uat-multi-page"}'

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
        fail "POST /draw-mode/complete with session_name returned HTTP $status. Body: $(truncate "$body")"
        return
    fi

    local resp_status ann_count
    resp_status=$(echo "$body" | jq -r '.status' 2>/dev/null)
    ann_count=$(echo "$body" | jq -r '.annotation_count' 2>/dev/null)

    if [ "$resp_status" != "stored" ] || [ "$ann_count" != "1" ]; then
        fail "Expected stored+1, got status=$resp_status, count=$ann_count"
        return
    fi
    pass "POST /draw-mode/complete with session_name=uat-multi-page returned 200, stored 1 annotation."
}
run_test_23_9

# ── 23.10 — analyze(annotations) with session param ───────────
begin_test "23.10" "analyze(annotations) with session param retrieves named session" \
    "After POST with session_name, analyze(annotations, session=name) should return data" \
    "Named session retrieval via MCP."
run_test_23_10() {
    RESPONSE=$(call_tool "analyze" '{"what":"annotations","session":"uat-multi-page"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "analyze(annotations, session=uat-multi-page) returned error. Content: $(truncate "$text")"
        return
    fi

    if check_contains "$text" "fix nav spacing\|uat_sn_001\|uat-multi-page"; then
        pass "analyze(annotations, session=uat-multi-page) returned annotation data."
    else
        fail "Expected annotation data in response. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_10

# ── 23.11 — generate(visual_test) format in schema ────────────
begin_test "23.11" "visual_test format in generate schema" \
    "Verify tools/list includes visual_test in generate format enum" \
    "Schema correctness for new generate formats."
run_test_23_11() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    local formats
    formats=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "generate")
        | .inputSchema.properties.format.enum[]
    ' 2>/dev/null)

    local missing=""
    for fmt in visual_test annotation_report annotation_issues; do
        if ! echo "$formats" | grep -q "^${fmt}$"; then
            missing="$missing $fmt"
        fi
    done
    if [ -n "$missing" ]; then
        fail "Missing from generate format enum:$missing"
    else
        pass "All annotation generate formats (visual_test, annotation_report, annotation_issues) found in schema."
    fi
}
run_test_23_11

# ── 23.12 — generate(visual_test) produces Playwright test ────
begin_test "23.12" "generate(visual_test) produces Playwright test code" \
    "After POST with annotations, generate(visual_test) should return Playwright test" \
    "Core generate artifact: Playwright test from annotations."
run_test_23_12() {
    RESPONSE=$(call_tool "generate" '{"format":"visual_test"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "generate(visual_test) returned error. Content: $(truncate "$text")"
        return
    fi

    if check_contains "$text" "test(\|page.goto\|playwright"; then
        pass "generate(visual_test) produced Playwright test code."
    else
        fail "Expected Playwright test output. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_12

# ── 23.13 — generate(annotation_report) produces markdown ─────
begin_test "23.13" "generate(annotation_report) produces markdown report" \
    "generate(annotation_report) should return markdown with annotation details" \
    "Core generate artifact: markdown report from annotations."
run_test_23_13() {
    RESPONSE=$(call_tool "generate" '{"format":"annotation_report"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "generate(annotation_report) returned error. Content: $(truncate "$text")"
        return
    fi

    if check_contains "$text" "#\|annotation\|Annotation"; then
        pass "generate(annotation_report) produced markdown report."
    else
        fail "Expected markdown report output. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_13

# ── 23.14 — generate(annotation_issues) produces JSON issues ──
begin_test "23.14" "generate(annotation_issues) produces structured issue list" \
    "generate(annotation_issues) should return JSON with issues array" \
    "Core generate artifact: structured issues from annotations."
run_test_23_14() {
    RESPONSE=$(call_tool "generate" '{"format":"annotation_issues"}')
    local text
    text=$(extract_content_text "$RESPONSE")

    if check_is_error "$RESPONSE"; then
        fail "generate(annotation_issues) returned error. Content: $(truncate "$text")"
        return
    fi

    if check_contains "$text" "issues\|issue"; then
        pass "generate(annotation_issues) produced structured issue list."
    else
        fail "Expected issues in output. Content: $(truncate "$text" 300)"
    fi
}
run_test_23_14

# ── 23.15 — session param in generate schema ──────────────────
begin_test "23.15" "session param in generate schema" \
    "Verify generate tool has 'session' param in tools/list schema" \
    "Schema correctness: session param enables multi-page generate."
run_test_23_15() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    local has_session
    has_session=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "generate")
        | .inputSchema.properties
        | has("session")
    ' 2>/dev/null)
    if [ "$has_session" = "true" ]; then
        pass "generate tool has 'session' param in schema."
    else
        fail "generate tool missing 'session' param in schema."
    fi
}
run_test_23_15

# ── 23.16 — wait param in analyze schema ──────────────────────
begin_test "23.16" "wait param in analyze schema" \
    "Verify analyze tool has 'wait' boolean param in tools/list schema" \
    "Schema correctness: wait param enables blocking annotation retrieval."
run_test_23_16() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    local has_wait wait_type
    has_wait=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "analyze")
        | .inputSchema.properties
        | has("wait")
    ' 2>/dev/null)
    wait_type=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "analyze")
        | .inputSchema.properties.wait.type
    ' 2>/dev/null)
    if [ "$has_wait" = "true" ] && [ "$wait_type" = "boolean" ]; then
        pass "analyze tool has 'wait' boolean param in schema."
    else
        fail "analyze tool missing or wrong type for 'wait' param. has=$has_wait type=$wait_type"
    fi
}
run_test_23_16

finish_category
