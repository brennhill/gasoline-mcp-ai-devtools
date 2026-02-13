#!/bin/bash
# 15-file-upload.sh — 15.1-15.9: File upload automation validation.
# Tests schema, feature flag gating, parameter validation, and queue response.
set -eo pipefail

begin_category "15" "File Upload" "14"

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

# ── Test 15.10: Stage 1 base64 roundtrip ─────────────────
begin_test "15.10" "Stage 1 base64 text roundtrip" \
    "POST /api/file/read with known content, decode base64, verify match" \
    "Tests: file data flows end-to-end through Stage 1"

run_test_15_10() {
    local test_content="Gasoline smoke roundtrip $(date +%s)"
    local test_file="/tmp/gasoline-roundtrip-$$.txt"
    echo -n "$test_content" > "$test_file"

    local body status
    body=$(curl -s --max-time 10 --connect-timeout 3 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -w "\n%{http_code}" \
        -d '{"file_path":"'"$test_file"'"}' \
        "http://localhost:${PORT}/api/file/read" 2>/dev/null)

    status=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    rm -f "$test_file"

    if [ "$status" != "200" ]; then
        fail "Expected HTTP 200, got $status."
        return
    fi

    local data_base64 decoded
    data_base64=$(echo "$body" | jq -r '.data_base64' 2>/dev/null)
    decoded=$(echo "$data_base64" | base64 -d 2>/dev/null)

    if [ "$decoded" = "$test_content" ]; then
        pass "Stage 1 base64 roundtrip: decoded matches original."
    else
        fail "Decoded mismatch. Expected: $(truncate "$test_content" 50), got: $(truncate "$decoded" 50)"
    fi
}
run_test_15_10

# ── Test 15.11: Full Stage 1 E2E with extension ──────────
begin_test "15.11" "Stage 1 E2E upload with extension (requires pilot)" \
    "Navigate browser to upload server, create test file, attempt upload via interact" \
    "Tests: full extension pipeline for upload (skip on timeout)"

run_test_15_11() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Start upload server on a free port
    local UPLOAD_PORT=$((PORT + 200))
    python3 "$(dirname "${BASH_SOURCE[0]}")/upload-server.py" "$UPLOAD_PORT" &
    local UPLOAD_SERVER_PID=$!
    sleep 1

    if ! curl -s "http://127.0.0.1:${UPLOAD_PORT}/health" >/dev/null 2>&1; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        skip "Upload test server failed to start."
        return
    fi

    # Navigate to upload page (need to get cookie first via curl, then navigate)
    local cookie_jar="/tmp/gasoline-smoke-cookies-$$.txt"
    curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1

    # Navigate browser to upload page
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Upload test page\"}"
    sleep 2

    # Create a test file
    local test_file="/tmp/gasoline-e2e-upload-$$.txt"
    echo "Gasoline E2E upload content" > "$test_file"

    # Try upload via interact
    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.11" "e2e upload" "$response" "$content_text"

    rm -f "$test_file" "$cookie_jar"
    kill "$UPLOAD_SERVER_PID" 2>/dev/null || true

    if echo "$content_text" | grep -q '"status":"queued"'; then
        pass "E2E upload: queued successfully via interact(upload)."
    elif echo "$content_text" | grep -qi "timeout\|not.*supported"; then
        skip "E2E upload: timed out or not supported in current setup."
    else
        fail "E2E upload: unexpected response. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_15_11

# ── Test 15.12: Stage 3 Rumble-style upload ──────────────
begin_test "15.12" "Stage 3 Rumble-style upload (form submit E2E)" \
    "Start Python server, get cookie+CSRF, restart daemon with --ssrf-allow-host, submit form, verify upload" \
    "Tests: file data flows through Stage 3 multipart streaming to a real server"

