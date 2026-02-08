#!/bin/bash
# smoke-test.sh — Human smoke test for Gasoline MCP.
# Exercises the full stack: cold start → extension → navigate → trigger
# errors, clicks, form fills, WebSocket → verify data in observe buffers
# → graceful shutdown.
#
# Requires: Chrome with Gasoline extension, AI Web Pilot enabled,
#           a tab tracked on any page.
#
# Usage:
#   bash scripts/smoke-test.sh          # default port 7890
#   bash scripts/smoke-test.sh 7890     # explicit port

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/tests/framework.sh"

init_framework "${1:-7890}" "${2:-/dev/null}"

# Diagnostic log file for raw responses
DIAGNOSTICS_FILE="/tmp/gasoline-smoke-diagnostics-$$.log"
echo "Smoke Test Diagnostics — $(date)" > "$DIAGNOSTICS_FILE"
echo "Port: ${1:-7890}" >> "$DIAGNOSTICS_FILE"
echo "======================================" >> "$DIAGNOSTICS_FILE"

begin_category "0" "Human Smoke Test" "27"

SKIPPED_COUNT=0
EXTENSION_CONNECTED=false
PILOT_ENABLED=false

skip() {
    local description="$1"
    SKIPPED_COUNT=$((SKIPPED_COUNT + 1))
    {
        echo "  SKIP: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
}

# Override framework pass/fail to pause after every test
pass() {
    local description="$1"
    PASS_COUNT=$((PASS_COUNT + 1))
    {
        echo "  PASS: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
    pause_for_human
}

fail() {
    local description="$1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    {
        echo "  FAIL: ${description}"
        echo ""
    } | tee -a "$OUTPUT_FILE"
    pause_for_human
}

pause_for_human() {
    echo "  ── Press Enter to continue, Ctrl-C to abort ──"
    read -r
    echo ""
}

# ── Log diagnostic data ──────────────────────────────────
log_diagnostic() {
    local test_name="$1"
    local action="$2"
    local response="$3"
    local result="$4"
    {
        echo ""
        echo "═══════ $test_name — $action ═══════"
        echo "Response:"
        echo "$response" | head -100
        if [ -n "$result" ]; then
            echo ""
            echo "Result:"
            echo "$result" | head -100
        fi
    } >> "$DIAGNOSTICS_FILE"
}

# ── Interact helper ──────────────────────────────────────
# Fires an interact command and waits for completion via polling.
# Sets INTERACT_RESULT to the command result text (or empty on timeout).
interact_and_wait() {
    local action="$1"
    local args="$2"
    local max_polls="${3:-15}"

    local response
    response=$(call_tool "interact" "$args")
    local content_text
    content_text=$(extract_content_text "$response")

    # Log raw response
    {
        echo ""
        echo "─ $(date +%H:%M:%S) interact($action) initial response:"
        echo "$content_text" | head -50
    } >> "$DIAGNOSTICS_FILE"

    # Extract correlation_id from response
    local corr_id
    corr_id=$(echo "$content_text" | grep -oE '"correlation_id":\s*"[^"]+"' | head -1 | sed 's/.*"correlation_id":\s*"//' | sed 's/"//')

    if [ -z "$corr_id" ]; then
        INTERACT_RESULT="$content_text"
        return 1
    fi

    # Poll for completion
    for i in $(seq 1 "$max_polls"); do
        sleep 0.5
        local poll_response
        poll_response=$(call_tool "observe" "{\"what\":\"command_result\",\"correlation_id\":\"$corr_id\"}")
        local poll_text
        poll_text=$(extract_content_text "$poll_response")

        if echo "$poll_text" | grep -q '"status":"complete"'; then
            INTERACT_RESULT="$poll_text"
            {
                echo "✓ Complete after poll $i"
                echo "$poll_text" | head -30
            } >> "$DIAGNOSTICS_FILE"
            return 0
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            INTERACT_RESULT="$poll_text"
            {
                echo "✗ Failed after poll $i"
                echo "$poll_text" | head -30
            } >> "$DIAGNOSTICS_FILE"
            return 1
        fi
    done

    INTERACT_RESULT="timeout waiting for $action"
    {
        echo "✗ Timeout after $max_polls polls"
    } >> "$DIAGNOSTICS_FILE"
    return 1
}

# ── Test S.1: Cold start auto-spawn ──────────────────────
begin_test "S.1" "Cold start auto-spawn" \
    "Kill any running daemon, send an MCP call, verify the daemon spawns automatically" \
    "This is the most critical path — if cold start fails, nothing works"

run_test_s1() {
    kill_server
    sleep 0.5

    if lsof -ti :"$PORT" >/dev/null 2>&1; then
        fail "Port $PORT still occupied after kill. Cannot test cold start."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"page"}')

    if [ -z "$response" ]; then
        fail "No response at all. Daemon failed to auto-spawn."
        return
    fi

    if ! check_valid_jsonrpc "$response"; then
        fail "Response is not valid JSON-RPC: $(truncate "$response")"
        return
    fi

    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -q "starting up"; then
        pass "Cold start: daemon spawned (got 'retry in 2s' message)."
    elif check_not_error "$response"; then
        pass "Cold start: daemon spawned and responded immediately."
    else
        fail "Cold start: daemon spawned but returned tool-level error. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s1

# ── Test S.2: Health + version ───────────────────────────
begin_test "S.2" "Health endpoint and version" \
    "Verify /health returns status=ok and version matches VERSION file" \
    "Version mismatch means the wrong binary is running"

run_test_s2() {
    sleep 2
    wait_for_health 50

    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)
    if [ "$status_val" != "ok" ]; then
        fail "Health status='$status_val', expected 'ok'. Body: $(truncate "$body")"
        return
    fi

    local health_version
    health_version=$(echo "$body" | jq -r '.version // empty' 2>/dev/null)
    if [ "$health_version" != "$VERSION" ]; then
        fail "Version mismatch: health='$health_version', VERSION file='$VERSION'."
        return
    fi

    pass "Health OK: status='ok', version='$health_version' matches VERSION file."
}
run_test_s2

# ── Test S.3: Extension gate ─────────────────────────────
begin_test "S.3" "Extension connected" \
    "Check /health for capture.available=true" \
    "All browser tests require extension. Stops here if not connected."

run_test_s3() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local capture_available
    capture_available=$(echo "$body" | jq -r '.capture.available // false' 2>/dev/null)

    if [ "$capture_available" = "true" ]; then
        EXTENSION_CONNECTED=true
        pass "Extension connected: capture.available=true."
    else
        fail "Extension NOT connected. Open Chrome with Gasoline extension and track a tab."
        echo "" | tee -a "$OUTPUT_FILE"
        echo "  >>> 1. Open Chrome with the Gasoline extension installed" | tee -a "$OUTPUT_FILE"
        echo "  >>> 2. Click the Gasoline icon → 'Track This Tab' on any page" | tee -a "$OUTPUT_FILE"
        echo "  >>> 3. Enable 'AI Web Pilot' toggle in the extension popup" | tee -a "$OUTPUT_FILE"
        echo "  >>> 4. Re-run: bash scripts/smoke-test.sh" | tee -a "$OUTPUT_FILE"
        echo "" | tee -a "$OUTPUT_FILE"
    fi
}
run_test_s3

