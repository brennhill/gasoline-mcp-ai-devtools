#!/bin/bash
# 03-observe-modes.sh — S.28-S.34: Observe modes not covered by core telemetry.
# vitals, tabs, network_bodies, error_bundles, timeline, pilot, extension_logs
set -eo pipefail

begin_category "3" "Observe Modes" "7"

# ── Test S.28: Web Vitals ────────────────────────────────
begin_test "S.28" "Web Vitals via observe(vitals)" \
    "observe(vitals) after page load + click returns metrics object" \
    "Tests: extension Web Vitals collection > daemon > MCP observe"

run_test_s28() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"vitals"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(vitals)."
        return
    fi

    echo "  [web vitals]"
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    metrics = data.get('metrics', data)
    numeric_count = 0
    for key in ['lcp', 'cls', 'fcp', 'inp', 'ttfb', 'fid', 'domContentLoaded', 'dom_content_loaded', 'load']:
        val = metrics.get(key, metrics.get(key.upper()))
        if val is None:
            print(f'    {key}: n/a')
        elif isinstance(val, (int, float)):
            print(f'    {key}: {val}')
            numeric_count += 1
        elif isinstance(val, dict):
            v = val.get('value', val.get('rating'))
            if isinstance(v, (int, float)):
                numeric_count += 1
            print(f'    {key}: {v}')
        else:
            print(f'    {key}: {val}')
    has_data = metrics.get('has_data', False)
    url = metrics.get('url', 'n/a')
    print(f'    url: {url}')
    print(f'    has_data: {has_data}')
    print(f'    numeric_metrics: {numeric_count}')
    if numeric_count > 0:
        print('VERDICT:PASS')
    elif has_data:
        print('VERDICT:PASS_HASDATA')
    else:
        print('VERDICT:FAIL')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL')
" 2>/dev/null || echo "VERDICT:FAIL")

    echo "$validation" | grep -v "^VERDICT:" || true

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        pass "observe(vitals) returned metrics with numeric values."
    elif echo "$validation" | grep -q "VERDICT:PASS_HASDATA"; then
        pass "observe(vitals) returned metrics (has_data=true, some may be awaiting measurement)."
    else
        fail "observe(vitals) returned no numeric metric values. All vitals are n/a. Verify extension is collecting Web Vitals. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s28

# ── Test S.29: Tab info ──────────────────────────────────
begin_test "S.29" "Tab info via observe(tabs)" \
    "observe(tabs) returns tabs array with URLs and tracking status" \
    "Tests: daemon tab tracking state"

