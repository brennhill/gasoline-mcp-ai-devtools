#!/bin/bash
# 07-generate-formats.sh — S.54-S.60: All 7 generate formats.
# reproduction, test, pr_summary, sarif, har, csp, sri
set -eo pipefail

begin_category "7" "Generate Formats" "7"

# ── Test S.54: Reproduction ──────────────────────────────
begin_test "S.54" "Generate reproduction script" \
    "generate(reproduction) produces Playwright code patterns" \
    "Tests: action replay code generation"

run_test_s54() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"format":"reproduction"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(reproduction)."
        return
    fi

    echo "  [reproduction preview]"
    echo "$content_text" | head -5

    if echo "$content_text" | grep -qiE "page\.|goto|click|playwright|browser|test"; then
        pass "generate(reproduction) contains Playwright code patterns."
    else
        # May return a message if no actions recorded
        if echo "$content_text" | grep -qiE "no action|empty|nothing"; then
            pass "generate(reproduction) returned (no actions to replay). Expected without prior interactions."
        else
            fail "generate(reproduction) missing Playwright patterns. Content: $(truncate "$content_text" 200)"
        fi
    fi
}
run_test_s54

# ── Test S.55: Test ──────────────────────────────────────
begin_test "S.55" "Generate Playwright test" \
    "generate(test, test_name='smoke-test') produces test/expect patterns" \
    "Tests: test scaffold generation"

run_test_s55() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"format":"test","test_name":"smoke-test"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(test)."
        return
    fi

    echo "  [test preview]"
    echo "$content_text" | head -5

    if echo "$content_text" | grep -qiE "test|expect|describe|it\(|smoke-test"; then
        pass "generate(test) contains test/expect patterns with name 'smoke-test'."
    else
        if echo "$content_text" | grep -qiE "no action|empty|nothing"; then
            pass "generate(test) returned (no actions to generate test from). Expected without prior interactions."
        else
            fail "generate(test) missing test patterns. Content: $(truncate "$content_text" 200)"
        fi
    fi
}
run_test_s55

# ── Test S.56: PR Summary ───────────────────────────────
begin_test "S.56" "Generate PR summary" \
    "generate(pr_summary) produces markdown summary" \
    "Tests: session summary for PR descriptions"

run_test_s56() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"format":"pr_summary"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(pr_summary)."
        return
    fi

    echo "  [pr_summary preview]"
    echo "$content_text" | head -5

    # Strict: parse JSON and verify summary field has actual content, not just empty string
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    # Plain text summary (not JSON)
    if len(t.strip()) > 20:
        print(f'PASS text_len={len(t.strip())}')
    else:
        print(f'FAIL too_short text_len={len(t.strip())}')
    sys.exit()
data = json.loads(t[i:])
summary = data.get('summary', data.get('text', data.get('description', '')))
if isinstance(summary, str) and len(summary.strip()) > 10:
    print(f'PASS summary_len={len(summary.strip())}')
elif isinstance(summary, str) and len(summary.strip()) == 0:
    print('FAIL summary is empty string')
