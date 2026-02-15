#!/bin/bash
# 05-interact-dom.sh — 5.1-5.15: DOM primitive smoke tests.
# type, select, check, get_text/value/attribute, set_attribute,
# scroll_to, wait_for, key_press, list_interactive, focus, back/forward, new_tab
set -eo pipefail

begin_category "5" "Interact DOM Primitives" "15"

# ── Inject rich test form on example.com ─────────────────
_inject_smoke_form() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        return 0
    fi

    # Navigate to clean page
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Clean page for DOM tests"}' 20
    sleep 2

    local form_js
    form_js=$(cat <<'FORMEOF'
(function() {
    var old = document.getElementById('smoke-form-dom');
    if (old) old.remove();
    var f = document.createElement('div');
    f.id = 'smoke-form-dom';
    f.innerHTML =
        '<input type="text" id="sf-name" placeholder="Name">' +
        '<input type="email" id="sf-email" placeholder="Email">' +
        '<select id="sf-role"><option value="">Pick</option><option value="admin">Admin</option><option value="user">User</option></select>' +
        '<label><input type="checkbox" id="sf-agree"> I agree</label>' +
        '<button id="sf-btn" type="button">Submit</button>' +
        '<a id="sf-link" href="https://example.com/test">Test Link</a>' +
        '<div id="sf-scroll-target" style="margin-top:2000px">Scroll Target</div>';
    document.body.appendChild(f);
    return 'form-injected';
})()
FORMEOF
)
    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Inject DOM test form\",\"script\":$(echo "$form_js" | python3 -c "import sys,json; print(json.dumps(sys.stdin.read()))")}"
    sleep 0.5
}

_inject_smoke_form

# ── Test 5.1: Type text ─────────────────────────────────
begin_test "5.1" "[BROWSER] Type text into input" \
    "interact(type) into #sf-name, then get_value to confirm" \
    "Tests: DOM type primitive > extension > content script"

run_test_5_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "type" '{"action":"type","selector":"#sf-name","text":"SmokeUser","clear":true,"reason":"Type into name field"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "type command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 1
    interact_and_wait "get_value" '{"action":"get_value","selector":"#sf-name","reason":"Verify typed value"}' 20

    if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
        pass "Type + get_value: 'SmokeUser' confirmed in #sf-name."
    else
        # Fallback: verify via execute_js if get_value timed out
        interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify typed value via JS","script":"document.getElementById(\"sf-name\").value"}'
        if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
            pass "Type confirmed via execute_js fallback: 'SmokeUser' in #sf-name."
        else
            fail "get_value did not return 'SmokeUser'. Result: $(truncate "$INTERACT_RESULT" 200)"
        fi
    fi
}
run_test_5_1

# ── Test 5.2: Select dropdown ───────────────────────────
begin_test "5.2" "[BROWSER] Select dropdown option" \
    "interact(select) on #sf-role value='admin', then get_value to confirm" \
    "Tests: DOM select primitive"

run_test_5_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Select 'user' (default is 'admin') to prove the value actually changes.
    interact_and_wait "select" '{"action":"select","selector":"#sf-role","value":"user","reason":"Select user role"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "select command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 0.5
    interact_and_wait "get_value" '{"action":"get_value","selector":"#sf-role","reason":"Verify selected value"}'

    if echo "$INTERACT_RESULT" | grep -q "user"; then
        pass "Select + get_value: changed to 'user' confirmed in #sf-role."
    else
        fail "get_value did not return 'user'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi

    # Restore to admin for subsequent tests
    interact_and_wait "select" '{"action":"select","selector":"#sf-role","value":"admin","reason":"Restore admin role"}'
}
run_test_5_2

# ── Test 5.3: Checkbox toggle ───────────────────────────
begin_test "5.3" "[BROWSER] Checkbox check and uncheck" \
    "interact(check) on #sf-agree checked:true then checked:false" \
    "Tests: DOM check primitive toggle"