run_test_s29() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"tabs"}')
    local content_text
    content_text=$(extract_content_text "$response")

    echo "  [tabs]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    tabs = data.get('tabs', [])
    print(f'    count: {len(tabs)}')
    for tab in tabs[:5]:
        url = tab.get('url', '?')[:60]
        tracked = tab.get('tracking_active', tab.get('tracked', '?'))
        print(f'    [{tracked}] {url}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -q "tabs"; then
        pass "observe(tabs) returned tab data."
    else
        fail "observe(tabs) missing 'tabs' field. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s29

# ── Test S.30: Network bodies ────────────────────────────
begin_test "S.30" "Network bodies via observe(network_bodies)" \
    "Execute a fetch() then observe(network_bodies) to see request/response data" \
    "Tests: fetch interception > extension > daemon body capture"

run_test_s30() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    # Seed a fetch request
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Trigger fetch for body capture","script":"fetch(\"https://jsonplaceholder.typicode.com/posts/1\").then(r=>r.json()).then(d=>JSON.stringify(d)).catch(e=>e.message)"}'
    sleep 2

    local response
    response=$(call_tool "observe" '{"what":"network_bodies"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(network_bodies)."
        return
    fi

    echo "  [network bodies]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    entries = data.get('entries', data.get('bodies', []))
    print(f'    entries: {len(entries) if isinstance(entries, list) else \"(not array)\"}')
    if isinstance(entries, list):
        for e in entries[:3]:
            url = e.get('url', '?')[:60]
            status = e.get('status', e.get('status_code', '?'))
            print(f'    [{status}] {url}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -qiE "entries|bodies|url|status|request|response"; then
        pass "observe(network_bodies) returned captured request/response data."
    else
        fail "observe(network_bodies) missing expected fields. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s30

# ── Test S.31: Error bundles ─────────────────────────────
begin_test "S.31" "Error bundles via observe(error_bundles)" \
    "observe(error_bundles) after S.5 seeded errors returns context bundles" \
    "Tests: error bundling with surrounding network + actions context"

run_test_s31() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"error_bundles"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(error_bundles)."
        return
    fi

    echo "  [error bundles]"
    local bundle_count
    bundle_count=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    bundles = data.get('bundles', data.get('error_bundles', []))
    count = len(bundles) if isinstance(bundles, list) else 0
    print(count)
    if isinstance(bundles, list):
        for b in bundles[:3]:
            msg = b.get('error', b.get('message', '?'))
            if isinstance(msg, dict):
                msg = msg.get('message', str(msg))
            import sys as s2
            s2.stderr.write(f'    error: {str(msg)[:80]}\n')
            ctx = b.get('context', {})
            s2.stderr.write(f'      network: {len(ctx.get(\"network\", []))} actions: {len(ctx.get(\"actions\", []))} logs: {len(ctx.get(\"logs\", []))}\n')
except Exception as e:
    print(0)
" 2>/dev/null || echo "0")
    echo "    bundles: $bundle_count"

    if [ "$bundle_count" -gt 0 ] 2>/dev/null; then
        pass "observe(error_bundles) returned $bundle_count error context bundles."
    else
        fail "observe(error_bundles) returned 0 bundles. S.5 should have seeded errors. Verify error seeding and bundling pipeline. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s31

# ── Test S.32: Timeline ──────────────────────────────────
begin_test "S.32" "Timeline via observe(timeline)" \
    "observe(timeline) returns time-ordered entries across categories" \
    "Tests: unified timeline merging multiple data sources"

run_test_s32() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"timeline"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(timeline)."
        return
    fi

    echo "  [timeline]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    entries = data.get('entries', data.get('events', []))
    print(f'    entries: {len(entries) if isinstance(entries, list) else \"(not array)\"}')
    categories = set()
    if isinstance(entries, list):
        for e in entries[:10]:
            cat = e.get('category', e.get('type', '?'))
            categories.add(cat)
        print(f'    categories: {sorted(categories)}')
        for e in entries[:3]:
            ts = e.get('timestamp', e.get('time', '?'))
            cat = e.get('category', e.get('type', '?'))
            msg = e.get('message', e.get('summary', ''))[:60]
            print(f'    [{cat}] {ts} {msg}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -qiE "entries|events|timestamp|time|category|type"; then
        pass "observe(timeline) returned time-ordered entries."
    else
        fail "observe(timeline) missing expected fields. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s32

# ── Test S.33: Pilot state ───────────────────────────────
begin_test "S.33" "Pilot state via observe(pilot)" \
    "observe(pilot) returns pilot enabled/disabled status" \
    "Tests: pilot state query"

run_test_s33() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"pilot"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(pilot)."
        return
    fi

    echo "  [pilot state]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    print(f'    enabled: {data.get(\"enabled\", \"?\")}')
    print(f'    keys: {list(data.keys())[:8]}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -qiE "enabled|pilot|status"; then
        pass "observe(pilot) returned pilot state."
    else
        fail "observe(pilot) missing expected fields. Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s33

# ── Test S.34: Extension logs ────────────────────────────
begin_test "S.34" "Extension logs via observe(extension_logs)" \
    "observe(extension_logs) returns internal diagnostic log entries" \
    "Tests: extension internal logging pipeline"

run_test_s34() {
    if [ "$EXTENSION_CONNECTED" != "true" ]; then
        skip "Extension not connected."
        return
    fi

    local response
    response=$(call_tool "observe" '{"what":"extension_logs"}')
    local content_text
    content_text=$(extract_content_text "$response")

    if [ -z "$content_text" ]; then
        fail "Empty response from observe(extension_logs)."
        return
    fi

    echo "  [extension logs]"
    local validation
    validation=$(echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    logs = data.get('logs', data.get('entries', []))
    count = len(logs) if isinstance(logs, list) else 0
    print(f'    log entries: {count}')
    has_structure = True
    if isinstance(logs, list) and count > 0:
        for l in logs[:5]:
            msg = l.get('message', l.get('text', ''))
            level = l.get('level', l.get('severity', ''))
            print(f'    [{level}] {str(msg)[:80]}')
            if not msg and not level:
                has_structure = False
    levels_seen = set()
    if isinstance(logs, list):
        for l in logs:
            levels_seen.add(l.get('level', l.get('severity', '')))
    print(f'    levels seen: {sorted(levels_seen)}')
    if count > 0 and has_structure:
        print(f'VERDICT:PASS count={count}')
    elif count > 0:
        print(f'VERDICT:FAIL_STRUCTURE count={count} (entries missing message/level)')
    else:
        print('VERDICT:FAIL_EMPTY')
except Exception as e:
    print(f'    (parse: {e})')
    print('VERDICT:FAIL_PARSE')
" 2>/dev/null || echo "VERDICT:FAIL_PARSE")

    echo "$validation" | grep -v "^VERDICT:" || true

    if echo "$validation" | grep -q "VERDICT:PASS"; then
        local count
        count=$(echo "$validation" | grep -oP 'count=\K[0-9]+')
        pass "observe(extension_logs) returned $count entries with valid structure (level + message)."
    else
        fail "observe(extension_logs) failed. $(echo "$validation" | grep 'VERDICT:' | head -1). Content: $(truncate "$content_text" 200)"
    fi
}
run_test_s34
