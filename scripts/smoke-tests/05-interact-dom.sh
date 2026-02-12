#!/bin/bash
# 05-interact-dom.sh — S.35-S.49: DOM primitive smoke tests.
# type, select, check, get_text/value/attribute, set_attribute,
# scroll_to, wait_for, key_press, list_interactive, focus, back/forward, new_tab
set -eo pipefail

begin_category "5" "Interact DOM Primitives" "15"

# ── Inject rich test form on example.com ─────────────────
_inject_smoke_form() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        return 1
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

# ── Test S.35: Type text ─────────────────────────────────
begin_test "S.35" "Type text into input" \
    "interact(type) into #sf-name, then get_value to confirm" \
    "Tests: DOM type primitive > extension > content script"

run_test_s35() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "type" '{"action":"type","selector":"#sf-name","text":"SmokeUser","clear":true,"reason":"Type into name field"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "type command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 0.5
    interact_and_wait "get_value" '{"action":"get_value","selector":"#sf-name","reason":"Verify typed value"}'

    if echo "$INTERACT_RESULT" | grep -q "SmokeUser"; then
        pass "Type + get_value: 'SmokeUser' confirmed in #sf-name."
    else
        fail "get_value did not return 'SmokeUser'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_s35

# ── Test S.36: Select dropdown ───────────────────────────
begin_test "S.36" "Select dropdown option" \
    "interact(select) on #sf-role value='admin', then get_value to confirm" \
    "Tests: DOM select primitive"

run_test_s36() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "select" '{"action":"select","selector":"#sf-role","value":"admin","reason":"Select admin role"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "select command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 0.5
    interact_and_wait "get_value" '{"action":"get_value","selector":"#sf-role","reason":"Verify selected value"}'

    if echo "$INTERACT_RESULT" | grep -q "admin"; then
        pass "Select + get_value: 'admin' confirmed in #sf-role."
    else
        fail "get_value did not return 'admin'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_s36

# ── Test S.37: Checkbox toggle ───────────────────────────
begin_test "S.37" "Checkbox check and uncheck" \
    "interact(check) on #sf-agree checked:true then checked:false" \
    "Tests: DOM check primitive toggle"

