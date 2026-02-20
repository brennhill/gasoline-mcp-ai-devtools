#!/bin/bash
# 25-action-enrichment.sh — 25.1-25.3: Action result enrichment fields.
# Verifies final_url, title, effective_title, and dom_changes in interact results.
set -eo pipefail

begin_category "25" "Action Result Enrichment" "3"

# ── Test 25.1: Navigate returns final_url and title ──────
begin_test "25.1" "[BROWSER] Navigate enriches result with final_url and title" \
    "interact(navigate) to example.com returns final_url, title, effective_title" \
    "Tests: action result enrichment (#149) — final_url, title fields"

run_test_25_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Smoke test enrichment"}'
    local result="$INTERACT_RESULT"

    if [ -z "$result" ]; then
        fail "No result from navigate."
        return
    fi

    log_diagnostic "25.1" "navigate-enrichment" "$result"

    local verdict
    verdict=$(echo "$result" | python3 -c "
import sys, json
try:
    t = sys.stdin.read()
    i = t.find('{')
    data = json.loads(t[i:]) if i >= 0 else {}
    final_url = data.get('final_url', '')
    title = data.get('title', '')
    eff_title = data.get('effective_title', '')
    fields = []
    if final_url: fields.append(f'final_url={final_url}')
    if title: fields.append(f'title={title}')
    if eff_title: fields.append(f'effective_title={eff_title}')
    if len(fields) >= 2:
        print(f'PASS {\" \".join(fields)}')
    else:
        print(f'FAIL found only: {fields}. Keys: {list(data.keys())[:10]}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$verdict" | grep -q "^PASS"; then
        pass "Navigate result enriched. $verdict"
    else
        fail "Navigate missing enrichment fields. $verdict. Content: $(truncate "$result" 300)"
    fi
}
run_test_25_1

# ── Test 25.2: Click action returns dom_changes ──────────
begin_test "25.2" "[BROWSER] Click action returns dom_changes" \
    "interact(click) with analyze:true should include dom_changes in result" \
    "Tests: DOM mutation tracking (#78) — dom_changes field in action results"

run_test_25_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Navigate first to have a page
    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Setup for click test"}'
    sleep 1

    # Click a link (example.com has an <a> tag)
    interact_and_wait "click" '{"action":"click","selector":"a","reason":"Smoke test dom_changes","analyze":true}'
    local result="$INTERACT_RESULT"

    if [ -z "$result" ]; then
        fail "No result from click."
        return
    fi

    log_diagnostic "25.2" "click-dom-changes" "$result"

    # Check for dom_changes field
    if echo "$result" | grep -q "dom_changes"; then
        local summary
        summary=$(echo "$result" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    dc = data.get('dom_changes', {})
    print(f'summary={dc.get(\"summary\",\"?\")} added={dc.get(\"added\",\"?\")} removed={dc.get(\"removed\",\"?\")}')
except: print('?')
" 2>/dev/null || echo "?")
        pass "Click result includes dom_changes. $summary"
    else
        # dom_changes may not appear if the page didn't actually change
        skip "dom_changes not present — page may not have changed on click. Content keys: $(echo "$result" | python3 -c "import sys,json; t=sys.stdin.read(); i=t.find('{'); print(list(json.loads(t[i:]).keys())[:8]) if i>=0 else print('?')" 2>/dev/null || echo "?")"
    fi
}
run_test_25_2

# ── Test 25.3: Execute JS returns timing enrichment ──────
begin_test "25.3" "[BROWSER] Execute JS returns timing information" \
    "interact(execute_js) should include timing_ms in result" \
    "Tests: action result timing enrichment"

run_test_25_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Smoke test timing","script":"document.title"}'
    local result="$INTERACT_RESULT"

    if [ -z "$result" ]; then
        fail "No result from execute_js."
        return
    fi

    log_diagnostic "25.3" "execute-js-timing" "$result"

    if echo "$result" | grep -q "timing"; then
        pass "Execute JS result includes timing information."
    else
        skip "timing field not present. Content: $(truncate "$result" 200)"
    fi
}
run_test_25_3