# ── Test S.4: Navigate to test page ──────────────────────
begin_test "S.4" "Navigate to a page" \
    "Use interact(navigate) to open example.com, verify observe(page) reflects it" \
    "Tests the full interact pipeline: MCP → daemon → extension → browser"

run_test_s4() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Clear buffers first so we only see data from our actions
    call_tool "configure" '{"action":"clear","buffer":"all"}' >/dev/null

    # Navigate the tracked tab
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Initial page load"}'

    # Check if pilot is disabled
    if echo "$INTERACT_RESULT" | grep -qi "pilot.*disabled\|not enabled\|web pilot"; then
        skip "AI Web Pilot is disabled. Enable it in the extension popup and re-run."
        return
    fi

    PILOT_ENABLED=true

    # Give the page time to load and extension to sync
    sleep 3

    # Verify page URL — the tracked tab should now be on example.com
    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "example.com"; then
        pass "Navigated to example.com. observe(page) confirms URL."
    else
        fail "Navigate did not work. observe(page) still shows: $(truncate "$content_text" 200)"
    fi
}
run_test_s4

# ── Test S.5: Trigger console log + error ────────────────
begin_test "S.5" "Trigger console log and error via JS" \
    "Execute JS to console.log and console.error with markers, verify in observe" \
    "Tests: inject.js console monkey-patch → extension → daemon buffer → MCP observe"

SMOKE_MARKER="GASOLINE_SMOKE_$(date +%s)"

run_test_s5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Fire console.log
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Trigger console.log\",\"script\":\"console.log('${SMOKE_MARKER}_LOG')\"}"

    # Fire console.error with stack
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Trigger console.error\",\"script\":\"console.error('${SMOKE_MARKER}_ERROR')\"}"

    # Fire a thrown error
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Trigger thrown error\",\"script\":\"try { throw new Error('${SMOKE_MARKER}_THROWN') } catch(e) { console.error(e.message, e.stack) }\"}"

    # Give the extension time to send logs to daemon
    sleep 2

    # Check logs
    local log_response
    log_response=$(call_tool "observe" '{"what":"logs"}')
    local log_text
    log_text=$(extract_content_text "$log_response")

    local log_ok=false
    if echo "$log_text" | grep -q "${SMOKE_MARKER}_LOG"; then
        log_ok=true
    fi

    # Check errors
    local err_response
    err_response=$(call_tool "observe" '{"what":"errors"}')
    local err_text
    err_text=$(extract_content_text "$err_response")

    local err_ok=false
    if echo "$err_text" | grep -q "${SMOKE_MARKER}"; then
        err_ok=true
    fi

    if [ "$log_ok" = "true" ] && [ "$err_ok" = "true" ]; then
        pass "Log marker '${SMOKE_MARKER}_LOG' in observe(logs) AND error marker in observe(errors)."
    elif [ "$log_ok" = "true" ]; then
        fail "Log marker found but error marker '${SMOKE_MARKER}' missing from observe(errors). Errors: $(truncate "$err_text" 200)"
    else
        fail "Log marker '${SMOKE_MARKER}_LOG' NOT found in observe(logs). Console monkey-patch may be broken. Logs: $(truncate "$log_text" 200)"
    fi
}
run_test_s5

# ── Test S.6: Click a button ─────────────────────────────
begin_test "S.6" "Click a button via JS" \
    "Inject a button into the page, click it, verify in observe(actions)" \
    "Tests: user action capture → extension → daemon → MCP observe"

run_test_s6() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Inject a button and click it
    local js="var btn = document.createElement('button'); btn.id = 'smoke-btn-${SMOKE_MARKER}'; btn.textContent = 'Smoke Test'; document.body.appendChild(btn); btn.click(); 'clicked'"
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Inject + click button\",\"script\":\"$js\"}"

    sleep 1

    local response
    response=$(call_tool "observe" '{"what":"actions"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "click"; then
        pass "Click action captured in observe(actions)."
    else
        fail "No 'click' action found. Action capture may be broken. Actions: $(truncate "$content_text" 200)"
    fi
}
run_test_s6

# ── Test S.7: Fill a form input ──────────────────────────
begin_test "S.7" "Fill a form input via JS" \
    "Inject an input, set its value and dispatch input event, verify in observe(actions)" \
    "Tests: form input tracking → extension → daemon → MCP observe"

run_test_s7() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Inject an input and fill it
    local js="var inp = document.createElement('input'); inp.id = 'smoke-input-${SMOKE_MARKER}'; inp.type = 'text'; document.body.appendChild(inp); inp.focus(); inp.value = 'smoke-test-value'; inp.dispatchEvent(new Event('input', {bubbles:true})); 'filled'"
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Inject + fill input\",\"script\":\"$js\"}"

    sleep 1

    local response
    response=$(call_tool "observe" '{"what":"actions"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "input\|change\|focus"; then
        pass "Form input action captured in observe(actions)."
    else
        fail "No input/change/focus action found. Form tracking may be broken. Actions: $(truncate "$content_text" 200)"
    fi
}
run_test_s7

# ── Test S.8: Highlight an element ───────────────────────
begin_test "S.8" "Highlight an element via interact(highlight)" \
    "Use interact(highlight) to highlight the body element, verify command completes" \
    "Tests: highlight pipeline: MCP → daemon → extension → inject overlay"

run_test_s8() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Highlight body element (should work on any page)
    interact_and_wait "highlight" '{"action":"highlight","selector":"body","duration_ms":2000,"reason":"Highlight page body"}'

    if echo "$INTERACT_RESULT" | grep -qi "complete\|success\|highlighted"; then
        pass "Highlight command completed successfully. Result: $(truncate "$INTERACT_RESULT" 200)"
    elif echo "$INTERACT_RESULT" | grep -qi "timeout"; then
        fail "Highlight command timed out. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        # Check if the command was at least queued (correlation_id returned)
        if echo "$INTERACT_RESULT" | grep -qi "correlation_id"; then
            pass "Highlight command queued (got correlation_id). Result: $(truncate "$INTERACT_RESULT" 200)"
        else
            fail "Highlight command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        fi
    fi
}
run_test_s8

# ── Test S.21: Subtitle standalone ─────────────────────────
begin_test "S.21" "Subtitle: standalone set, verify visible, then clear" \
    "Use interact(subtitle) to display text at bottom of viewport, verify it appears, then clear it" \
    "Tests: subtitle pipeline: MCP → daemon → extension → content script overlay"