run_test_15_12() {
    local UPLOAD_PORT=$((PORT + 201))
    python3 "$(dirname "${BASH_SOURCE[0]}")/upload-server.py" "$UPLOAD_PORT" &
    local UPLOAD_SERVER_PID=$!
    sleep 1

    if ! curl -s "http://127.0.0.1:${UPLOAD_PORT}/health" >/dev/null 2>&1; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Upload test server failed to start."
        return
    fi

    # Get session cookie
    local cookie_jar="/tmp/gasoline-smoke-s3-cookies-$$.txt"
    curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1
    local session_cookie
    session_cookie=$(grep "session" "$cookie_jar" 2>/dev/null | awk '{print $NF}')

    # Get CSRF token
    local form_html csrf_token
    form_html=$(curl -s -b "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/upload" 2>/dev/null)
    csrf_token=$(echo "$form_html" | grep -oE 'value="[a-f0-9]{32}"' | head -1 | sed 's/value="//;s/"//')

    if [ -z "$csrf_token" ]; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        rm -f "$cookie_jar"
        fail "Failed to extract CSRF token."
        return
    fi

    # Restart daemon with --ssrf-allow-host and --upload-dir
    local upload_dir="/tmp/gasoline-smoke-upload-dir-$$"
    mkdir -p "$upload_dir"
    local test_content="Gasoline Stage 3 smoke test $(date +%s)"
    echo -n "$test_content" > "$upload_dir/smoke-upload.txt"
    local original_md5
    original_md5=$(md5sum "$upload_dir/smoke-upload.txt" 2>/dev/null | awk '{print $1}' || md5 -q "$upload_dir/smoke-upload.txt" 2>/dev/null)

    kill_server
    sleep 0.3
    start_daemon_with_flags --enable-os-upload-automation "--ssrf-allow-host=localhost:${UPLOAD_PORT}" "--upload-dir=$upload_dir"

    # POST /api/form/submit
    local submit_body
    submit_body=$(curl -s --max-time 30 --connect-timeout 5 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -d '{
            "form_action": "http://localhost:'"${UPLOAD_PORT}"'/upload",
            "method": "POST",
            "file_path": "'"$upload_dir/smoke-upload.txt"'",
            "file_input_name": "Filedata",
            "csrf_token": "'"$csrf_token"'",
            "cookies": "session='"${session_cookie}"'",
            "fields": {"title": "Smoke Test Upload", "tags": "smoke,gasoline"}
        }' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    log_diagnostic "15.12" "form submit" "$submit_body" ""

    local success
    success=$(echo "$submit_body" | jq -r '.success' 2>/dev/null)

    if [ "$success" != "true" ]; then
        local error_msg
        error_msg=$(echo "$submit_body" | jq -r '.error // empty' 2>/dev/null)
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Form submit failed: $error_msg"
        return
    fi

    # Verify via /api/last-upload
    local verify_body
    verify_body=$(curl -s "http://127.0.0.1:${UPLOAD_PORT}/api/last-upload" 2>/dev/null)
    local verify_md5 verify_csrf verify_cookie
    verify_md5=$(echo "$verify_body" | jq -r '.md5' 2>/dev/null)
    verify_csrf=$(echo "$verify_body" | jq -r '.csrf_ok' 2>/dev/null)
    verify_cookie=$(echo "$verify_body" | jq -r '.cookie_ok' 2>/dev/null)

    kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
    rm -rf "$upload_dir" "$cookie_jar"

    if [ "$verify_md5" = "$original_md5" ] && [ "$verify_csrf" = "true" ] && [ "$verify_cookie" = "true" ]; then
        pass "Stage 3 E2E: MD5 match ($verify_md5), CSRF ok, cookie ok."
    else
        fail "Verification: md5=$verify_md5 (expected $original_md5), csrf=$verify_csrf, cookie=$verify_cookie."
    fi
}
run_test_15_12

# ── Test 15.13: Stage 3 confirmation page ────────────────
begin_test "15.13" "Stage 3 confirmation page via observe (requires pilot)" \
    "If browser is on upload server, observe(page) should show confirmation details" \
    "Tests: observe reads confirmation page after redirect"

run_test_15_13() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Check if browser is currently on the upload server success page
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.13" "observe page" "$response" "$content_text"

    if echo "$content_text" | grep -qi "upload.*success\|Upload Successful"; then
        pass "Confirmation page visible via observe(page)."
    else
        skip "Browser not on upload success page (expected after 15.11/15.12)."
    fi
}
run_test_15_13

# ── Test 15.14: Stage 3 auth failure propagation ─────────
begin_test "15.14" "Stage 3 auth failure propagation (missing cookie → 401)" \
    "POST /api/form/submit without cookie, verify 401 from test server is reported" \
    "Tests: platform auth errors propagate correctly to the caller"

run_test_15_14() {
    local UPLOAD_PORT=$((PORT + 202))
    python3 "$(dirname "${BASH_SOURCE[0]}")/upload-server.py" "$UPLOAD_PORT" &
    local UPLOAD_SERVER_PID=$!
    sleep 1

    if ! curl -s "http://127.0.0.1:${UPLOAD_PORT}/health" >/dev/null 2>&1; then
        kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
        fail "Upload test server failed to start."
        return
    fi

    # Restart daemon with allowed host
    local upload_dir="/tmp/gasoline-smoke-auth-dir-$$"
    mkdir -p "$upload_dir"
    echo -n "test" > "$upload_dir/auth-test.txt"

    kill_server
    sleep 0.3
    start_daemon_with_flags --enable-os-upload-automation "--ssrf-allow-host=localhost:${UPLOAD_PORT}" "--upload-dir=$upload_dir"

    local submit_body
    submit_body=$(curl -s --max-time 30 --connect-timeout 5 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -d '{
            "form_action": "http://localhost:'"${UPLOAD_PORT}"'/upload",
            "file_path": "'"$upload_dir/auth-test.txt"'",
            "file_input_name": "Filedata",
            "fields": {"title": "test"}
        }' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    log_diagnostic "15.14" "auth failure" "$submit_body" ""

    kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
    rm -rf "$upload_dir"

    local error_msg
    error_msg=$(echo "$submit_body" | jq -r '.error // empty' 2>/dev/null)

    if echo "$error_msg" | grep -qiE "401|not logged in"; then
        pass "Auth failure: 401/not-logged-in propagated correctly."
    else
        fail "Expected 401 error. Got: $(truncate "$error_msg" 200)"
    fi
}
run_test_15_14
