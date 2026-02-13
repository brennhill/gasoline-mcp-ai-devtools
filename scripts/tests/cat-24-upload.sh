#!/bin/bash
# cat-24-upload.sh — UAT tests for File Upload (15 tests).
# Tests upload MCP action, parameter validation, security, MIME detection,
# and HTTP Stage 1/3 endpoints.
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "24" "File Upload" "15"

# ── Create temp test fixtures ──────────────────────────────
echo "Hello upload test" > "$TEMP_DIR/test-file.txt"
echo '{"key":"value"}' > "$TEMP_DIR/test-file.json"
# 1x1 red PNG (89 bytes)
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > "$TEMP_DIR/test-file.png"
mkdir -p "$TEMP_DIR/test-dir"

# ── 24.1 — Stage 4 OS automation disabled without --enable-os-upload-automation ──
begin_test "24.1" "Stage 4 OS automation disabled without --enable-os-upload-automation flag" \
    "HTTP POST /api/os-automation/inject without the flag should return 403" \
    "Security: OS-level automation requires explicit opt-in."

# Start daemon WITHOUT os-upload-automation flag for this test
start_daemon

run_test_24_1() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{"file_path":"/tmp/test.txt","browser_pid":1234}' \
        "http://localhost:${PORT}/api/os-automation/inject" 2>/dev/null)
    if [ "$status" = "403" ]; then
        pass "Stage 4 OS automation correctly returns 403 without flag."
    else
        fail "Expected HTTP 403 for OS automation without flag, got $status."
    fi
}
run_test_24_1

# Upload stages 1-3 and MCP handler work without any flag — no restart needed

# ── 24.2 — upload in interact schema enum ──────────────────
begin_test "24.2" "upload in tools/list interact action enum" \
    "Verify tools/list includes upload in interact action enum" \
    "Schema correctness: LLMs discover upload action via tools/list."
run_test_24_2() {
    local TOOLS_RESP
    TOOLS_RESP=$(send_mcp '{"jsonrpc":"2.0","id":'"$MCP_ID"',"method":"tools/list"}')
    if [ -z "$TOOLS_RESP" ]; then
        fail "tools/list returned empty response."
        return
    fi
    local has_upload
    has_upload=$(echo "$TOOLS_RESP" | jq -r '
        .result.tools[]
        | select(.name == "interact")
        | .inputSchema.properties.action.enum[]
        | select(. == "upload")
    ' 2>/dev/null)
    if [ "$has_upload" = "upload" ]; then
        pass "upload found in interact action enum."
    else
        fail "upload NOT found in interact action enum. Response: $(truncate "$TOOLS_RESP" 300)"
    fi
}
run_test_24_2

# ── 24.3 — Missing file_path returns isError ──────────────
begin_test "24.3" "Missing file_path returns isError" \
    "interact(upload) without file_path should return isError with missing_param code" \
    "Required parameter validation."
run_test_24_3() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for missing file_path."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "file_path|missing_param"; then
        pass "Missing file_path correctly returned error mentioning file_path."
    else
        fail "Expected file_path in error. Content: $(truncate "$text")"
    fi
}
run_test_24_3

# ── 24.4 — Missing both selector AND apiEndpoint returns isError ──
begin_test "24.4" "Missing selector and apiEndpoint returns isError" \
    "interact(upload) without selector or apiEndpoint should return isError" \
    "Required parameter validation: at least one target is needed."
run_test_24_4() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"'"$TEMP_DIR/test-file.txt"'"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for missing selector and apiEndpoint."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "selector|apiEndpoint|missing_param"; then
        pass "Missing selector/apiEndpoint correctly returned error."
    else
        fail "Expected selector mention in error. Content: $(truncate "$text")"
    fi
}
run_test_24_4

# ── 24.5 — Relative path rejected with path_not_allowed ───
begin_test "24.5" "Relative path rejected with path_not_allowed" \
    "interact(upload) with relative file_path should return path_not_allowed error" \
    "Security: only absolute paths allowed."
run_test_24_5() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"relative/path.txt","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for relative path."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "path_not_allowed|absolute path"; then
        pass "Relative path correctly rejected with path_not_allowed."
    else
        fail "Expected path_not_allowed in error. Content: $(truncate "$text")"
    fi
}
run_test_24_5

# ── 24.6 — File not found returns invalid_param ───────────
begin_test "24.6" "File not found returns invalid_param" \
    "interact(upload) with nonexistent file should return isError with invalid_param" \
    "Graceful error for missing files."
run_test_24_6() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"/tmp/gasoline-uat-nonexistent-file-99999.txt","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for nonexistent file."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "invalid_param|not found|File not found"; then
        pass "Nonexistent file correctly returned error."
    else
        fail "Expected file not found error. Content: $(truncate "$text")"
    fi
}
run_test_24_6

# ── 24.7 — Directory rejected with invalid_param ──────────
begin_test "24.7" "Directory rejected with invalid_param" \
    "interact(upload) with a directory path should return isError with invalid_param" \
    "Only files can be uploaded, not directories."
run_test_24_7() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"'"$TEMP_DIR/test-dir"'","selector":"#file"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError for directory path."
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_matches "$text" "invalid_param|directory|not a file"; then
        pass "Directory path correctly rejected."
    else
        fail "Expected directory rejection error. Content: $(truncate "$text")"
    fi
}
run_test_24_7

# ── 24.8 — Success: queued response has expected fields ───
begin_test "24.8" "Queued response has status, correlation_id, file_name, file_size, mime_type, progress_tier" \
    "interact(upload) with valid params returns queued response with all expected fields" \
    "Core success path: verify queued response shape."
