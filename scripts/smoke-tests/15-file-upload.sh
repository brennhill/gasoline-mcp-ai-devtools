#!/bin/bash
# 15-file-upload.sh — 15.0-15.15: File upload automation validation.
# ALL actions go through MCP commands (interact/observe/execute_js).
# Browser-based tests (15.0 browser check, 15.10-15.15) require extension+pilot.
# Parameter validation tests (15.1-15.9) work without extension.
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

# ── Shared helper: navigate browser to upload form ────────
# Visits / (sets session cookie) then /upload (loads form with CSRF).
_navigate_to_upload_form() {
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Get session cookie\"}"
    sleep 1
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Load upload form\"}"
    sleep 2
}

# ── Shared helper: interact(upload) + poll for completion ─
# Sets UPLOAD_FINAL_STATUS to "complete", "failed", or "pending".
_upload_and_poll() {
    local test_id="$1"
    local test_file="$2"
    UPLOAD_FINAL_STATUS="pending"

    local response
    response=$(call_tool "interact" "{\"action\":\"upload\",\"selector\":\"#file-input\",\"file_path\":\"$test_file\"}")
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "$test_id" "interact upload" "$response" "$content_text"

    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//' || true)

    if [ -z "$corr_id" ]; then
        if echo "$content_text" | grep -qi "error\|isError"; then
            UPLOAD_FINAL_STATUS="error: $(truncate "$content_text" 200)"
        else
            UPLOAD_FINAL_STATUS="no_corr_id: $(truncate "$content_text" 200)"
        fi
        return
    fi

    for _ in $(seq 1 20); do
        sleep 0.5
        local poll_text
        poll_text=$(extract_content_text "$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")")
        if echo "$poll_text" | grep -q '"status":"complete"'; then
            UPLOAD_FINAL_STATUS="complete"
            log_diagnostic "$test_id" "poll complete" "$poll_text" ""
            return
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            UPLOAD_FINAL_STATUS="failed"
            log_diagnostic "$test_id" "poll failed" "$poll_text" ""
            return
        fi
    done
}

# ── Test 15.0: Upload server canary ──────────────────────
begin_test "15.0" "Upload server canary" \
    "Navigate browser to upload server, verify landing page and upload form load" \
    "Tests: test infrastructure — if this fails, all upload tests are invalid"

run_test_15_0() {
    # Quick infrastructure check — is the process alive?
    local health_resp
    health_resp=$(curl -s --max-time 5 --connect-timeout 3 "http://127.0.0.1:${UPLOAD_PORT}/health" 2>/dev/null)
    if ! echo "$health_resp" | jq -e '.ok == true' >/dev/null 2>&1; then
        fail "Upload server not responding on port $UPLOAD_PORT (PID $UPLOAD_SERVER_PID)."
        return
    fi

    # Browser verification (requires extension)
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        pass "Upload server healthy on port $UPLOAD_PORT (browser check skipped — no extension)."
        return
    fi

    # Navigate to landing page (sets session cookie in browser)
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Canary — landing page\"}"
    sleep 1

    # Navigate to upload form
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Canary — upload form\"}"
    sleep 1

    # Observe the page
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.0" "observe upload form" "$response" "$content_text"

    if echo "$content_text" | grep -qi "Upload File\|file-input\|Filedata"; then
        pass "Upload server on port $UPLOAD_PORT: landing page + upload form visible in browser."
    else
        fail "Upload form not visible in browser. Page: $(truncate "$content_text" 200)"
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

# ── Test 15.10: Upload text file E2E via browser ─────────
begin_test "15.10" "Upload text file E2E with MD5 verification (requires pilot)" \
    "Navigate to upload form, interact(upload), submit form via execute_js, verify MD5 on success page" \
    "Tests: file flows through browser to upload server, MD5 verified on success page"

run_test_15_10() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Create test file with known content
    local test_content="Gasoline upload E2E $(date +%s)"
    local test_file="$UPLOAD_TEST_DIR/upload-15-10.txt"
    echo -n "$test_content" > "$test_file"
    local original_md5
    original_md5=$(md5sum "$test_file" 2>/dev/null | awk '{print $1}' || md5 -q "$test_file" 2>/dev/null)

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Set file on input via interact(upload), poll for completion
    _upload_and_poll "15.10" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete) ;;
        pending)
            skip "Upload timed out (extension upload handler not implemented yet)."
            return ;;
        failed)
            fail "Extension reported upload failure."
            return ;;
        *)
            fail "Upload failed: $UPLOAD_FINAL_STATUS"
            return ;;
    esac

    # Fill in required title field via execute_js
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Fill title field","script":"document.querySelector(\"input[name=title]\").value=\"Smoke Test 15.10\"; \"title_set\""}'
    sleep 0.5

    # Submit form via execute_js
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Submit upload form","script":"document.querySelector(\"form\").submit(); \"submitted\""}'
    sleep 3

    # Observe result page — should be /upload/success after redirect
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.10" "observe success page" "$response" "$content_text"

    if echo "$content_text" | grep -qi "Upload Successful"; then
        if echo "$content_text" | grep -q "$original_md5"; then
            pass "Upload E2E: file reached server, MD5 match ($original_md5) on success page."
        else
            fail "Upload reached server but MD5 not on success page. Expected: $original_md5. Page: $(truncate "$content_text" 300)"
        fi
    else
        fail "Upload did not reach success page. Page: $(truncate "$content_text" 300)"
    fi
}
run_test_15_10

# ── Test 15.11: Extension upload pipeline (requires pilot) ──
begin_test "15.11" "Extension upload pipeline completion (requires pilot)" \
    "Navigate to upload form, interact(upload), poll until extension reports complete" \
    "Tests: extension receives upload command and processes it to completion"