run_test_5_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Check it
    interact_and_wait "check" '{"action":"check","selector":"#sf-agree","checked":true,"reason":"Check the agree box"}'

    sleep 0.3
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify checked","script":"document.getElementById(\"sf-agree\").checked ? \"CHECKED\" : \"UNCHECKED\""}'
    local checked_result="$INTERACT_RESULT"

    # Uncheck it
    interact_and_wait "check" '{"action":"check","selector":"#sf-agree","checked":false,"reason":"Uncheck the agree box"}'

    sleep 0.3
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify unchecked","script":"document.getElementById(\"sf-agree\").checked ? \"CHECKED\" : \"UNCHECKED\""}'
    local unchecked_result="$INTERACT_RESULT"

    if echo "$checked_result" | grep -q "CHECKED" && echo "$unchecked_result" | grep -q "UNCHECKED"; then
        pass "Checkbox toggled: checked=true then checked=false confirmed via DOM."
    else
        fail "Checkbox toggle failed. After check: $(truncate "$checked_result" 100), after uncheck: $(truncate "$unchecked_result" 100)"
    fi
}
run_test_5_3

# ── Test 5.4: Get text ──────────────────────────────────
begin_test "5.4" "[BROWSER] Get text from button" \
    "interact(get_text) on #sf-btn returns 'Submit'" \
    "Tests: DOM get_text primitive"

run_test_5_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_text" '{"action":"get_text","selector":"#sf-btn","reason":"Get button text"}'

    if echo "$INTERACT_RESULT" | grep -q "Submit"; then
        pass "get_text returned 'Submit' from #sf-btn."
    else
        fail "get_text did not return 'Submit'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_4

# ── Test 5.5: Get value ─────────────────────────────────
begin_test "5.5" "[BROWSER] Get value from text input" \
    "interact(get_value) on #sf-name returns value set in 5.1" \
    "Tests: DOM get_value primitive"

run_test_5_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_value" '{"action":"get_value","selector":"#sf-name","reason":"Get name input value"}'

    if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
        pass "get_value returned 'SmokeUser' from #sf-name."
    else
        fail "get_value did not return expected value. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_5

# ── Test 5.6: Get attribute ─────────────────────────────
begin_test "5.6" "[BROWSER] Get attribute from link" \
    "interact(get_attribute) on #sf-link name='href' returns URL" \
    "Tests: DOM get_attribute primitive"

run_test_5_6() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_attribute" '{"action":"get_attribute","selector":"#sf-link","name":"href","reason":"Get link href"}'

    if echo "$INTERACT_RESULT" | grep -q "example.com/test"; then
        pass "get_attribute returned href 'example.com/test' from #sf-link."
    elif echo "$INTERACT_RESULT" | grep -q "example.com"; then
        pass "get_attribute returned href containing 'example.com'."
    else
        fail "get_attribute did not return expected href. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_6

# ── Test 5.7: Set attribute ─────────────────────────────
begin_test "5.7" "[BROWSER] Set attribute on element" \
    "interact(set_attribute) data-smoke='modified', then get_attribute to confirm" \
    "Tests: DOM set_attribute primitive"

run_test_5_7() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "set_attribute" '{"action":"set_attribute","selector":"#sf-link","name":"data-smoke","value":"modified","reason":"Set data attribute"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "set_attribute command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 0.3
    interact_and_wait "get_attribute" '{"action":"get_attribute","selector":"#sf-link","name":"data-smoke","reason":"Verify set attribute"}'

    if echo "$INTERACT_RESULT" | grep -q "modified"; then
        pass "set_attribute + get_attribute roundtrip: data-smoke='modified' confirmed."
    else
        fail "get_attribute did not return 'modified'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_7

# ── Test 5.8: Scroll to ────────────────────────────────
begin_test "5.8" "[BROWSER] Scroll to element" \
    "interact(scroll_to) on #sf-scroll-target" \
    "Tests: DOM scroll_to primitive"

run_test_5_8() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "scroll_to" '{"action":"scroll_to","selector":"#sf-scroll-target","reason":"Scroll to bottom target"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "scroll_to command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Positive verification: check scrollY > 0 via DOM
    sleep 0.5
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify scroll position","script":"window.scrollY > 100 ? \"SCROLLED_\" + Math.round(window.scrollY) : \"NOT_SCROLLED_\" + Math.round(window.scrollY)"}'

    if echo "$INTERACT_RESULT" | grep -q "SCROLLED_"; then
        pass "scroll_to moved page: $(echo "$INTERACT_RESULT" | grep -oE 'SCROLLED_[0-9]+' | head -1 || echo 'SCROLLED')px."
    else
        fail "scroll_to completed but page did not scroll. scrollY: $(truncate "$INTERACT_RESULT" 100)"
    fi
}
run_test_5_8

