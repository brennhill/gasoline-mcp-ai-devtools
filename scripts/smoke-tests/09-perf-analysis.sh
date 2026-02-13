#!/bin/bash
# 09-perf-analysis.sh — 9.1-9.5: Performance analysis tests.
# perf_diff on refresh, click timing, analyze:true, user timing, LLM fields
set -eo pipefail

begin_category "9" "Performance Analysis" "5"

# ── Test 9.1: Refresh returns perf_diff ─────────────────
begin_test "9.1" "Refresh returns perf_diff after baseline" \
    "Navigate to a page (baseline), refresh (comparison), verify perf_diff in command result" \
    "Tests: extension perf tracking > auto-diff > enriched action result"

run_test_9_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Load baseline page"}' 20
    sleep 3

    interact_and_wait "refresh" '{"action":"refresh","reason":"Establish perf baseline"}' 20
    sleep 3

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
run_test_9_1

# ── Test 9.2: Click returns compact feedback ────────────
begin_test "9.2" "Click returns timing_ms and dom_summary" \
    "Click a button, verify the command result includes timing_ms and dom_summary" \
    "Tests: always-on compact DOM feedback (~30 tokens per action)"

run_test_9_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "navigate" '{"action":"navigate","url":"https://example.com","reason":"Clean page for click test"}' 20
    sleep 2

    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Inject test button","script":"var btn = document.createElement(\"button\"); btn.id = \"perf-test-btn\"; btn.textContent = \"Test\"; btn.onclick = function() { var d = document.createElement(\"div\"); d.textContent = \"clicked\"; document.body.appendChild(d); }; document.body.appendChild(btn); \"injected\""}'
    sleep 0.5

    interact_and_wait "click" '{"action":"click","selector":"#perf-test-btn","reason":"Click test button"}'

    echo "  [click result]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    if 'result' in data and isinstance(data['result'], dict):
        data = data['result']
    print(f'    timing_ms: {data.get(\"timing_ms\", \"MISSING\")}')
    print(f'    dom_summary: {data.get(\"dom_summary\", \"MISSING\")}')
    print(f'    success: {data.get(\"success\", \"?\")}')
except Exception as e:
    print(f'    (parse: {e})')
" 2>/dev/null || true

    # Strict validation: parse JSON and verify both fields have actual values
    local validation
    validation=$(echo "$INTERACT_RESULT" | python3 -c "
import sys, json
t = sys.stdin.read(); i = t.find('{')
if i < 0:
    print('NO_JSON')
    sys.exit()
data = json.loads(t[i:])
timing = data.get('timing_ms')
dom = data.get('dom_summary')
result = data.get('result', {})
if isinstance(result, dict):
    timing = timing or result.get('timing_ms')
    dom = dom or result.get('dom_summary')
has_timing = timing is not None and isinstance(timing, (int, float)) and timing > 0
has_dom = dom is not None and isinstance(dom, str) and len(dom) > 0
if has_timing and has_dom:
    print(f'PASS timing_ms={timing} dom_summary={dom}')
elif has_timing:
    print(f'FAIL_DOM timing_ms={timing} dom_summary=MISSING')
elif has_dom:
    print(f'FAIL_TIMING dom_summary={dom}')
else:
    print(f'FAIL_BOTH keys={list(data.keys())[:10]}')
" 2>/dev/null || echo "PARSE_ERROR")

    if echo "$validation" | grep -q "^PASS"; then
        pass "Click result includes timing_ms and dom_summary. $validation"
    else
        fail "Click result missing required fields. $validation. Result: $(truncate "$INTERACT_RESULT" 300)"
    fi
}
run_test_9_2

# ── Test 9.3: analyze:true returns full breakdown ───────
begin_test "9.3" "Click with analyze:true returns full breakdown" \
    "Click with analyze:true, verify timing breakdown, dom_changes, and analysis string" \
    "Tests: opt-in detailed profiling for interaction debugging"

run_test_9_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Inject profiling button","script":"var btn2 = document.createElement(\"button\"); btn2.id = \"analyze-btn\"; btn2.textContent = \"Analyze Me\"; btn2.onclick = function() { for (var i=0; i<5; i++) { var d = document.createElement(\"p\"); d.textContent = \"item-\" + i; document.body.appendChild(d); } }; document.body.appendChild(btn2); \"injected\""}'
    sleep 0.5

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
run_test_9_3