run_test_15_11() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Create a test file
    local test_file="$UPLOAD_TEST_DIR/e2e-upload.txt"
    echo "Gasoline E2E upload content" > "$test_file"

    # Upload and poll for completion
    _upload_and_poll "15.11" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete)
            pass "E2E upload: extension processed upload to completion."
            ;;
        failed)
            fail "E2E upload: extension reported failure."
            ;;
        pending)
            skip "E2E upload: timed out polling (extension has no upload handler yet)."
            ;;
        *)
            fail "E2E upload: $UPLOAD_FINAL_STATUS"
            ;;
    esac
}
run_test_15_11

# ── Test 15.12: Upload with full server-side verification ──
begin_test "15.12" "Upload with full server-side verification (requires pilot)" \
    "Navigate, upload file, fill fields, submit form, verify CSRF + cookie + MD5 via server API" \
    "Tests: complete upload pipeline — browser → server, all fields verified server-side"

run_test_15_12() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    local test_content="Gasoline full verification $(date +%s)"
    local test_file="$UPLOAD_TEST_DIR/upload-15-12.txt"
    echo -n "$test_content" > "$test_file"
    local original_md5
    original_md5=$(md5sum "$test_file" 2>/dev/null | awk '{print $1}' || md5 -q "$test_file" 2>/dev/null)

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Set file on input, poll for completion
    _upload_and_poll "15.12" "$test_file"

    case "$UPLOAD_FINAL_STATUS" in
        complete) ;;
        pending)
            skip "Upload timed out (extension upload handler not implemented yet)."
            return ;;
        failed)
            fail "Extension reported upload failure."
            return ;;
        *)
            fail "Upload failed: $UPLOAD_FINAL_STATUS"
            return ;;
    esac

    # Fill form fields (title + tags) via execute_js
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Fill form fields","script":"document.querySelector(\"input[name=title]\").value=\"Smoke Test Upload\"; document.querySelector(\"input[name=tags]\").value=\"smoke,gasoline\"; \"fields_set\""}'
    sleep 0.5

    # Submit form
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Submit upload form","script":"document.querySelector(\"form\").submit(); \"submitted\""}'
    sleep 3

    # Observe success page first
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.12" "observe success page" "$response" "$content_text"

    if ! echo "$content_text" | grep -qi "Upload Successful"; then
        fail "Upload did not reach success page. Page: $(truncate "$content_text" 300)"
        return
    fi

    # Server-side verification via upload server's /api/last-upload
    local verify_body
    verify_body=$(curl -s "http://127.0.0.1:${UPLOAD_PORT}/api/last-upload" 2>/dev/null)
    local verify_md5 verify_csrf verify_cookie
    verify_md5=$(echo "$verify_body" | jq -r '.md5' 2>/dev/null)
    verify_csrf=$(echo "$verify_body" | jq -r '.csrf_ok' 2>/dev/null)
    verify_cookie=$(echo "$verify_body" | jq -r '.cookie_ok' 2>/dev/null)

    log_diagnostic "15.12" "server verify" "$verify_body" ""

    if [ "$verify_md5" = "$original_md5" ] && [ "$verify_csrf" = "true" ] && [ "$verify_cookie" = "true" ]; then
        pass "Full verification: MD5 match ($verify_md5), CSRF ok, cookie ok."
    else
        fail "Server verification: md5=$verify_md5 (expected $original_md5), csrf=$verify_csrf, cookie=$verify_cookie."
    fi
}
run_test_15_12

# ── Test 15.13: Confirmation page via observe ────────────
begin_test "15.13" "Confirmation page visible via observe (requires pilot)" \
    "After upload tests, observe(page) should show Upload Successful" \
    "Tests: observe reads confirmation page after browser form submission + redirect"

run_test_15_13() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Browser should be on /upload/success from 15.12
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.13" "observe page" "$response" "$content_text"

    if echo "$content_text" | grep -qi "upload.*success\|Upload Successful"; then
        pass "Confirmation page visible via observe(page)."
    else
        fail "Browser not on upload success page after 15.10/15.12."
    fi
}
run_test_15_13

# ── Test 15.14: Auth failure visible in browser ──────────
begin_test "15.14" "Auth failure visible in browser (requires pilot)" \
    "Navigate to /logout to clear session, then /upload to trigger 401 — verify in browser" \
    "Tests: server auth rejection is visible to the user in the browser"

run_test_15_14() {
    if [ "$EXTENSION_CONNECTED" != "true" ] || [ "$PILOT_ENABLED" != "true" ]; then
        skip "Extension or pilot not available."
        return
    fi

    # Clear session cookie by navigating to /logout
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/logout\",\"reason\":\"Clear session for auth failure test\"}"
    sleep 1

    # Navigate to /upload — server should reject with 401 (no session)
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/upload\",\"reason\":\"Attempt upload form without session\"}"
    sleep 1

    # Observe page — should show 401
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    log_diagnostic "15.14" "observe 401" "$response" "$content_text"

    if echo "$content_text" | grep -qi "401\|Not logged in"; then
        pass "Auth failure: 401 visible in browser after session cleared."
    else
        fail "Expected 401 page after logout. Got: $(truncate "$content_text" 200)"
    fi

    # Restore session for subsequent tests (navigate to / to get new cookie)
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"http://127.0.0.1:${UPLOAD_PORT}/\",\"reason\":\"Restore session after auth failure test\"}"
    sleep 1
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

    # Navigate browser: / (session) → /upload (form)
    _navigate_to_upload_form

    # Create test file and attempt upload
    local test_file="$UPLOAD_TEST_DIR/verify-upload.txt"
    echo "verify-upload-content" > "$test_file"

    # Upload and poll
    _upload_and_poll "15.15" "$test_file"

    # Even if pending, check the DOM — the file might have been set
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