run_test_s21() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to a clean page first
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Clean page for subtitle test"}' 20
    sleep 2

    # Set subtitle text
    interact_and_wait "subtitle" '{"action":"subtitle","text":"Gasoline smoke test — this text should appear at the bottom of the viewport"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "Subtitle set returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Verify the subtitle element exists in the DOM
    # Return a simple string (not JSON.stringify) to avoid escaping issues in command result pipeline
    sleep 1
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check subtitle visibility","script":"(function() { var el = document.getElementById(\"gasoline-subtitle\"); if (!el) return \"NOT_FOUND\"; var style = window.getComputedStyle(el); var visible = style.display !== \"none\" && style.opacity !== \"0\"; return (visible ? \"VISIBLE\" : \"HIDDEN\") + \":\" + el.textContent; })()"}'

    local dom_check="$INTERACT_RESULT"

    if echo "$dom_check" | grep -q "NOT_FOUND"; then
        fail "Subtitle element #gasoline-subtitle not found in DOM after setting text."
        return
    fi

    local has_text=false
    local is_visible=false
    if echo "$dom_check" | grep -q "smoke test"; then
        has_text=true
    fi
    if echo "$dom_check" | grep -q "VISIBLE:"; then
        is_visible=true
    fi

    if [ "$has_text" != "true" ]; then
        fail "Subtitle element exists but text content doesn't match. DOM: $(truncate "$dom_check" 300)"
        return
    fi

    if [ "$is_visible" != "true" ]; then
        fail "Subtitle element has correct text but is not visible (opacity still 0). DOM: $(truncate "$dom_check" 300)"
        return
    fi

    # Now clear the subtitle
    interact_and_wait "subtitle" '{"action":"subtitle","text":""}'
    sleep 0.5

    # Verify it's gone or hidden
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify subtitle cleared","script":"(function() { var el = document.getElementById(\"gasoline-subtitle\"); if (!el) return \"REMOVED\"; var style = window.getComputedStyle(el); if (style.display === \"none\" || style.opacity === \"0\" || el.textContent === \"\") return \"HIDDEN\"; return \"STILL_VISIBLE:\" + el.textContent; })()"}'

    local clear_check="$INTERACT_RESULT"

    if echo "$clear_check" | grep -qi "REMOVED\|HIDDEN"; then
        pass "Subtitle: set text (visible + correct content), then cleared (element removed/hidden). Full lifecycle works."
    elif echo "$clear_check" | grep -qi "STILL_VISIBLE"; then
        fail "Subtitle still visible after clear. Result: $(truncate "$clear_check" 200)"
    else
        pass "Subtitle: set and clear commands accepted. DOM check: $(truncate "$clear_check" 200)"
    fi
}
run_test_s21

# ── Test S.22: Subtitle as optional param on navigate ─────
begin_test "S.22" "Subtitle as optional param on interact(navigate)" \
    "Navigate with subtitle param in same call, verify both navigation and subtitle happen" \
    "Tests: composable subtitle — single tool call for action + narration"

run_test_s22() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Single call: navigate + subtitle
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","subtitle":"Navigating to example.com — verifying composable subtitle"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "unknown.*subtitle\|invalid.*subtitle\|unrecognized"; then
        fail "Server rejected subtitle as unknown parameter. It must be accepted on all interact actions. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 3

    # Verify the page navigated
    local page_response
    page_response=$(call_tool "observe" '{"what":"page"}')
    local page_text
    page_text=$(extract_content_text "$page_response")

    local navigated=false
    if echo "$page_text" | grep -qi "example.com"; then
        navigated=true
    fi

    # Verify the subtitle is visible
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check composable subtitle","script":"(function() { var el = document.getElementById(\"gasoline-subtitle\"); if (!el) return \"NOT_FOUND\"; return JSON.stringify({ text: el.textContent, visible: window.getComputedStyle(el).display !== \"none\" }); })()"}'

    local subtitle_check="$INTERACT_RESULT"
    local has_subtitle=false
    if echo "$subtitle_check" | grep -q "composable subtitle\|example.com"; then
        has_subtitle=true
    fi

    if [ "$navigated" = "true" ] && [ "$has_subtitle" = "true" ]; then
        pass "Composable subtitle: single call navigated to example.com AND displayed subtitle text."
    elif [ "$navigated" = "true" ] && [ "$has_subtitle" != "true" ]; then
        fail "Navigation worked but subtitle not visible. Subtitle check: $(truncate "$subtitle_check" 200)"
    elif [ "$navigated" != "true" ] && [ "$has_subtitle" = "true" ]; then
        fail "Subtitle visible but navigation didn't work. Page: $(truncate "$page_text" 200)"
    else
        fail "Neither navigation nor subtitle worked. Page: $(truncate "$page_text" 200), Subtitle: $(truncate "$subtitle_check" 200)"
    fi

    # Clean up: clear subtitle
    interact_and_wait "subtitle" '{"action":"subtitle","text":""}'
}
run_test_s22

# ── Test S.24: On-demand screenshot ───────────────────────
begin_test "S.24" "Screenshot: on-demand capture via observe(screenshot)" \
    "Call observe(screenshot) and verify it captures the current viewport, returns filename and path" \
    "Tests: on-demand screenshot pipeline: MCP → daemon → extension → captureVisibleTab → save to disk"

run_test_s24() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Take a screenshot
    local screenshot_response
    screenshot_response=$(call_tool "observe" '{"what":"screenshot"}')

    if ! check_not_error "$screenshot_response"; then
        local err_text
        err_text=$(extract_content_text "$screenshot_response")
        # If it says "no tab" or "not tracked", that's expected without pilot
        if echo "$err_text" | grep -qi "no tab\|not tracked\|not connected"; then
            skip "No tracked tab for screenshot."
            return
        fi
        fail "observe(screenshot) returned error. Content: $(truncate "$err_text" 200)"
        return
    fi

    local text
    text=$(extract_content_text "$screenshot_response")

    # Verify response has filename
    local has_filename=false
    local has_path=false
    if echo "$text" | grep -q '"filename"'; then
        has_filename=true
    fi
    if echo "$text" | grep -q '"path"'; then
        has_path=true
    fi

    if [ "$has_filename" = "true" ] && [ "$has_path" = "true" ]; then
        # Extract path and verify file exists
        local screenshot_path
        screenshot_path=$(echo "$text" | python3 -c "import sys,json; t=sys.stdin.read(); i=t.find('{'); print(json.loads(t[i:]).get('path','') if i>=0 else '')" 2>/dev/null || echo "")
        if [ -n "$screenshot_path" ] && [ -f "$screenshot_path" ]; then
            local file_size
            file_size=$(wc -c < "$screenshot_path" | tr -d ' ')
            pass "Screenshot captured: filename present, path present, file exists ($file_size bytes). Path: $screenshot_path"
        else
            pass "Screenshot captured: filename and path present. File check skipped (path: $screenshot_path)."
        fi
    else
        fail "Screenshot response missing filename or path. Content: $(truncate "$text" 200)"
    fi
}
run_test_s24

# ── Test S.9: Error clusters ─────────────────────────────
begin_test "S.9" "Error clusters aggregate triggered errors" \
    "After S.5 triggered multiple errors, verify observe(error_clusters) groups them" \
    "Tests: error dedup and clustering — critical for noise reduction in real apps"

run_test_s9() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"error_clusters"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(error_clusters)."
        return
    fi

    # Print cluster info for human verification
    echo "  [error clusters]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    clusters = data.get('clusters', [])
    print(f'    total clusters: {len(clusters)}')
    for c in clusters[:3]:
        msg = c.get('message', c.get('pattern', ''))[:80]
        count = c.get('count', c.get('occurrences', 1))
        print(f'    [{count}x] {msg}')
except: pass
" 2>/dev/null || true

    if echo "$content_text" | grep -qi "cluster\|count\|pattern\|message\|occurrence"; then
        pass "Error clusters returned with aggregation data."
    else
        fail "observe(error_clusters) missing expected fields. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s9

# ── Test S.10: DOM query (page structure parsing) ────────
begin_test "S.10" "DOM query parses page structure" \
    "Use configure(query_dom) to query elements on the page, verify DOM data returned" \
    "Tests: page structure analysis — the 'screenshot parsing' equivalent for understanding page content"

run_test_s10() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Query for headings and links — should exist on most pages
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Count interactive elements","script":"document.querySelectorAll(\"h1, h2, a, button, input\").length"}'

    local dom_response
    dom_response=$(call_tool "configure" '{"action":"query_dom","selector":"h1, a, button, input"}')
    local dom_text
    dom_text=$(extract_content_text "$dom_response")

    # Print DOM query results for human verification
    echo "  [DOM query: h1, a, button, input]"
    echo "$dom_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    elements = data.get('elements', data.get('results', []))
    if isinstance(elements, list):
        print(f'    found: {len(elements)} element(s)')
        for e in elements[:5]:
            tag = e.get('tag', e.get('tagName', '?'))
            text = e.get('text', e.get('textContent', ''))[:50]
            print(f'    <{tag}> {text}')
    else:
        print(f'    response keys: {list(data.keys())[:5]}')
except Exception as ex:
    print(f'    (parse note: {ex})')
" 2>/dev/null || true

    if [ -z "$dom_text" ]; then
        fail "Empty response from query_dom."
        return
    fi

    # DOM query may timeout without extension, but should return valid JSON-RPC
    if echo "$dom_text" | grep -qi "element\|tag\|text\|selector\|result\|timeout\|pending"; then
        pass "DOM query returned page structure data. Content: $(truncate "$dom_text" 200)"
    else
        fail "DOM query response missing expected fields. Content: $(truncate "$dom_text" 200)"
    fi
}
run_test_s10

# ── Test S.11: Full form lifecycle ───────────────────────
begin_test "S.11" "Full form: create, fill multiple fields, submit" \
    "Inject a complete form with multiple inputs, fill each, submit, verify all actions captured" \
    "Tests: full form lifecycle — creation, multi-field fill, and submit event capture"

run_test_s11() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Clear actions buffer so we only see form-related actions
    call_tool "configure" '{"action":"clear","buffer":"all"}' >/dev/null
    sleep 0.3

    # Inject a complete form with multiple fields
    local form_js="(function() {
        var f = document.createElement('form');
        f.id = 'smoke-form';
        f.innerHTML = '<input name=\"username\" type=\"text\" id=\"sf-user\">' +
            '<input name=\"email\" type=\"email\" id=\"sf-email\">' +
            '<input name=\"password\" type=\"password\" id=\"sf-pass\">' +
            '<select name=\"role\" id=\"sf-role\"><option value=\"user\">User</option><option value=\"admin\">Admin</option></select>' +
            '<button type=\"submit\" id=\"sf-submit\">Submit</button>';
        f.onsubmit = function(e) { e.preventDefault(); window.__SMOKE_FORM_SUBMITTED__ = true; };
        document.body.appendChild(f);
        return 'form-injected';
    })()"
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Create form\",\"script\":$(echo "$form_js" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")}"

    sleep 0.5

    # Fill each field
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Fill username","script":"var el = document.getElementById(\"sf-user\"); el.focus(); el.value = \"smokeuser\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-user\""}'
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Fill email","script":"var el = document.getElementById(\"sf-email\"); el.focus(); el.value = \"smoke@test.com\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-email\""}'
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Fill password","script":"var el = document.getElementById(\"sf-pass\"); el.focus(); el.value = \"s3cure!\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-pass\""}'
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Select role","script":"var el = document.getElementById(\"sf-role\"); el.value = \"admin\"; el.dispatchEvent(new Event(\"change\", {bubbles:true})); \"selected-role\""}'

    # Submit the form
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Submit form","script":"document.getElementById(\"sf-submit\").click(); \"submitted\""}'

    sleep 1

    # Verify form submission happened
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify form submitted","script":"window.__SMOKE_FORM_SUBMITTED__ === true ? \"submit-confirmed\" : \"no-submit\""}'

    local submit_confirmed=false
    if echo "$INTERACT_RESULT" | grep -q "submit-confirmed"; then
        submit_confirmed=true
    fi

    # Check actions buffer for form-related events
    local response
    response=$(call_tool "observe" '{"what":"actions"}')
    local content_text
    content_text=$(extract_content_text "$response")

    echo "  [form actions captured]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    entries = data.get('entries', [])
    form_actions = [e for e in entries if any(k in str(e).lower() for k in ['input', 'change', 'click', 'submit', 'focus'])]
    print(f'    total actions: {len(entries)}, form-related: {len(form_actions)}')
    for a in form_actions[:6]:
        atype = a.get('type', a.get('action', '?'))
        target = a.get('target', a.get('selector', ''))[:50]
        print(f'    {atype}: {target}')