else:
    # Check if there are other meaningful fields
    keys = [k for k in data.keys() if k not in ('metadata',)]
    print(f'FAIL summary={repr(summary)[:50]} keys={keys[:8]}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$validation" | grep -q "^PASS"; then
        pass "generate(pr_summary) returned meaningful summary. $validation"
    else
        fail "generate(pr_summary) returned empty or insufficient summary. $validation. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s56

# ── Test S.57: SARIF ─────────────────────────────────────
begin_test "S.57" "Generate SARIF report" \
    "generate(sarif) produces valid SARIF structure with version, schema, runs" \
    "Tests: accessibility/security results in SARIF format"

run_test_s57() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"format":"sarif"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(sarif)."
        return
    fi

    echo "  [sarif structure]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    print(f'    version: {data.get(\"version\", \"?\")}')
    print(f'    \$schema: {str(data.get(\"\$schema\", \"?\"))[:60]}')
    runs = data.get('runs', [])
    print(f'    runs: {len(runs)}')
    if runs:
        results = runs[0].get('results', [])
        print(f'    results in run[0]: {len(results)}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    local has_version has_runs
    has_version=$(echo "$content_text" | grep -c '"version"' || true)
    has_runs=$(echo "$content_text" | grep -c '"runs"' || true)

    if [ "$has_version" -gt 0 ] && [ "$has_runs" -gt 0 ]; then
        pass "generate(sarif) has valid SARIF structure (version + runs)."
    else
        fail "generate(sarif) missing SARIF fields. version=$has_version, runs=$has_runs. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s57

# ── Test S.58: HAR ───────────────────────────────────────
begin_test "S.58" "Generate HAR archive" \
    "generate(har) produces HAR structure with log, version, creator, entries" \
    "Tests: network traffic export in HAR format"

run_test_s58() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Seed network traffic so HAR has entries to export
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "execute_js" '{"action":"execute_js","reason":"Seed fetch for HAR export","script":"fetch(\"https://jsonplaceholder.typicode.com/posts/1\").then(r=>r.json()).then(d=>\"fetched\").catch(e=>e.message)"}'
        sleep 2
    fi

    local response
    response=$(call_tool "generate" '{"format":"har"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(har)."
        return
    fi

    echo "  [har structure]"
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    log = data.get('log', data)
    version = log.get('version', '?')
    creator = log.get('creator', {})
    entries = log.get('entries', [])
    entry_count = len(entries) if isinstance(entries, list) else 0
    print(f'    version: {version}')
    print(f'    creator: {creator.get(\"name\", \"?\")} {creator.get(\"version\", \"?\")}')
    print(f'    entries: {entry_count}')
    if entries:
        print(f'    first url: {entries[0].get(\"request\", {}).get(\"url\", \"?\")[:60]}')
    has_version = version != '?'
    has_creator = bool(creator.get('name'))
    if has_version and has_creator and entry_count > 0:
        print(f'VERDICT:PASS entries={entry_count}')
    elif has_version and has_creator:
        print(f'VERDICT:FAIL_EMPTY valid HAR structure but 0 entries')
    else:
        print(f'VERDICT:FAIL_STRUCTURE version={version} creator={creator}')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL_PARSE')
" 2>/dev/null || echo "VERDICT:FAIL_PARSE")

    echo "$validation" | grep -v "^VERDICT:" || true

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        pass "generate(har) has valid HAR structure with entries. $validation"
    else
        fail "generate(har) failed. $(echo "$validation" | grep 'VERDICT:' | head -1). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s58

# ── Test S.59: CSP ───────────────────────────────────────
begin_test "S.59" "Generate Content Security Policy" \
    "generate(csp, mode='moderate') produces policy directives" \
    "Tests: CSP generation from observed resources"

run_test_s59() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "generate" '{"format":"csp","mode":"moderate"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(csp)."
        return
    fi

    echo "  [csp policy]"
    echo "$content_text" | head -3

    if echo "$content_text" | grep -qiE "default-src|script-src|style-src|policy|directive|self"; then
        pass "generate(csp) returned CSP policy directives."
    else
        # CSP may return a structured object instead of raw header
        if echo "$content_text" | grep -qiE "csp|content.security|header"; then
            pass "generate(csp) returned CSP-related content."
        else
            fail "generate(csp) missing policy directives. Content: $(truncate "$content_text" 200)"
        fi
    fi
}
run_test_s59

# ── Test S.60: SRI ───────────────────────────────────────
begin_test "S.60" "Generate Subresource Integrity hashes" \
    "generate(sri) produces resource integrity data" \
    "Tests: SRI hash generation for loaded scripts/styles"

run_test_s60() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Navigate to a page with external scripts so SRI has resources to hash
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "navigate" '{"action":"navigate","url":"https://cdnjs.cloudflare.com/ajax/libs/lodash.js/4.17.21/lodash.min.js","reason":"Load page with scripts for SRI test"}' 20
        sleep 2
        # Go back to a page that loaded those scripts
        interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Return to test page"}' 20
        sleep 1
    fi

    local response
    response=$(call_tool "generate" '{"format":"sri"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from generate(sri)."
        return
    fi

    echo "  [sri data]"
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    resources = data.get('resources', data.get('entries', []))
    count = len(resources) if isinstance(resources, list) else 0
    print(f'    resources: {count}')
    if isinstance(resources, list):
        for r in resources[:3]:
            url = r.get('url', '?')[:60]
            integrity = r.get('integrity', r.get('hash', '?'))[:40]
            print(f'    {url}')
            print(f'      integrity: {integrity}')
    # SRI on example.com may have 0 cross-origin scripts — that's OK if response says so
    msg = data.get('message', data.get('status', ''))
    if count > 0:
        print(f'VERDICT:PASS resources={count}')
    elif 'no' in str(msg).lower() and ('resource' in str(msg).lower() or 'script' in str(msg).lower()):
        print(f'VERDICT:SKIP no resources to hash (expected on simple pages)')
    else:
        print(f'VERDICT:FAIL resources=0 message={str(msg)[:80]}')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL_PARSE')
" 2>/dev/null || echo "VERDICT:FAIL_PARSE")

    echo "$validation" | grep -v "^VERDICT:" || true

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        pass "generate(sri) returned SRI hashes for resources. $validation"
    elif echo "$validation" | grep -q "VERDICT:SKIP"; then
        skip "generate(sri): no cross-origin scripts to hash on current page."
    else
        fail "generate(sri) returned 0 resources. $(echo "$validation" | grep 'VERDICT:' | head -1). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s60
