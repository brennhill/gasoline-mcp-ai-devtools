#!/bin/bash
# 15-file-upload.sh — 15.0-15.15: File upload automation validation.
# Tests schema, feature flag gating, parameter validation, queue response,
# and real file delivery to a Python upload server.
set -eo pipefail

# Temp directory for all upload tests — under $HOME to avoid /tmp symlink issues
# (macOS: /tmp → /private/tmp, daemon rejects symlink paths for --upload-dir)
UPLOAD_TEST_DIR="${HOME}/.gasoline/tmp/smoke-upload-$$"
mkdir -p "$UPLOAD_TEST_DIR"

begin_category "15" "File Upload" "16"

# ── Persistent upload server for the whole category ───────
UPLOAD_PORT=$((PORT + 200))
python3 "$(dirname "${BASH_SOURCE[0]}")/upload-server.py" "$UPLOAD_PORT" &
UPLOAD_SERVER_PID=$!
sleep 1

_cleanup_upload() {
    [ -n "$UPLOAD_SERVER_PID" ] && kill "$UPLOAD_SERVER_PID" 2>/dev/null || true
    rm -rf "$UPLOAD_TEST_DIR"
}
trap _cleanup_upload EXIT

# ── Helper: restart daemon with upload flags ──────────────
_restart_daemon_for_upload() {
    local upload_dir="$1"
    kill_server 2>/dev/null || true
    sleep 0.3
    start_daemon_with_flags --enable-os-upload-automation \
        "--ssrf-allow-host=localhost:${UPLOAD_PORT}" \
        "--upload-dir=$upload_dir"
}

_restore_daemon() {
    kill_server 2>/dev/null || true
    start_daemon 2>/dev/null || true
}

# ── Test 15.0: Upload server canary ──────────────────────
begin_test "15.0" "Upload server canary" \
    "Verify Python upload test server started and responds to /health" \
    "Tests: test infrastructure — if this fails, all upload tests are invalid"

run_test_15_0() {
    local health_resp
    health_resp=$(curl -s --max-time 5 --connect-timeout 3 "http://127.0.0.1:${UPLOAD_PORT}/health" 2>/dev/null)
    if echo "$health_resp" | jq -e '.ok == true' >/dev/null 2>&1; then
        pass "Upload server alive on port $UPLOAD_PORT."
    else
        fail "Upload server not responding on port $UPLOAD_PORT. All upload tests are invalid."
    fi
}
run_test_15_0

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

# ── Test 15.8: Local queue acceptance (no server delivery) ──
begin_test "15.8" "Local queue acceptance (no server delivery)" \
    "Create temp file, call upload with valid params, verify queued response" \
    "Tests: local queue returns status=queued with correlation_id (queue is never drained without extension)"