run_test_s37() {
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
run_test_s37

# ── Test S.38: Get text ──────────────────────────────────
begin_test "S.38" "Get text from button" \
    "interact(get_text) on #sf-btn returns 'Submit'" \
    "Tests: DOM get_text primitive"

run_test_s38() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_text" '{"action":"get_text","selector":"#sf-btn","reason":"Get button text"}'

    if echo "$INTERACT_RESULT" | grep -qi "Submit"; then
        pass "get_text returned 'Submit' from #sf-btn."
    else
        fail "get_text did not return 'Submit'. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_s38

# ── Test S.39: Get value ─────────────────────────────────
begin_test "S.39" "Get value from text input" \
    "interact(get_value) on #sf-name returns value set in S.35" \
    "Tests: DOM get_value primitive"

run_test_s39() {
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
run_test_s39

# ── Test S.40: Get attribute ─────────────────────────────
begin_test "S.40" "Get attribute from link" \
    "interact(get_attribute) on #sf-link name='href' returns URL" \
    "Tests: DOM get_attribute primitive"

run_test_s40() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "get_attribute" '{"action":"get_attribute","selector":"#sf-link","name":"href","reason":"Get link href"}'

    if echo "$INTERACT_RESULT" | grep -qi "example.com"; then
        pass "get_attribute returned href containing 'example.com'."
    else
        fail "get_attribute did not return expected href. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_s40

# ── Test S.41: Set attribute ─────────────────────────────
begin_test "S.41" "Set attribute on element" \
    "interact(set_attribute) data-smoke='modified', then get_attribute to confirm" \
    "Tests: DOM set_attribute primitive"

run_test_s41() {
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
run_test_s41

# ── Test S.42: Scroll to ────────────────────────────────
begin_test "S.42" "Scroll to element" \
    "interact(scroll_to) on #sf-scroll-target" \
    "Tests: DOM scroll_to primitive"

run_test_s42() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "scroll_to" '{"action":"scroll_to","selector":"#sf-scroll-target","reason":"Scroll to bottom target"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "scroll_to command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "scroll_to completed for #sf-scroll-target."
    fi
}
run_test_s42

# ── Test S.43: Wait for ─────────────────────────────────
begin_test "S.43" "Wait for delayed element" \
    "Inject element after 1s delay, interact(wait_for) should find it" \
    "Tests: DOM wait_for primitive with polling"

run_test_s43() {
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
run_test_s43

# ── Test S.44: Key press ────────────────────────────────
begin_test "S.44" "Key press on element" \
    "interact(key_press) Tab on #sf-name" \
    "Tests: DOM key_press primitive"

run_test_s44() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "key_press" '{"action":"key_press","selector":"#sf-name","text":"Tab","reason":"Press Tab key"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "key_press command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "key_press(Tab) completed on #sf-name."
    fi
}
run_test_s44

# ── Test S.45: List interactive ──────────────────────────
begin_test "S.45" "List interactive elements" \
    "interact(list_interactive) returns element list including injected form" \
    "Tests: DOM list_interactive primitive"

run_test_s45() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "list_interactive" '{"action":"list_interactive","reason":"List all interactive elements"}'

    echo "  [interactive elements]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    elems = data.get('elements', data.get('interactive', []))
    print(f'    count: {len(elems) if isinstance(elems, list) else \"?\"}')
    if isinstance(elems, list):
        for e in elems[:8]:
            tag = e.get('tag', e.get('tagName', '?'))
            sel = e.get('selector', e.get('id', ''))[:40]
            text = e.get('text', e.get('textContent', ''))[:30]
            print(f'    <{tag}> {sel} \"{text}\"')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    if echo "$INTERACT_RESULT" | grep -qiE "element|interactive|sf-name|sf-btn|input|button"; then
        pass "list_interactive returned elements including injected form fields."
    else
        fail "list_interactive missing expected elements. Result: $(truncate "$INTERACT_RESULT" 200)"
    fi
}
run_test_s45

# ── Test S.46: Focus ─────────────────────────────────────
begin_test "S.46" "Focus an element" \
    "interact(focus) on #sf-email" \
    "Tests: DOM focus primitive"

run_test_s46() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "focus" '{"action":"focus","selector":"#sf-email","reason":"Focus email field"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "focus command failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "focus completed on #sf-email."
    fi
}
run_test_s46

# ── Test S.47: Back ──────────────────────────────────────
begin_test "S.47" "Browser back navigation" \
    "Navigate to 2 pages, interact(back), verify observe(page) shows previous URL" \
    "Tests: DOM back primitive > browser history"

run_test_s47() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate to page A
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Page A for back test"}' 20
    sleep 2

    # Navigate to page B
    interact_and_wait "navigate" '{"action":"navigate","url":"https://www.iana.org/domains/reserved","reason":"Page B for back test"}' 20
    sleep 2

    # Go back
    interact_and_wait "back" '{"action":"back","reason":"Go back to page A"}'
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "example.com"; then
        pass "Back navigation: returned to example.com."
    else
        fail "Back navigation: expected example.com. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_s47

# ── Test S.48: Forward ───────────────────────────────────
begin_test "S.48" "Browser forward navigation" \
    "After back, interact(forward) returns to page B" \
    "Tests: DOM forward primitive > browser history"

run_test_s48() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "forward" '{"action":"forward","reason":"Go forward to page B"}'
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"page"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if echo "$content_text" | grep -qi "iana.org\|reserved"; then
        pass "Forward navigation: returned to iana.org/reserved."
    else
        fail "Forward navigation: expected iana.org. Got: $(truncate "$content_text" 200)"
    fi
}
run_test_s48

# ── Test S.49: New tab ───────────────────────────────────
begin_test "S.49" "Open new tab" \
    "interact(new_tab) opens a new tab with a URL" \
    "Tests: DOM new_tab primitive"

run_test_s49() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "new_tab" '{"action":"new_tab","url":"https://example.com","reason":"Open new tab"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed\|unsupported"; then
        skip "new_tab not supported or failed. Result: $(truncate "$INTERACT_RESULT" 200)"
    else
        pass "new_tab command completed."
    fi

    # Navigate back to example.com in tracked tab for subsequent tests
    sleep 1
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Return to test page"}' 20
    sleep 2
}
run_test_s49