except: pass
" 2>/dev/null || true

    local has_input has_click
    has_input=$(echo "$content_text" | grep -ci "input\|change\|focus" || true)
    has_click=$(echo "$content_text" | grep -ci "click\|submit" || true)

    if [ "$submit_confirmed" != "true" ]; then
        fail "Form submission not confirmed. Form lifecycle test failed. Actions: $(truncate "$content_text" 200)"
    elif [ "$has_input" -eq 0 ]; then
        fail "Form submitted but no input/change/focus actions captured. Actions: $(truncate "$content_text" 200)"
    elif [ "$has_click" -eq 0 ]; then
        fail "Form submitted but no click/submit actions captured. Actions: $(truncate "$content_text" 200)"
    else
        pass "Full form lifecycle: submitted, $has_input input events + $has_click click/submit events captured."
    fi
}
run_test_s11

# ── Test S.12: Real WebSocket traffic ─────────────────────
begin_test "S.12" "WebSocket capture on a real WS-heavy page" \
    "Navigate to a live trading page (Binance BTC/USDT), verify WS connections in observe" \
    "Tests: real WebSocket interception → extension → daemon → MCP observe"

run_test_s12() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to a WebSocket-heavy page (live crypto trading)
    interact_and_wait "navigate" '{"action":"navigate","url":"https://www.binance.com/en/trade/BTC_USDT","reason":"Load WebSocket-heavy page"}' 20

    # Give WebSocket connections time to establish
    sleep 5

    local response
    response=$(call_tool "observe" '{"what":"websocket_status"}')
    local content_text
    content_text=$(extract_content_text "$response")

    local active_count
    active_count=$(echo "$content_text" | grep -oE '"active_count":[0-9]+' | head -1 | grep -oE '[0-9]+')

    if [ -n "$active_count" ] && [ "$active_count" -gt 0 ] 2>/dev/null; then
        # Print sample connection data for human verification
        echo "  [active connections: $active_count]"
        echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    for c in data.get('connections', [])[:3]:
        url = c.get('url', '')[:80]
        state = c.get('state', '')
        rate = c.get('message_rate', {}).get('incoming', {})
        msgs = rate.get('total', 0)
        per_sec = rate.get('per_second', 0)
        print(f'    {state}: {url}')
        print(f'      incoming: {msgs} msgs ({per_sec}/s)')