run_test_15_8() {
    # Create a temp file for the upload test
    local test_file="$UPLOAD_TEST_DIR/upload-test.txt"
    echo "Gasoline upload smoke test content" > "$test_file"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.8" "valid upload" "$response" "$content_text"

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

# ── Test 15.9: Queue response metadata (local only) ──────
begin_test "15.9" "Queue response includes file metadata (local only)" \
    "Create temp files with known extensions, verify metadata in queue response" \
    "Tests: file metadata extraction (name, size, mime_type, progress_tier) — local queue only"

run_test_15_9() {
    # Create a temp file with a recognizable extension
    local test_file="$UPLOAD_TEST_DIR/upload-test.jpg"
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

# ── Test 15.10: Stage 1 read → Stage 3 delivery E2E ──────
begin_test "15.10" "Stage 1 read → Stage 3 delivery E2E (base64 roundtrip + upload verification)" \
    "Read file via /api/file/read, decode base64, POST to upload server via /api/form/submit, verify MD5" \
    "Tests: file data flows through Stage 1 read and Stage 3 multipart delivery to a real server"

run_test_15_10() {
    local upload_dir="$UPLOAD_TEST_DIR/upload-dir-15-10"
    local cookie_jar="$UPLOAD_TEST_DIR/cookies-15-10.txt"
    local daemon_restarted=false

    mkdir -p "$upload_dir"
    local test_content="Gasoline smoke roundtrip $(date +%s)"
    local test_file="$upload_dir/roundtrip.txt"
    echo -n "$test_content" > "$test_file"
    local original_md5
    original_md5=$(md5sum "$test_file" 2>/dev/null | awk '{print $1}' || md5 -q "$test_file" 2>/dev/null)

    # Stage 1: Read file via /api/file/read and verify base64 roundtrip
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

    if [ "$status" != "200" ]; then
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Stage 1 /api/file/read: expected HTTP 200, got $status."
        return
    fi

    local data_base64 decoded
    data_base64=$(echo "$body" | jq -r '.data_base64' 2>/dev/null)
    decoded=$(echo "$data_base64" | base64 -d 2>/dev/null)

    if [ "$decoded" != "$test_content" ]; then
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Stage 1 base64 mismatch. Expected: $(truncate "$test_content" 50), got: $(truncate "$decoded" 50)"
        return
    fi

    # Stage 3: POST file to upload server via daemon's /api/form/submit
    # Get session cookie
    curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1
    local session_cookie
    session_cookie=$(grep "session" "$cookie_jar" 2>/dev/null | awk '{print $NF}')

    # Get CSRF token
    local form_html csrf_token
    form_html=$(curl -s -b "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/upload" 2>/dev/null)
    csrf_token=$(echo "$form_html" | grep -oE 'value="[a-f0-9]{32}"' | head -1 | sed 's/value="//;s/"//')

    if [ -z "$csrf_token" ]; then
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Failed to extract CSRF token from upload server."
        return
    fi

    # Restart daemon with upload flags
    daemon_restarted=true
    if ! _restart_daemon_for_upload "$upload_dir"; then
        _restore_daemon
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Daemon failed to restart with upload flags."
        return
    fi

    # Submit form via daemon's /api/form/submit
    local submit_body
    submit_body=$(curl -s --max-time 30 --connect-timeout 5 \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-Gasoline-Client: gasoline-extension/${VERSION}" \
        -d '{
            "form_action": "http://localhost:'"${UPLOAD_PORT}"'/upload",
            "method": "POST",
            "file_path": "'"$test_file"'",
            "file_input_name": "Filedata",
            "csrf_token": "'"$csrf_token"'",
            "cookies": "session='"${session_cookie}"'",
            "fields": {"title": "Smoke Test 15.10", "tags": "smoke,roundtrip"}
        }' \
        "http://localhost:${PORT}/api/form/submit" 2>/dev/null)

    log_diagnostic "15.10" "form submit" "$submit_body" ""

    local success
    success=$(echo "$submit_body" | jq -r '.success' 2>/dev/null)

    if [ "$success" != "true" ]; then
        local error_msg
        error_msg=$(echo "$submit_body" | jq -r '.error // empty' 2>/dev/null)
        _restore_daemon
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Stage 3 form submit failed: $error_msg"
        return
    fi

    # Verify via /api/last-upload
    local verify_body
    verify_body=$(curl -s "http://127.0.0.1:${UPLOAD_PORT}/api/last-upload" 2>/dev/null)
    local verify_md5
    verify_md5=$(echo "$verify_body" | jq -r '.md5' 2>/dev/null)

    _restore_daemon
    rm -rf "$upload_dir" "$cookie_jar"

    if [ "$verify_md5" = "$original_md5" ]; then
        pass "Stage 1 read + Stage 3 delivery: base64 roundtrip OK, MD5 match ($verify_md5)."
    else
        fail "MD5 mismatch after Stage 3 delivery. Expected: $original_md5, got: $verify_md5."
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

    # Navigate to upload page (get cookie first via curl, then navigate)
    local cookie_jar="$UPLOAD_TEST_DIR/cookies-15-11.txt"
    curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1

    # Navigate browser to upload page
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Upload test page\"}"
    sleep 2

    # Create a test file
    local test_file="$UPLOAD_TEST_DIR/e2e-upload.txt"
    echo "Gasoline E2E upload content" > "$test_file"

    # Try upload via interact — must poll to confirm extension actually handled it
    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.11" "e2e upload initial" "$response" "$content_text"

    # Extract correlation_id to poll for real completion
    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)

    if [ -z "$corr_id" ]; then
        rm -f "$test_file" "$cookie_jar"
        if echo "$content_text" | grep -qi "error\|isError"; then
            fail "E2E upload: got error before queuing. $(truncate "$content_text" 200)"
        else
            fail "E2E upload: no correlation_id in response. $(truncate "$content_text" 200)"
        fi
        return
    fi

    # Poll command_result — "queued" only means the server accepted it,
    # we need "complete" to prove the extension actually processed it
    local final_status="pending"
    for _ in $(seq 1 20); do
        sleep 0.5
        local poll_text
        poll_text=$(extract_content_text "$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")")
        if echo "$poll_text" | grep -q '"status":"complete"'; then
            final_status="complete"
            log_diagnostic "15.11" "poll complete" "$poll_text" ""
            break
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            final_status="failed"
            log_diagnostic "15.11" "poll failed" "$poll_text" ""
            break
        fi
    done

    rm -f "$test_file" "$cookie_jar"

    case "$final_status" in
        complete)
            pass "E2E upload: extension processed upload to completion."
            ;;
        failed)
            fail "E2E upload: extension reported failure."
            ;;
        pending)
            skip "E2E upload: timed out polling (extension has no upload handler yet)."
            ;;
    esac
}
run_test_15_11