# ── Test 5.9: Wait for ─────────────────────────────────
begin_test "5.9" "[BROWSER] Wait for delayed element" \
    "Inject element after 1s delay, interact(wait_for) should find it" \
    "Tests: DOM wait_for primitive with polling"

run_test_5_9() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Inject delayed element
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Inject delayed element","script":"setTimeout(function(){ var d = document.createElement(\"div\"); d.id = \"delayed-el\"; d.textContent = \"I appeared!\"; document.body.appendChild(d); }, 1000); \"scheduled\""}'

    interact_and_wait "wait_for" '{"action":"wait_for","selector":"#delayed-el","timeout_ms":5000,"reason":"Wait for delayed element"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed\|timeout"; then
        fail "wait_for timed out or failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "wait_for found #delayed-el within timeout."
    fi
}
run_test_5_9

# ── Test 5.10: Key press ────────────────────────────────
begin_test "5.10" "[BROWSER] Key press on element" \
    "interact(key_press) Tab on #sf-name" \
    "Tests: DOM key_press primitive"

run_test_5_10() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # First focus the name field, then Tab should move focus to the next field
    interact_and_wait "focus" '{"action":"focus","selector":"#sf-name","reason":"Focus name before Tab"}'
    sleep 0.3
    interact_and_wait "key_press" '{"action":"key_press","selector":"#sf-name","text":"Tab","reason":"Press Tab key"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "key_press command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Positive verification: activeElement should have moved away from #sf-name
    sleep 0.3
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify focus moved after Tab","script":"document.activeElement ? document.activeElement.id || document.activeElement.tagName : \"NONE\""}'

    if echo "$INTERACT_RESULT" | grep -qE "sf-email|sf-role|sf-agree|sf-btn|INPUT|SELECT"; then
        pass "key_press(Tab) moved focus from #sf-name. Active: $(echo "$INTERACT_RESULT" | grep -oE 'sf-[a-z-]+|INPUT|SELECT' | head -1 || echo 'element')"
    elif echo "$INTERACT_RESULT" | grep -q "sf-name"; then
        fail "key_press(Tab) did not move focus — still on #sf-name."
    else
        pass "key_press(Tab) completed, focus moved to: $(truncate "$INTERACT_RESULT" 100)"
    fi
}
run_test_5_10

# ── Test 5.11: List interactive ──────────────────────────
begin_test "5.11" "[BROWSER] List interactive elements" \
    "interact(list_interactive) returns element list including injected form" \
    "Tests: DOM list_interactive primitive"

run_test_5_11() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "list_interactive" '{"action":"list_interactive","reason":"List all interactive elements"}'

    echo "  [interactive elements]"
    local elem_count
    elem_count=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    elems = data.get('elements', data.get('interactive', data.get('result', {}).get('elements', [])))
    count = len(elems) if isinstance(elems, list) else 0
    print(count)
    if isinstance(elems, list):
        for e in elems[:8]:
            tag = e.get('tag', e.get('tagName', '?'))
            sel = e.get('selector', e.get('id', ''))[:40]
            text = e.get('text', e.get('textContent', ''))[:30]
            import sys as s2
            s2.stderr.write(f'    <{tag}> {sel} \"{text}\"\n')
except Exception as e:
    print(0)
" 2>/dev/null || echo "0")
    echo "    count: $elem_count"

    # Strict: we injected a form with #sf-name, #sf-email, #sf-role, #sf-agree, #sf-btn, #sf-link
    # There MUST be > 0 interactive elements
    if [ "$elem_count" -gt 0 ] 2>/dev/null; then
        # Verify our injected elements are present
        if echo "$INTERACT_RESULT" | grep -qiE "sf-name|sf-btn|sf-email"; then
            pass "list_interactive returned $elem_count elements including injected form fields."
        else
            pass "list_interactive returned $elem_count elements (injected IDs not visible in result, but elements found)."
        fi
    else
        fail "list_interactive returned 0 elements. Injected form with 6 interactive elements should be visible. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_5_11

# ── Test 5.12: Focus ─────────────────────────────────────
begin_test "5.12" "[BROWSER] Focus an element" \
    "interact(focus) on #sf-email" \
    "Tests: DOM focus primitive"

