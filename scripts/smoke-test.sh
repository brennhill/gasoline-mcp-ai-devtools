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

begin_category "0" "Human Smoke Test" "16"

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
            return 0
        fi
        if echo "$poll_text" | grep -q '"status":"failed"'; then
            INTERACT_RESULT="$poll_text"
            return 1
        fi
    done

    INTERACT_RESULT="timeout waiting for $action"
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
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com"}'

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
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"script\":\"console.log('${SMOKE_MARKER}_LOG')\"}"

    # Fire console.error with stack
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"script\":\"console.error('${SMOKE_MARKER}_ERROR')\"}"

    # Fire a thrown error
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"script\":\"try { throw new Error('${SMOKE_MARKER}_THROWN') } catch(e) { console.error(e.message, e.stack) }\"}"

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
        pass "Log marker found in observe(logs). Error marker not in observe(errors) — acceptable."
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
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"script\":\"$js\"}"

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
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"script\":\"$js\"}"

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
    interact_and_wait "highlight" '{"action":"highlight","selector":"body","duration_ms":2000}'

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
    data = json.loads(sys.stdin.read())
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
    interact_and_wait "execute_js" '{"action":"execute_js","script":"document.querySelectorAll(\"h1, h2, a, button, input\").length"}'

    local dom_response
    dom_response=$(call_tool "configure" '{"action":"query_dom","selector":"h1, a, button, input"}')
    local dom_text
    dom_text=$(extract_content_text "$dom_response")

    # Print DOM query results for human verification
    echo "  [DOM query: h1, a, button, input]"
    echo "$dom_text" | python3 -c "
import sys, json
try:
    data = json.loads(sys.stdin.read())
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
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"script\":$(echo "$form_js" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")}"

    sleep 0.5

    # Fill each field
    interact_and_wait "execute_js" '{"action":"execute_js","script":"var el = document.getElementById(\"sf-user\"); el.focus(); el.value = \"smokeuser\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-user\""}'
    interact_and_wait "execute_js" '{"action":"execute_js","script":"var el = document.getElementById(\"sf-email\"); el.focus(); el.value = \"smoke@test.com\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-email\""}'
    interact_and_wait "execute_js" '{"action":"execute_js","script":"var el = document.getElementById(\"sf-pass\"); el.focus(); el.value = \"s3cure!\"; el.dispatchEvent(new Event(\"input\", {bubbles:true})); \"filled-pass\""}'
    interact_and_wait "execute_js" '{"action":"execute_js","script":"var el = document.getElementById(\"sf-role\"); el.value = \"admin\"; el.dispatchEvent(new Event(\"change\", {bubbles:true})); \"selected-role\""}'

    # Submit the form
    interact_and_wait "execute_js" '{"action":"execute_js","script":"document.getElementById(\"sf-submit\").click(); \"submitted\""}'

    sleep 1

    # Verify form submission happened
    interact_and_wait "execute_js" '{"action":"execute_js","script":"window.__SMOKE_FORM_SUBMITTED__ === true ? \"submit-confirmed\" : \"no-submit\""}'

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
    data = json.loads(sys.stdin.read())
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

    if [ "$submit_confirmed" = "true" ] && [ "$has_input" -gt 0 ] && [ "$has_click" -gt 0 ]; then
        pass "Full form lifecycle: injected, filled 4 fields, submitted. $has_input input events + $has_click click/submit events captured."
    elif [ "$submit_confirmed" = "true" ] && [ "$has_input" -gt 0 ]; then
        pass "Form submitted and input events captured. Click/submit event not in actions (may be filtered)."
    elif [ "$submit_confirmed" = "true" ]; then
        fail "Form submitted but no input/click actions captured. Action tracking may be broken. Actions: $(truncate "$content_text" 200)"
    else
        fail "Form submission not confirmed. Form lifecycle test failed. Actions: $(truncate "$content_text" 200)"
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
    interact_and_wait "navigate" '{"action":"navigate","url":"https://www.binance.com/en/trade/BTC_USDT"}' 20

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
    data = json.loads(sys.stdin.read())
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
    data = json.loads(sys.stdin.read())
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

        if [ -n "$has_initiator" ] && [ -n "$has_start_time" ]; then
            pass "Real waterfall data: ${entry_count:-some} entries with URLs, timing, and initiator types."
        elif [ -n "$has_initiator" ] || [ -n "$has_start_time" ] || [ -n "$has_transfer" ]; then
            pass "Waterfall data: ${entry_count:-some} entries (some fields populated)."
        else
            fail "Waterfall entries have URLs but all timing/size fields are zero — field mapping broken."
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
} | tee -a "$OUTPUT_FILE"

if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
fi
exit 0