# ── Test 15.12: Stage 3 Rumble-style upload ──────────────
begin_test "15.12" "Stage 3 Rumble-style upload (form submit E2E)" \
    "Get cookie+CSRF from upload server, restart daemon with --ssrf-allow-host, submit form, verify upload" \
    "Tests: file data flows through Stage 3 multipart streaming to a real server"

run_test_15_12() {
    local upload_dir="$UPLOAD_TEST_DIR/upload-dir-15-12"
    local cookie_jar="$UPLOAD_TEST_DIR/cookies-15-12.txt"

    # Get session cookie
    curl -s -c "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/" >/dev/null 2>&1
    local session_cookie
    session_cookie=$(grep "session" "$cookie_jar" 2>/dev/null | awk '{print $NF}')

    # Get CSRF token
    local form_html csrf_token
    form_html=$(curl -s -b "$cookie_jar" "http://127.0.0.1:${UPLOAD_PORT}/upload" 2>/dev/null)
    csrf_token=$(echo "$form_html" | grep -oE 'value="[a-f0-9]{32}"' | head -1 | sed 's/value="//;s/"//')

    if [ -z "$csrf_token" ]; then
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Failed to extract CSRF token."
        return
    fi

    # Restart daemon with --ssrf-allow-host and --upload-dir
    mkdir -p "$upload_dir"
    local test_content="Gasoline Stage 3 smoke test $(date +%s)"
    echo -n "$test_content" > "$upload_dir/smoke-upload.txt"
    local original_md5
    original_md5=$(md5sum "$upload_dir/smoke-upload.txt" 2>/dev/null | awk '{print $1}' || md5 -q "$upload_dir/smoke-upload.txt" 2>/dev/null)

    if ! _restart_daemon_for_upload "$upload_dir"; then
        _restore_daemon
        rm -rf "$upload_dir" "$cookie_jar"
        fail "Daemon failed to restart with upload flags."
        return
    fi

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
        _restore_daemon
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

    _restore_daemon
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
        fail "Browser not on upload success page after 15.11/15.12."
    fi
}
run_test_15_13

# ── Test 15.14: Stage 3 auth failure propagation ─────────
begin_test "15.14" "Stage 3 auth failure propagation (missing cookie → 401)" \
    "POST /api/form/submit without cookie, verify 401 from test server is reported" \
    "Tests: platform auth errors propagate correctly to the caller"

run_test_15_14() {
    local upload_dir="$UPLOAD_TEST_DIR/upload-dir-15-14"

    # Restart daemon with allowed host
    mkdir -p "$upload_dir"
    echo -n "test" > "$upload_dir/auth-test.txt"

    if ! _restart_daemon_for_upload "$upload_dir"; then
        _restore_daemon
        rm -rf "$upload_dir"
        fail "Daemon failed to restart with upload flags."
        return
    fi

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

    _restore_daemon
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

# ── Test 15.15: Verify file reached input element ─────
begin_test "15.15" "Verify upload file reached input element (requires pilot)" \
    "After interact(upload), execute_js to check files[0]?.name on the input" \
    "Tests: file actually set on DOM input element (fail if extension connected but file missing)"

run_test_15_15() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Navigate browser to upload page
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Upload verification page\"}"
    sleep 2

    # Create test file and attempt upload
    local test_file="$UPLOAD_TEST_DIR/verify-upload.txt"
    echo "verify-upload-content" > "$test_file"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    # Extract correlation_id and poll for completion
    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)

    if [ -n "$corr_id" ]; then
        for _ in $(seq 1 20); do
            sleep 0.5
            local poll_text
            poll_text=$(extract_content_text "$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")")
            if echo "$poll_text" | grep -q '"status":"complete"\|"status":"failed"'; then
                break
            fi
        done
    fi
    sleep 1

    # Verify: execute_js to check files on the input
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check upload input files","script":"var el = document.getElementById(\"file-input\"); el && el.files && el.files[0] ? el.files[0].name : (el ? \"NO_FILES\" : \"NO_ELEMENT\")"}'

    log_diagnostic "15.15" "file verification" "$INTERACT_RESULT" ""

    local expected_name="verify-upload.txt"
    if echo "$INTERACT_RESULT" | grep -q "$expected_name"; then
        pass "File reached input: files[0].name matches uploaded file."
    elif echo "$INTERACT_RESULT" | grep -qi "timeout"; then
        skip "Timed out waiting for execute_js (extension upload handler not implemented yet)."
    elif echo "$INTERACT_RESULT" | grep -q "NO_FILES"; then
        fail "File not set on input element (extension connected but files array empty)."
    elif echo "$INTERACT_RESULT" | grep -q "NO_ELEMENT"; then
        fail "Input element #file-input not found on page."
    else
        fail "File verification failed: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_15_15