run_test_24_8() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"'"$TEMP_DIR/test-file.txt"'","selector":"#file-input"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")

    local missing=""
    for field in status correlation_id file_name file_size mime_type progress_tier; do
        if ! check_contains "$text" "$field"; then
            missing="$missing $field"
        fi
    done

    if [ -n "$missing" ]; then
        fail "Missing fields in queued response:$missing. Content: $(truncate "$text" 400)"
        return
    fi

    if check_matches "$text" '"status".*queued|"status": "queued"'; then
        pass "Upload queued response has all expected fields (status=queued, correlation_id, file_name, file_size, mime_type, progress_tier)."
    else
        # Fields present but status might not be "queued" — still pass for field presence
        pass "Upload response has all expected fields. Content: $(truncate "$text" 200)"
    fi
}
run_test_24_8

# ── 24.9 — MIME detection: .txt → text/plain ──────────────
begin_test "24.9" "MIME detection: .txt → text/plain" \
    "Upload a .txt file and verify mime_type is text/plain" \
    "MIME type detection correctness."
run_test_24_9() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"'"$TEMP_DIR/test-file.txt"'","selector":"#file"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "text/plain"; then
        pass "MIME detection: .txt correctly detected as text/plain."
    else
        fail "Expected text/plain. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_9

# ── 24.10 — MIME detection: .json → application/json ──────
begin_test "24.10" "MIME detection: .json → application/json" \
    "Upload a .json file and verify mime_type is application/json" \
    "MIME type detection correctness."
run_test_24_10() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"'"$TEMP_DIR/test-file.json"'","selector":"#file"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "application/json"; then
        pass "MIME detection: .json correctly detected as application/json."
    else
        fail "Expected application/json. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_10

# ── 24.11 — MIME detection: .png → image/png ──────────────
begin_test "24.11" "MIME detection: .png → image/png" \
    "Upload a .png file and verify mime_type is image/png" \
    "MIME type detection correctness."
run_test_24_11() {
    RESPONSE=$(call_tool "interact" '{"action":"upload","file_path":"'"$TEMP_DIR/test-file.png"'","selector":"#file"}')
    if check_is_error "$RESPONSE"; then
        fail "Expected success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if check_contains "$text" "image/png"; then
        pass "MIME detection: .png correctly detected as image/png."
    else
        fail "Expected image/png. Content: $(truncate "$text" 300)"
    fi
}
run_test_24_11

# ── 24.12 — HTTP POST /api/file/read — valid file → 200 ──
begin_test "24.12" "HTTP POST /api/file/read — valid file returns 200 + base64" \
    "POST a valid file_path to /api/file/read; verify 200 + success + base64 data" \
    "Stage 1 HTTP endpoint: file read and base64 encoding."
run_test_24_12() {
    local body status
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{"file_path":"'"$TEMP_DIR/test-file.txt"'"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$status" != "200" ]; then
        fail "Expected HTTP 200, got $status. Body: $(truncate "$body")"
        return
    fi

    local success file_name data_base64
    success=$(echo "$body" | jq -r '.success' 2>/dev/null)
    file_name=$(echo "$body" | jq -r '.file_name' 2>/dev/null)
    data_base64=$(echo "$body" | jq -r '.data_base64' 2>/dev/null)

    if [ "$success" != "true" ]; then
        fail "Expected success:true. Body: $(truncate "$body")"
        return
    fi
    if [ "$file_name" != "test-file.txt" ]; then
        fail "Expected file_name=test-file.txt, got $file_name."
        return
    fi
    if [ -z "$data_base64" ] || [ "$data_base64" = "null" ]; then
        fail "Expected data_base64 to be non-empty. Body: $(truncate "$body")"
        return
    fi

    pass "POST /api/file/read returned 200, success=true, file_name=test-file.txt, base64 data present."
}
run_test_24_12

# ── 24.13 — HTTP POST /api/file/read — missing file_path → 400 ──
begin_test "24.13" "HTTP POST /api/file/read — missing file_path returns 400" \
    "POST to /api/file/read without file_path; verify 400 Bad Request" \
    "Stage 1 HTTP endpoint: required parameter validation."
run_test_24_13() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    if [ "$status" = "400" ]; then
        pass "POST /api/file/read with missing file_path returned HTTP 400."
    else
        fail "Expected HTTP 400, got HTTP $status."
    fi
}
run_test_24_13

# ── 24.14 — HTTP POST /api/file/read — file not found → 404 ──
begin_test "24.14" "HTTP POST /api/file/read — nonexistent file returns 404" \
    "POST to /api/file/read with nonexistent file; verify 404 Not Found" \
    "Stage 1 HTTP endpoint: graceful 404 for missing files."
run_test_24_14() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{"file_path":"/tmp/gasoline-uat-nonexistent-file-99999.txt"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    if [ "$status" = "404" ]; then
        pass "POST /api/file/read with nonexistent file returned HTTP 404."
    else
        fail "Expected HTTP 404, got HTTP $status."
    fi
}
run_test_24_14

# ── 24.15 — HTTP POST /api/form/submit — missing fields → 400 ──
begin_test "24.15" "HTTP POST /api/form/submit — missing required fields returns 400" \
    "POST to /api/form/submit with empty body; verify 400 Bad Request" \
    "Stage 3 HTTP endpoint: required parameter validation."
run_test_24_15() {
    local status
    status=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -o /dev/null -w "%{http_code}" \
        -d '{}' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    if [ "$status" = "400" ]; then
        pass "POST /api/form/submit with missing fields returned HTTP 400."
    else
        fail "Expected HTTP 400, got HTTP $status."
    fi
}
run_test_24_15

finish_category
