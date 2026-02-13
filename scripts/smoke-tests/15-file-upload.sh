#!/bin/bash
# 15-file-upload.sh — 15.1-15.9: File upload automation validation.
# Tests schema, feature flag gating, parameter validation, and queue response.
set -eo pipefail

begin_category "15" "File Upload" "9"

# ── Test 15.1: Schema — upload in interact action enum ───
begin_test "15.1" "Schema: upload in interact action enum" \
    "Verify tools/list includes upload as a valid interact action" \
    "Tests: schema registration for file upload"

run_test_15_1() {
    local tools_resp
    tools_resp=$(send_mcp "{\"jsonrpc\":\"2.0\",\"id\":${MCP_ID},\"method\":\"tools/list\"}")
    if echo "$tools_resp" | jq -e '.result.tools[] | select(.name=="interact") | .inputSchema.properties.action.enum[] | select(.=="upload")' >/dev/null 2>&1; then
        pass "upload in interact action enum."
    else
        fail "upload NOT in interact action enum."
    fi
}
run_test_15_1

# ── Test 15.2: Upload works without any flag ─────────────
begin_test "15.2" "Upload works without any flag (queues successfully)" \
    "Call upload with nonexistent file, verify we get a file error (not disabled error)" \
    "Tests: basic upload always works without --enable-os-upload-automation"

run_test_15_2() {
    local response
    response=$(call_tool "interact" '{"action":"upload","selector":"#file","file_path":"/tmp/nonexistent-gasoline-test.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.2" "upload probe" "$response" "$content_text"

    if echo "$content_text" | grep -qi "not found\|File not found\|invalid_param"; then
        pass "Upload works without flag: got file validation error (not disabled)."
    elif echo "$content_text" | grep -q '"status":"queued"\|"correlation_id"'; then
        pass "Upload works without flag: queued successfully."
    else
        fail "Unexpected upload response: $(truncate "$content_text" 200)"
    fi
}
run_test_15_2

# ── Test 15.3: Missing file_path ─────────────────────────
begin_test "15.3" "Missing file_path returns clear error" \
    "Call upload without file_path, verify parameter error" \
    "Tests: required parameter validation"

run_test_15_3() {
    local response
    response=$(call_tool "interact" '{"action":"upload","selector":"#file"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.3" "missing file_path" "$response" "$content_text"

    if echo "$content_text" | grep -q "file_path"; then
        pass "Missing file_path: error mentions file_path parameter."
    else
        fail "Missing file_path: no clear error. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_3

# ── Test 15.4: Missing selector and api_endpoint ────────
begin_test "15.4" "Missing selector returns clear error" \
    "Call upload with file_path but no selector or api_endpoint" \
    "Tests: required selector/api_endpoint parameter validation"

run_test_15_4() {
    local response
    response=$(call_tool "interact" '{"action":"upload","file_path":"/tmp/test.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.4" "missing selector" "$response" "$content_text"

    if echo "$content_text" | grep -q "selector"; then
        pass "Missing selector: error mentions selector parameter."
    else
        fail "Missing selector: no clear error. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_4

# ── Test 15.5: Relative path rejected ───────────────────
begin_test "15.5" "Relative path rejected with security error" \
    "Call upload with relative file_path, verify rejection" \
    "Tests: absolute path requirement for security"

run_test_15_5() {
    local response
    response=$(call_tool "interact" '{"action":"upload","selector":"#file","file_path":"relative/path/file.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.5" "relative path" "$response" "$content_text"

    if echo "$content_text" | grep -qi "absolute\|not allowed\|path_not_allowed"; then
        pass "Relative path: rejected with security error."
    else
        fail "Relative path: not properly rejected. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_5

# ── Test 15.6: Nonexistent file rejected ────────────────
begin_test "15.6" "Nonexistent file rejected with clear error" \
    "Call upload with nonexistent absolute file_path" \
    "Tests: file existence validation before queuing"

run_test_15_6() {
    local response
    response=$(call_tool "interact" '{"action":"upload","selector":"#file","file_path":"/tmp/gasoline-nonexistent-file-12345.txt"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.6" "nonexistent file" "$response" "$content_text"

    if echo "$content_text" | grep -qi "not found\|no such file\|does not exist"; then
        pass "Nonexistent file: rejected with clear error."
    else
        fail "Nonexistent file: not properly rejected. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_6

# ── Test 15.7: Directory path rejected ──────────────────
begin_test "15.7" "Directory path rejected (not a file)" \
    "Call upload with path to a directory, verify rejection" \
    "Tests: file-vs-directory validation"

run_test_15_7() {
    local response
    response=$(call_tool "interact" '{"action":"upload","selector":"#file","file_path":"/tmp"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.7" "directory path" "$response" "$content_text"

    if echo "$content_text" | grep -qi "directory\|not a file"; then
        pass "Directory path: rejected with clear error."
    else
        fail "Directory path: not properly rejected. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_7

# ── Test 15.8: Upload queued with valid params ──────────
begin_test "15.8" "Valid upload returns queued status with correlation_id" \
    "Create temp file, call upload with valid params, verify queued response" \
    "Tests: successful upload queue and correlation_id generation"

run_test_15_8() {
    # Create a temp file for the upload test
    local test_file="/tmp/gasoline-upload-test-$$.txt"
    echo "Gasoline upload smoke test content" > "$test_file"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.8" "valid upload" "$response" "$content_text"

    # Clean up temp file
    rm -f "$test_file"

    local has_queued has_corr_id
    has_queued=$(echo "$content_text" | grep -c '"status":"queued"' || true)
    has_corr_id=$(echo "$content_text" | grep -c '"correlation_id":"upload_' || true)

    if [ "$has_queued" -gt 0 ] && [ "$has_corr_id" -gt 0 ]; then
        pass "Upload queued: status=queued, correlation_id present."
    else
        fail "Upload not properly queued: queued=$has_queued, corr_id=$has_corr_id. Got: $(truncate "$content_text" 300)"
    fi
}
run_test_15_8

# ── Test 15.9: Response includes file metadata ──────────
begin_test "15.9" "Upload response includes file name, size, and MIME type" \
    "Create temp files with known extensions, verify metadata in queue response" \
    "Tests: file metadata extraction (name, size, mime_type, progress_tier)"

run_test_15_9() {
    # Create a temp file with a recognizable extension
    local test_file="/tmp/gasoline-upload-test-$$.jpg"
    echo "fake image content for metadata test" > "$test_file"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"input[type=file]\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.9" "file metadata" "$response" "$content_text"

    rm -f "$test_file"

    local metadata_verdict
    metadata_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    has_name = 'file_name' in data and data['file_name'].endswith('.jpg')
    has_size = 'file_size' in data and isinstance(data['file_size'], (int, float)) and data['file_size'] > 0
    has_mime = 'mime_type' in data and 'image' in data.get('mime_type', '')
    has_tier = 'progress_tier' in data
    if has_name and has_size and has_mime and has_tier:
        print(f'PASS name={data[\"file_name\"]} size={data[\"file_size\"]} mime={data[\"mime_type\"]} tier={data[\"progress_tier\"]}')
    else:
        print(f'FAIL name={has_name} size={has_size} mime={has_mime} tier={has_tier} keys={list(data.keys())[:10]}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$metadata_verdict" | grep -q "^PASS"; then
        pass "File metadata correct. $metadata_verdict"
    else
        fail "File metadata incomplete. $metadata_verdict. Response: $(truncate "$content_text" 300)"
    fi
}
run_test_15_9