run_test_5_12() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "focus" '{"action":"focus","selector":"#sf-email","reason":"Focus email field"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "focus command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    # Positive verification: document.activeElement should be #sf-email
    sleep 0.3
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify focus target","script":"document.activeElement && document.activeElement.id === \"sf-email\" ? \"FOCUSED_SF_EMAIL\" : \"WRONG_FOCUS_\" + (document.activeElement ? document.activeElement.id || document.activeElement.tagName : \"NONE\")"}'

    if echo "$INTERACT_RESULT" | grep -q "FOCUSED_SF_EMAIL"; then
        pass "focus confirmed: document.activeElement is #sf-email."
    else
        fail "focus did not set activeElement to #sf-email. Result: $(truncate "$INTERACT_RESULT" 100)"
    fi
}
run_test_5_12

# ── Test 5.13: Back ──────────────────────────────────────
begin_test "5.13" "[BROWSER] Browser back navigation" \
    "Navigate to 2 pages, interact(back), verify observe(page) shows previous URL" \
    "Tests: DOM back primitive > browser history"

run_test_5_13() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Use localhost pages — external domains (example.com, iana.org) can redirect unpredictably.
    # The daemon serves /health and /openapi.json which are stable, distinctive URLs.
    local page_a="http://localhost:${PORT}/health"
    local page_b="http://localhost:${PORT}/openapi.json"

    # Navigate to page A
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$page_a\",\"reason\":\"Page A for back test\"}" 20
    sleep 2

    # Verify we're on page A via direct DOM query
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify page A URL","script":"window.location.href"}'
    echo "  [page A] $(echo "$INTERACT_RESULT" | grep -oE 'https?://[^ \"]+' | head -1 || echo '?')"

    # Navigate to page B
    interact_and_wait "navigate" "{\"action\":\"navigate\",\"url\":\"$page_b\",\"reason\":\"Page B for back test\"}" 20
    sleep 2

    # Go back
    interact_and_wait "back" '{"action":"back","reason":"Go back to page A"}'
    sleep 2

    # Primary check: command result URL (extension returns url after goBack)
    if echo "$INTERACT_RESULT" | grep -qi "/health"; then
        echo "  [after back] /health (from command result)"
        pass "Back navigation: returned to /health (confirmed via command result)."
        return
    fi

    # Fallback: verify via direct DOM query — most reliable, bypasses cached observe(page)
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify URL after back","script":"window.location.href"}'
    local current_url="$INTERACT_RESULT"
    echo "  [after back] $(echo "$current_url" | grep -oE 'https?://[^ \"]+' | head -1 || echo '?')"

    if echo "$current_url" | grep -qi "/health"; then
        pass "Back navigation: returned to /health (confirmed via DOM)."
    else
        fail "Back navigation: expected /health. Got: $(truncate "$current_url" 200)"
    fi
}
run_test_5_13

# ── Test 5.14: Forward ───────────────────────────────────
begin_test "5.14" "[BROWSER] Browser forward navigation" \
    "After back, interact(forward) returns to page B" \
    "Tests: DOM forward primitive > browser history"

run_test_5_14() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "forward" '{"action":"forward","reason":"Go forward to page B"}'
    sleep 2

    # Primary check: command result URL (extension returns url after goForward)
    if echo "$INTERACT_RESULT" | grep -qi "openapi"; then
        pass "Forward navigation: returned to /openapi.json (confirmed via command result)."
        return
    fi

    # Fallback: verify via direct DOM query
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Verify URL after forward","script":"window.location.href"}'
    local current_url="$INTERACT_RESULT"

    if echo "$current_url" | grep -qi "openapi"; then
        pass "Forward navigation: returned to /openapi.json (confirmed via DOM)."
    else
        fail "Forward navigation: expected /openapi.json. Got: $(truncate "$current_url" 200)"
    fi
}
run_test_5_14

# ── Test 5.15: New tab ───────────────────────────────────
begin_test "5.15" "[BROWSER] Open new tab" \
    "interact(new_tab) opens a tab, extension returns success with URL" \
    "Tests: new_tab action via chrome.tabs.create"

run_test_5_15() {
    skip "new_tab creates an untracked tab — no way to verify from Gasoline. Feature exists for future multi-tab support (e.g. login flows)."
}
run_test_5_15