# ── Test 9.4: User Timing in observe(performance) ──────
begin_test "9.4" "User Timing entries in observe(performance)" \
    "Insert performance.mark/measure via execute_js, verify they appear in observe(performance)" \
    "Tests: extension captures standard User Timing API entries"

run_test_9_4() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    local marker
    marker="gasoline_uat_$(date +%s)"

    interact_and_wait "execute_js" "{\"action\":\"execute_js\",\"reason\":\"Insert User Timing marks\",\"script\":\"performance.mark('${marker}_start'); for(var i=0;i<1000000;i++){} performance.mark('${marker}_end'); performance.measure('${marker}_duration','${marker}_start','${marker}_end'); 'marked'\"}"
    sleep 3

    # Performance data is under analyze(performance), NOT observe(performance)
    local response
    response=$(call_tool "analyze" '{"what":"performance"}')
    local content_text
    content_text=$(extract_content_text "$response")

    echo "  [user timing check]"
    echo "$content_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    # analyze(performance) returns {snapshots: [...], count: N}
    snapshots = data.get('snapshots', [])
    print(f'    snapshots: {len(snapshots)}')
    for snap in snapshots[:2]:
        ut = snap.get('user_timing', {})
        if ut:
            marks = ut.get('marks', [])
            measures = ut.get('measures', [])
            print(f'    marks: {len(marks)}')
            for m in marks[:4]:
                print(f'      {m.get(\"name\",\"?\")} @ {m.get(\"time\",m.get(\"start_time\",\"?\"))}')
            print(f'    measures: {len(measures)}')
            for m in measures[:2]:
                print(f'      {m.get(\"name\",\"?\")} duration={m.get(\"duration\",\"?\")}ms')
        else:
            print(f'    url: {snap.get(\"url\",\"?\")[:60]} (no user_timing)')
except Exception as e:
    print(f'    (parse error: {e})')
" 2>/dev/null || true

    if echo "$content_text" | grep -q "$marker"; then
        pass "User Timing markers ($marker) found in analyze(performance)."
    else
        local snap_count
        snap_count=$(echo "$content_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{')
data=json.loads(t[i:]) if i>=0 else {}
print(len(data.get('snapshots',[])))
" 2>/dev/null || echo "?")
        fail "User Timing marker '$marker' not found. snapshots=$snap_count. Performance snapshot may not have been sent by extension yet."
    fi
}
run_test_9_4

# ── Test 9.5: LLM-optimized perf_diff fields ───────────
begin_test "9.5" "perf_diff has verdict, unit, rating, clean summary" \
    "Refresh (baseline warm from 9.1), verify perf_diff has LLM-optimized fields" \
    "Tests: verdict, unit (ms/KB/count), rating (Web Vitals), clean summary"

run_test_9_5() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    interact_and_wait "refresh" '{"action":"refresh","reason":"Check LLM perf fields"}' 20

    if ! echo "$INTERACT_RESULT" | grep -q '"perf_diff"'; then
        fail "No perf_diff in refresh result. Result: $(truncate "$INTERACT_RESULT" 300)"
        return
    fi

    echo "  [LLM optimization fields]"
    echo "$INTERACT_RESULT" | python3 -c "
import sys, json
try:
    text = sys.stdin.read()
    idx = text.find('{')
    if idx < 0: raise ValueError('no JSON')
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

    if echo "$INTERACT_RESULT" | grep -qE '"verdict":\s*"(improved|regressed|mixed|unchanged)"'; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: verdict field"
    fi

    if echo "$INTERACT_RESULT" | grep -q '"unit":"ms"'; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: unit field (expected 'ms' on timing metrics)"
    fi

    if echo "$INTERACT_RESULT" | grep -qE '"rating":"(good|needs_improvement|poor)"'; then
        checks_passed=$((checks_passed + 1))
    else
        echo "  MISSING: rating field (expected on LCP/FCP/TTFB/CLS)"
    fi

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
run_test_9_5
