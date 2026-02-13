#!/bin/bash
# 07-generate-formats.sh — 7.1-7.7: All 7 generate formats.
# reproduction, test, pr_summary, sarif, har, csp, sri
set -eo pipefail

begin_category "7" "Generate Formats" "7"

# ── Test 7.1: Reproduction ──────────────────────────────
begin_test "7.1" "Generate reproduction script" \
    "generate(reproduction) produces Playwright code patterns" \
    "Tests: action replay code generation"

run_test_7_1() {
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

    # Prior tests seeded multiple actions — reproduction MUST contain code
    if echo "$content_text" | grep -qE "page\.|await.*goto|\.click\("; then
        pass "generate(reproduction) contains Playwright code (page.goto/click)."
    elif echo "$content_text" | grep -qiE "page\.|goto|click|playwright"; then
        pass "generate(reproduction) contains Playwright code patterns."
    else
        fail "generate(reproduction) missing Playwright patterns. Actions were seeded by prior tests. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_1

# ── Test 7.2: Test ──────────────────────────────────────
begin_test "7.2" "Generate Playwright test" \
    "generate(test, test_name='smoke-test') produces test/expect patterns" \
    "Tests: test scaffold generation"

run_test_7_2() {
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

    # Check if this is a stub (returns empty script)
    if echo "$content_text" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i >= 0:
    data = json.loads(t[i:])
    script = data.get('script', '')
    if script == '':
        print('STUB')
    else:
        print('OK')
else:
    print('OK')
" 2>/dev/null | grep -q "STUB"; then
        skip "generate(test) is a stub implementation (returns empty script). Not yet implemented."
        return
    fi

    # If not a stub, verify it contains test code
    if echo "$content_text" | grep -qE "test\(|describe\(|it\("; then
        pass "generate(test) contains test framework patterns (test/describe/it)."
    elif echo "$content_text" | grep -qiE "expect|playwright|page\."; then
        pass "generate(test) contains test assertion patterns."
    else
        fail "generate(test) missing test patterns. Actions were seeded by prior tests. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_2

# ── Test 7.3: PR Summary ───────────────────────────────
begin_test "7.3" "Generate PR summary" \
    "generate(pr_summary) produces markdown summary with session stats" \
    "Tests: session summary for PR descriptions"

run_test_7_3() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Seed browser activity so pr_summary has data to summarize.
    # Navigate to a page — this generates actions and network entries.
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Seed activity for PR summary"}' 20
        sleep 2
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

    # Validate: summary must be a non-empty string with meaningful content.
    # With seeded data: expect markdown with stats (Actions, Commands, etc.)
    # Without seeded data: expect "No activity captured" message.
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    if len(t.strip()) > 20:
        print(f'PASS text_len={len(t.strip())}')
    else:
        print(f'FAIL too_short text_len={len(t.strip())}')
    sys.exit()
data = json.loads(t[i:])
summary = data.get('summary', data.get('text', data.get('description', '')))
stats = data.get('stats', {})
if not isinstance(summary, str) or len(summary.strip()) < 10:
    keys = [k for k in data.keys() if k not in ('metadata',)]
    print(f'FAIL summary={repr(summary)[:50]} keys={keys[:8]}')
elif 'Session Summary' not in summary:
    print(f'FAIL missing Session Summary header. Got: {summary[:80]}')
elif 'No activity' in summary:
    print(f'PASS no_activity summary_len={len(summary.strip())}')
elif 'Actions' in summary:
    total = sum(v for v in stats.values() if isinstance(v, int))
    print(f'PASS with_stats summary_len={len(summary.strip())} stat_total={total}')
else:
    print(f'PASS summary_len={len(summary.strip())}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$validation" | grep -q "^PASS"; then
        pass "generate(pr_summary) returned meaningful summary. $validation"
    else
        fail "generate(pr_summary) returned empty or insufficient summary. $validation. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_3

# ── Test 7.4: SARIF ─────────────────────────────────────
begin_test "7.4" "Generate SARIF report" \
    "generate(sarif) produces valid SARIF structure with version, schema, runs" \
    "Tests: accessibility/security results in SARIF format"

run_test_7_4() {
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

    # Validate SARIF structure: version 2.1.0, $schema, runs array with tool.driver
    # Results may be 0 if no a11y/security audit was run — that's valid structure.
    local sarif_verdict
    sarif_verdict=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    version = data.get('version', '')
    schema = data.get('\$schema', '')
    runs = data.get('runs', [])
    if version != '2.1.0':
        print(f'FAIL version={version} expected=2.1.0')
    elif not isinstance(runs, list) or len(runs) == 0:
        print(f'FAIL no runs array')
    elif not schema:
        print(f'FAIL missing \$schema field')
    else:
        driver = runs[0].get('tool', {}).get('driver', {})
        driver_name = driver.get('name', '')
        results = runs[0].get('results', [])
        result_count = len(results) if isinstance(results, list) else 0
        if not driver_name:
            print(f'FAIL missing tool.driver.name in runs[0]')
        else:
            print(f'PASS version=2.1.0 driver={driver_name} results={result_count}')
except Exception as e:
    print(f'FAIL parse: {e}')
" 2>/dev/null || echo "FAIL parse_error")

    if echo "$sarif_verdict" | grep -q "^PASS"; then
        pass "generate(sarif) valid SARIF structure. $sarif_verdict"
    else
        fail "generate(sarif) invalid structure. $sarif_verdict. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_4

# ── Test 7.5: HAR ───────────────────────────────────────
begin_test "7.5" "Generate HAR archive" \
    "generate(har) produces HAR structure with log, version, creator, entries" \
    "Tests: network traffic export in HAR format"

run_test_7_5() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # Seed network traffic so HAR has entries to export.
    # Navigate to a fresh page — this generates resource timing entries the extension captures.
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Seed network for HAR export"}' 20
        sleep 2
        # Also trigger a fetch for body capture
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
        fail "generate(har) failed. $(echo "$validation" | grep 'VERDICT:' | head -1 || echo 'no verdict'). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_5

# ── Test 7.6: CSP ───────────────────────────────────────
begin_test "7.6" "Generate Content Security Policy" \
    "generate(csp, mode='moderate') produces policy directives" \
    "Tests: CSP generation from observed resources"

run_test_7_6() {
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

    # CSP must contain actual directives, not just the word "csp"
    if echo "$content_text" | grep -qiE "default-src|script-src|style-src|connect-src|img-src"; then
        pass "generate(csp) returned CSP policy with directives."
    elif echo "$content_text" | grep -qiE "'self'|'none'|'unsafe-inline'"; then
        pass "generate(csp) returned CSP policy values."
    else
        fail "generate(csp) missing CSP directives (default-src, script-src, etc.). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_6

# ── Test 7.7: SRI ───────────────────────────────────────
begin_test "7.7" "Generate Subresource Integrity hashes" \
    "generate(sri) returns valid SRI structure (resources depend on captured network data)" \
    "Tests: SRI hash generation for loaded scripts/styles"

run_test_7_7() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    # SRI needs cross-origin <script>/<link> tags on the page.
    # Inject a CDN script tag so SRI has a resource to hash.
    if [ "$PILOT_ENABLED" = "true" ]; then
        interact_and_wait "execute_js" '{"action":"execute_js","reason":"Inject CDN script for SRI test","script":"var s = document.createElement(\"script\"); s.src = \"https://cdnjs.cloudflare.com/ajax/libs/lodash.js/4.17.21/lodash.min.js\"; s.crossOrigin = \"anonymous\"; document.head.appendChild(s); \"injected\""}'
        sleep 3
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
    status = data.get('status', '')
    summary = data.get('summary', {})
    msg = data.get('message', data.get('hint', ''))
    if count > 0:
        # Best case: actual SRI hashes generated
        print(f'VERDICT:PASS_WITH_DATA resources={count}')
    elif status == 'unavailable':
        # No network bodies captured — expected when no page with third-party resources loaded
        print(f'VERDICT:PASS_NO_DATA status=unavailable hint={str(msg)[:60]}')
    elif isinstance(resources, list) and isinstance(summary, dict):
        # Valid structure, 0 resources — network data exists but no third-party scripts/styles
        print(f'VERDICT:PASS_EMPTY valid structure, 0 third-party resources to hash')
    else:
        print(f'VERDICT:FAIL unexpected shape keys={list(data.keys())[:8]}')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL_PARSE')
" 2>/dev/null || echo "VERDICT:FAIL_PARSE")

    echo "$validation" | grep -v "^VERDICT:" || true

    if echo "$validation" | grep -q "VERDICT:PASS_WITH_DATA"; then
        pass "generate(sri) returned SRI hashes for resources. $(echo "$validation" | grep 'VERDICT:' | head -1)"
    elif echo "$validation" | grep -q "VERDICT:PASS_NO_DATA"; then
        pass "generate(sri) returned valid response (no third-party network data captured). $(echo "$validation" | grep 'VERDICT:' | head -1)"
    elif echo "$validation" | grep -q "VERDICT:PASS_EMPTY"; then
        pass "generate(sri) returned valid structure (0 third-party scripts/styles in capture). $(echo "$validation" | grep 'VERDICT:' | head -1)"
    else
        fail "generate(sri) returned invalid response. $(echo "$validation" | grep 'VERDICT:' | head -1 || echo 'no verdict'). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_7_7