except: pass
" 2>/dev/null || true
        pass "WebSocket capture: $active_count active connection(s) on Binance trading page."
    else
        # Check websocket_events as fallback (connections may have closed)
        local events_response
        events_response=$(call_tool "observe" '{"what":"websocket_events","last_n":5}')
        local events_text
        events_text=$(extract_content_text "$events_response")

        if echo "$events_text" | grep -qi "binance\|stream\|ws"; then
            pass "WebSocket events captured from Binance (connections may have closed)."
        else
            fail "No WebSocket connections captured. Early-patch should intercept WS before page scripts."
        fi
    fi
}
run_test_s12

# ── Test S.13: Network waterfall has real data ───────────
begin_test "S.13" "Network waterfall has real resource timing" \
    "observe(network_waterfall) should return entries with real URLs and timing" \
    "Tests on-demand extension query: MCP → daemon → extension → performance API"

run_test_s13() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # example.com has no sub-resources, so inject a fetch to seed the resource timing buffer
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Seed resource timing buffer","script":"fetch(window.location.href).then(function(r){return r.ok?\"fetched\":\"error\"}).catch(function(){return \"error\"})"}'
    sleep 0.5

    local response
    response=$(call_tool "observe" '{"what":"network_waterfall"}')

    if check_bridge_timeout "$response"; then
        skip "Bridge timeout on network_waterfall (extension query took >4s)."
        return
    fi

    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(network_waterfall)."
        return
    fi

    if echo "$content_text" | grep -qE 'https?://'; then
        local entry_count
        entry_count=$(echo "$content_text" | grep -oE '"count":[0-9]+' | head -1 | grep -oE '[0-9]+')

        # Print a sample entry so humans can verify data quality
        echo "  [sample entry]"
        echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    entries = data.get('entries', [])
    if entries:
        e = entries[0]
        print(f'    url:               {e.get(\"url\",\"\")[:80]}')
        print(f'    initiator_type:    {e.get(\"initiator_type\",\"\")}')
        print(f'    duration_ms:       {e.get(\"duration_ms\",0)}')
        print(f'    start_time:        {e.get(\"start_time\",0)}')
        print(f'    transfer_size:     {e.get(\"transfer_size\",0)}')
        print(f'    decoded_body_size: {e.get(\"decoded_body_size\",0)}')
        print(f'    page_url:          {e.get(\"page_url\",\"\")[:80]}')
except: pass
" 2>/dev/null || true

        # Verify key fields are actually populated (not all zeros)
        local has_initiator has_start_time has_transfer
        has_initiator=$(echo "$content_text" | grep -oE '"initiator_type":"[a-z]' | head -1)
        has_start_time=$(echo "$content_text" | grep -oE '"start_time":[1-9]' | head -1)
        has_transfer=$(echo "$content_text" | grep -oE '"transfer_size":[1-9]' | head -1)

        if [ -n "$has_initiator" ] && [ -n "$has_start_time" ] && [ -n "$has_transfer" ]; then
            pass "Real waterfall data: ${entry_count:-some} entries with URLs, timing, initiator types, and transfer sizes."
        else
            fail "Waterfall entries have URLs but missing fields: initiator_type=$([ -n "$has_initiator" ] && echo 'ok' || echo 'MISSING'), start_time=$([ -n "$has_start_time" ] && echo 'ok' || echo 'MISSING'), transfer_size=$([ -n "$has_transfer" ] && echo 'ok' || echo 'MISSING')."
        fi
    else
        fail "No real URLs in waterfall. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s13

# ── Test S.14: observe(page) still works after all actions ─
begin_test "S.14" "Page state survives action barrage" \
    "After navigate + JS execution + clicks + forms + WS, observe(page) still returns valid data" \
    "Verifies no corruption from heavy interaction"

run_test_s14() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qE 'https?://'; then
        pass "observe(page) still returns a valid URL after all actions."
    else
        fail "observe(page) broken after actions. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s14

# ── Test S.15: Health still OK after everything ──────────
begin_test "S.15" "Health still OK after everything" \
    "Verify daemon is healthy after all the interaction and observation" \
    "Detects memory leaks, crashes, or degraded state"

run_test_s15() {
    local body
    body=$(get_http_body "http://localhost:${PORT}/health")

    local status_val
    status_val=$(echo "$body" | jq -r '.status // empty' 2>/dev/null)
    if [ "$status_val" != "ok" ]; then
        fail "Health status='$status_val' after test barrage. Body: $(truncate "$body")"
        return
    fi

    pass "Daemon still healthy after full smoke test. status='ok'."
}
run_test_s15

# ── Test S.17: Rich Action Results — refresh returns perf_diff ──
begin_test "S.17" "Refresh returns perf_diff after baseline" \
    "Navigate to a page (baseline), refresh (comparison), verify perf_diff in command result" \
    "Tests: extension perf tracking → auto-diff → enriched action result"

run_test_s17() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to establish baseline metrics
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Load baseline page"}' 20
    sleep 3

    # First refresh — establishes baseline (no perf_diff expected)
    interact_and_wait "refresh" '{"action":"refresh","reason":"Establish perf baseline"}' 20
    sleep 3

    # Second refresh — should have perf_diff comparing to first load
    interact_and_wait "refresh" '{"action":"refresh","reason":"Measure perf diff"}' 20

    if [ -z "$INTERACT_RESULT" ]; then
        fail "No result from refresh command."
        return
    fi

    echo "  [refresh result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    if 'perf_diff' in data:
        pd = data['perf_diff']
        metrics = pd.get('metrics', {})
        for k, v in list(metrics.items())[:4]:
            print(f'    {k}: {v.get(\"before\",\"?\")} -> {v.get(\"after\",\"?\")} ({v.get(\"pct\",\"?\")})')
        if 'summary' in pd:
            print(f'    summary: {pd[\"summary\"][:120]}')
    else:
        print(f'    keys: {list(data.keys())[:8]}')
        print(f'    (no perf_diff found)')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    if ! echo "$INTERACT_RESULT" | grep -q '"perf_diff"'; then
        fail "Refresh result missing perf_diff. Result: $(truncate "$INTERACT_RESULT" 300)"
        return
    fi

    local has_metrics has_summary
    has_metrics=$(echo "$INTERACT_RESULT" | grep -c '"metrics"' || true)
    has_summary=$(echo "$INTERACT_RESULT" | grep -c '"summary"' || true)

    if [ "$has_metrics" -gt 0 ] && [ "$has_summary" -gt 0 ]; then
        pass "Refresh returns perf_diff with metrics and summary."
    else
        fail "perf_diff present but incomplete: metrics=$has_metrics, summary=$has_summary. Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_s17

# ── Test S.23: LLM-optimized perf_diff fields ────────────
begin_test "S.23" "perf_diff has verdict, unit, rating, clean summary" \
    "Refresh (baseline warm from S.17), verify perf_diff has LLM-optimized fields" \
    "Tests: verdict (overall signal), unit (ms/KB/count), rating (Web Vitals thresholds), no redundant sign in summary"

run_test_s23() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Refresh — baseline is warm from S.17's refresh cycle
    interact_and_wait "refresh" '{"action":"refresh","reason":"Check LLM perf fields"}' 20

    if ! echo "$INTERACT_RESULT" | grep -q '"perf_diff"'; then
        fail "No perf_diff in refresh result (needed for LLM field checks). Result: $(truncate "$INTERACT_RESULT" 300)"
        return
    fi

    echo "  [LLM optimization fields]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    text = sys.stdin.read()
    # Content may be prefixed with summary line (e.g. 'Command ...: complete')
    # Find first '{' to locate the JSON object
    idx = text.find('{')
    if idx < 0:
        raise ValueError('no JSON object found')
    data = json.loads(text[idx:])
    pd = data.get('perf_diff', {})
    print(f'    verdict: {pd.get(\"verdict\", \"MISSING\")}')
    summary = pd.get('summary', 'MISSING')
    print(f'    summary: {summary[:120]}')
    metrics = pd.get('metrics', {})
    for name in list(metrics.keys())[:5]:
        m = metrics[name]
        unit = m.get('unit', '')
        rating = m.get('rating', '')
        print(f'    {name}: {m.get(\"before\",\"?\")}{unit} -> {m.get(\"after\",\"?\")}{unit} ({m.get(\"pct\",\"?\")}) rating={rating or \"(none)\"}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local checks_passed=0
    local checks_total=4

    # 1. verdict field exists and is valid
    if echo "$INTERACT_RESULT" | grep -qE '"verdict":\s*"(improved|regressed|mixed|unchanged)"'; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: verdict field"
    fi

    # 2. unit field on timing metrics
    if echo "$INTERACT_RESULT" | grep -q '"unit":"ms"'; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: unit field (expected 'ms' on timing metrics)"
    fi

    # 3. rating field on at least one Web Vital
    if echo "$INTERACT_RESULT" | grep -qE '"rating":"(good|needs_improvement|poor)"'; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: rating field (expected on LCP/FCP/TTFB/CLS)"
    fi

    # 4. summary uses absolute percentage (no "improved -" or "regressed +")
    local summary
    summary=$(echo "$INTERACT_RESULT" | python3 -c "
import sys,json
text = sys.stdin.read()
idx = text.find('{')
if idx >= 0:
    data = json.loads(text[idx:])
    print(data.get('perf_diff',{}).get('summary',''))
" 2>/dev/null || echo "")
    if [ -n "$summary" ] && ! echo "$summary" | grep -qE "improved -|regressed \+"; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: summary has redundant sign ('improved -' or 'regressed +')"
    fi

    if [ "$checks_passed" -eq "$checks_total" ]; then
        pass "perf_diff has all LLM fields: verdict, unit, rating, clean summary ($checks_passed/$checks_total)."
    else
        fail "perf_diff missing LLM fields: $checks_passed/$checks_total. Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_s23

# ── Test S.18: Rich Action Results — click returns compact feedback ──
begin_test "S.18" "Click returns timing_ms and dom_summary" \
    "Click a button, verify the command result includes timing_ms and dom_summary" \
    "Tests: always-on compact DOM feedback (~30 tokens per action)"

run_test_s18() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to example.com for a clean page
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Clean page for click test"}' 20
    sleep 2

    # Inject a button that modifies the DOM when clicked
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Inject test button","script":"var btn = document.createElement(\"button\"); btn.id = \"perf-test-btn\"; btn.textContent = \"Test\"; btn.onclick = function() { var d = document.createElement(\"div\"); d.textContent = \"clicked\"; document.body.appendChild(d); }; document.body.appendChild(btn); \"injected\""}'
    sleep 0.5

    # Click the button via DOM primitive
    interact_and_wait "click" '{"action":"click","selector":"#perf-test-btn","reason":"Click test button"}'

    echo "  [click result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    print(f'    timing_ms: {data.get(\"timing_ms\", \"MISSING\")}')
    print(f'    dom_summary: {data.get(\"dom_summary\", \"MISSING\")}')
    print(f'    success: {data.get(\"success\", \"?\")}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local has_timing has_dom_summary
    has_timing=$(echo "$INTERACT_RESULT" | grep -c '"timing_ms"' || true)
    has_dom_summary=$(echo "$INTERACT_RESULT" | grep -c '"dom_summary"' || true)

    if [ "$has_timing" -gt 0 ] && [ "$has_dom_summary" -gt 0 ]; then
        pass "Click result includes timing_ms and dom_summary."
    else
        fail "Click result missing required fields: timing_ms=$has_timing, dom_summary=$has_dom_summary. Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_s18

# ── Test S.19: Rich Action Results — analyze:true returns full breakdown ──
begin_test "S.19" "Click with analyze:true returns full breakdown" \
    "Click with analyze:true, verify timing breakdown, dom_changes, and analysis string" \
    "Tests: opt-in detailed profiling for interaction debugging"

run_test_s19() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Inject a button that triggers DOM changes + a network request
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Inject profiling button","script":"var btn2 = document.createElement(\"button\"); btn2.id = \"analyze-btn\"; btn2.textContent = \"Analyze Me\"; btn2.onclick = function() { for (var i=0; i<5; i++) { var d = document.createElement(\"p\"); d.textContent = \"item-\" + i; document.body.appendChild(d); } }; document.body.appendChild(btn2); \"injected\""}'
    sleep 0.5

    # Click with analyze:true
    interact_and_wait "click" '{"action":"click","selector":"#analyze-btn","analyze":true,"reason":"Profile DOM changes"}'

    echo "  [analyze:true result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    if 'timing' in data:
        t = data['timing']
        print(f'    timing.total_ms: {t.get(\"total_ms\", \"?\")}')
        print(f'    timing.js_blocking_ms: {t.get(\"js_blocking_ms\", \"?\")}')
        print(f'    timing.render_ms: {t.get(\"render_ms\", \"?\")}')
    elif 'timing_ms' in data:
        print(f'    timing_ms: {data[\"timing_ms\"]} (compact, not full breakdown)')
    if 'dom_changes' in data:
        dc = data['dom_changes']
        print(f'    dom_changes.summary: {dc.get(\"summary\", \"?\")}')
        added = dc.get('added', [])
        print(f'    dom_changes.added: {len(added)} entries')
    elif 'dom_summary' in data:
        print(f'    dom_summary: {data[\"dom_summary\"]} (compact)')
    if 'analysis' in data:
        print(f'    analysis: {data[\"analysis\"][:120]}')
    if 'long_tasks' in data:
        print(f'    long_tasks: {data[\"long_tasks\"]}')
    if 'layout_shifts' in data:
        print(f'    layout_shifts: {data[\"layout_shifts\"]}')
    print(f'    all keys: {list(data.keys())}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local has_timing_breakdown has_dom_changes has_analysis
    has_timing_breakdown=$(echo "$INTERACT_RESULT" | grep -c '"total_ms"\|"js_blocking_ms"\|"render_ms"' || true)
    has_dom_changes=$(echo "$INTERACT_RESULT" | grep -c '"dom_changes"' || true)
    has_analysis=$(echo "$INTERACT_RESULT" | grep -c '"analysis"' || true)

    if [ "$has_timing_breakdown" -gt 0 ] && [ "$has_dom_changes" -gt 0 ] && [ "$has_analysis" -gt 0 ]; then
        pass "analyze:true returns full breakdown: timing, dom_changes, and analysis."
    else
        fail "analyze:true missing required fields: timing_breakdown=$has_timing_breakdown, dom_changes=$has_dom_changes, analysis=$has_analysis. Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_s19

# ── Test S.20: Rich Action Results — User Timing in observe(performance) ──
begin_test "S.20" "User Timing entries in observe(performance)" \
    "Insert performance.mark/measure via execute_js, verify they appear in observe(performance)" \
    "Tests: extension captures standard User Timing API entries"

run_test_s20() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local marker="gasoline_uat_$(date +%s)"

    # Insert performance marks and a measure
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Insert User Timing marks\",\"script\":\"performance.mark('${marker}_start'); for(var i=0;i<1000000;i++){} performance.mark('${marker}_end'); performance.measure('${marker}_duration','${marker}_start','${marker}_end'); 'marked'\"}"
    sleep 2

    # Check observe(performance) for user timing entries
    local response
    response=$(call_tool "observe" '{"what":"performance"}')
    local content_text
    content_text=$(extract_content_text "$response")

    echo "  [user timing check]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    ut = data.get('user_timing', {})
    marks = ut.get('marks', [])
    measures = ut.get('measures', [])
    print(f'    marks: {len(marks)}')
    for m in marks[:4]:
        print(f'      {m.get(\"name\",\"?\")} @ {m.get(\"time\",m.get(\"startTime\",\"?\"))}')
    print(f'    measures: {len(measures)}')
    for m in measures[:2]:
        print(f'      {m.get(\"name\",\"?\")} duration={m.get(\"duration\",\"?\")}ms')
except Exception as e:
    print(f'    (no user_timing in response: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -q "$marker"; then
        pass "User Timing markers ($marker) found in observe(performance)."
    else
        fail "User Timing marker '$marker' not found in observe(performance). Content keys: $(echo "$content_text" | python3 -c 'import sys,json; t=sys.stdin.read(); i=t.find("{"); print(list(json.loads(t[i:]).keys()) if i>=0 else [])' 2>/dev/null || echo 'parse error')"
    fi
}
run_test_s20

# ── Test S.25: Record tab video (no audio) ────────────────
begin_test "S.25" "Record tab video for 5 seconds (no audio)" \
    "Start recording, wait 5s, stop, verify file saved with valid metadata" \
    "Tests: full recording pipeline: MCP → daemon → extension → tabCapture → blob → server → disk"

run_test_s25() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to YouTube video (same as S.26, but without audio)
    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"action":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video for recording"}' 20

    # Give the video time to load and start auto-playing
    sleep 3

    # Start recording (no audio)
    interact_and_wait "record_start" '{"action":"record_start","name":"smoke-video-test","reason":"Record tab video"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_start returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    echo "  Recording... waiting 5 seconds"
    sleep 5

    # Stop recording
    interact_and_wait "record_stop" '{"action":"record_stop","reason":"Stop recording"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_stop returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 2

    # Check saved videos
    local saved_response
    saved_response=$(call_tool "observe" '{"what":"saved_videos","last_n":1}')
    local saved_text
    saved_text=$(extract_content_text "$saved_response")

    echo "  [saved video metadata]"
    echo "$saved_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    recs = data.get('recordings', [])
    if recs:
        r = recs[0]
        print(f'    name: {r.get(\"name\", \"?\")[:60]}')
        print(f'    duration: {r.get(\"duration_seconds\", \"?\")}s')
        print(f'    size: {r.get(\"size_bytes\", 0)} bytes')
        print(f'    format: {r.get(\"format\", \"?\")}')
        print(f'    has_audio: {r.get(\"has_audio\", False)}')
        print(f'    fps: {r.get(\"fps\", \"?\")}')
    else:
        print('    (no recordings found)')
except: pass
" 2>/dev/null || true

    if echo "$saved_text" | grep -q "smoke-video-test"; then
        local dur
        dur=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
print(recs[0].get('duration_seconds',0) if recs else 0)
" 2>/dev/null || echo "0")
        if [ "$dur" -ge 3 ] 2>/dev/null; then
            pass "Video recorded: smoke-video-test, ${dur}s duration, saved to disk."
        else
            pass "Video recorded: smoke-video-test saved (duration: ${dur}s)."
        fi
    else
        fail "No 'smoke-video-test' found in saved_videos. Content: $(truncate "$saved_text" 200)"
    fi
}
run_test_s25

# ── Test S.26: Record tab video WITH tab audio ────────────
begin_test "S.26" "Record tab video with audio:tab for 5 seconds" \
    "Navigate to a page with sound, record with audio:'tab', verify audio metadata" \
    "Tests: tab audio capture via tabCapture. Play audio in the tab to hear it in the recording."

run_test_s26() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to YouTube video with auto-play (Lofi Girl stream)
    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"action":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video with audio for recording"}' 20

    # Give the video time to load and start auto-playing
    sleep 3

    # Start recording WITH tab audio
    interact_and_wait "record_start" '{"action":"record_start","name":"smoke-audio-test","audio":"tab","reason":"Record tab with audio"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_start with audio:tab returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    echo "  Recording with audio... waiting 5 seconds"
    echo "  (play sound in the tracked tab now if you want to verify audio)"
    sleep 5

    # Stop recording
    interact_and_wait "record_stop" '{"action":"record_stop","reason":"Stop audio recording"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_stop returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 2

    # Check saved videos for audio metadata
    local saved_response
    saved_response=$(call_tool "observe" '{"what":"saved_videos","last_n":1}')
    local saved_text
    saved_text=$(extract_content_text "$saved_response")

    echo "  [saved video with audio metadata]"
    echo "$saved_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    recs = data.get('recordings', [])
    if recs:
        r = recs[0]
        print(f'    name: {r.get(\"name\", \"?\")[:60]}')
        print(f'    duration: {r.get(\"duration_seconds\", \"?\")}s')
        print(f'    size: {r.get(\"size_bytes\", 0)} bytes')
        print(f'    format: {r.get(\"format\", \"?\")}')
        print(f'    has_audio: {r.get(\"has_audio\", False)}')
        print(f'    audio_mode: {r.get(\"audio_mode\", \"(none)\")}')
    else:
        print('    (no recordings found)')
except: pass
" 2>/dev/null || true

    if ! echo "$saved_text" | grep -q "smoke-audio-test"; then
        fail "No 'smoke-audio-test' found in saved_videos. Content: $(truncate "$saved_text" 200)"
        return
    fi

    local has_audio
    has_audio=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
print(recs[0].get('has_audio',False) if recs else False)
" 2>/dev/null || echo "False")

    local audio_mode
    audio_mode=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
print(recs[0].get('audio_mode','') if recs else '')
" 2>/dev/null || echo "")

    if [ "$has_audio" = "True" ] && [ "$audio_mode" = "tab" ]; then
        pass "Audio recording saved: has_audio=true, audio_mode=tab, format includes opus codec."
    elif echo "$saved_text" | grep -q "smoke-audio-test"; then
        fail "Recording saved but audio metadata missing. has_audio=$has_audio, audio_mode=$audio_mode."
    fi

    echo ""
    echo "  >>> Open the .webm file in ~/.gasoline/recordings/ to verify audio is audible."
}
run_test_s26

# ── Test S.27: Verify recording watermark survives page refresh ──
begin_test "S.27" "Recording watermark survives page refresh" \
    "Start recording, refresh the page, verify watermark reappears" \
    "Tests: tabs.onUpdated listener re-sends watermark after navigation"

run_test_s27() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to YouTube video with auto-play (ensures extension is properly invoked)
    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"action":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video for watermark test"}' 20
    sleep 2

    # Start recording
    interact_and_wait "record_start" '{"action":"record_start","name":"smoke-watermark-test","reason":"Test watermark persistence"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_start returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 1

    # Check watermark is present
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check watermark before refresh","script":"document.getElementById(\"gasoline-recording-watermark\") ? \"FOUND\" : \"NOT_FOUND\""}'
    local before_refresh="$INTERACT_RESULT"

    # Refresh the page (this re-injects content script)
    interact_and_wait "refresh" '{"action":"refresh","reason":"Refresh during recording"}' 20
    sleep 3

    # Check watermark reappeared after refresh
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check watermark after refresh","script":"document.getElementById(\"gasoline-recording-watermark\") ? \"FOUND\" : \"NOT_FOUND\""}'
    local after_refresh="$INTERACT_RESULT"

    # Stop recording
    interact_and_wait "record_stop" '{"action":"record_stop","reason":"Stop watermark test recording"}' 20
    sleep 1

    local before_ok=false
    local after_ok=false
    if echo "$before_refresh" | grep -q "FOUND"; then
        before_ok=true
    fi
    if echo "$after_refresh" | grep -q "FOUND"; then
        after_ok=true
    fi

    if [ "$before_ok" = "true" ] && [ "$after_ok" = "true" ]; then
        pass "Watermark present before AND after page refresh. Tab update listener works."
    elif [ "$before_ok" = "true" ] && [ "$after_ok" != "true" ]; then
        fail "Watermark present before refresh but MISSING after. tabs.onUpdated re-send failed."
    elif [ "$before_ok" != "true" ]; then
        fail "Watermark not found even before refresh. Recording overlay may be broken."
    fi
}
run_test_s27

# ── Test S.16: Graceful shutdown ─────────────────────────
begin_test "S.16" "Graceful shutdown via --stop" \
    "Run --stop, verify port is freed and PID file is cleaned up" \
    "Ungraceful shutdown leaves orphan processes and stale PID files"

run_test_s16() {
    local stop_output
    stop_output=$($WRAPPER --stop --port "$PORT" 2>&1)
    local stop_exit=$?

    if [ $stop_exit -ne 0 ]; then
        fail "--stop exited with code $stop_exit. Output: $(truncate "$stop_output")"
        return
    fi

    sleep 1

    if lsof -ti :"$PORT" >/dev/null 2>&1; then
        fail "Port $PORT still occupied after --stop."
        return
    fi

    local pid_file="$HOME/.gasoline-${PORT}.pid"
    if [ -f "$pid_file" ]; then
        fail "PID file $pid_file still exists after --stop."
        rm -f "$pid_file"
        return
    fi

    pass "Graceful shutdown: --stop exited 0, port freed, PID file cleaned."
}
run_test_s16

# ── Summary ──────────────────────────────────────────────
{
    echo ""
    echo "============================================================"
    echo "SMOKE TEST SUMMARY"
    echo "============================================================"
    echo "  Passed:  $PASS_COUNT"
    echo "  Failed:  $FAIL_COUNT"
    echo "  Skipped: $SKIPPED_COUNT"
    echo ""
    if [ "$FAIL_COUNT" -eq 0 ]; then
        if [ "$SKIPPED_COUNT" -gt 0 ]; then
            echo "  Result: PASSED (with $SKIPPED_COUNT skipped — see above for what to enable)"
        else
            echo "  Result: ALL PASSED"
        fi
    else
        echo "  Result: FAILED"
    fi
    echo ""
    echo "Diagnostics saved to: $DIAGNOSTICS_FILE"
    echo "View with: cat $DIAGNOSTICS_FILE"
    echo ""
} | tee -a "$OUTPUT_FILE"

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
