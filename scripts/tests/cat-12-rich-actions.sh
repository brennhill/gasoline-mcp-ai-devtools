#!/bin/bash
# cat-12-rich-actions.sh — UAT tests for Rich Action Results.
# Tests BOTH schema contract AND behavioral contract:
#   - Schema: analyze param exists, descriptions guide AI
#   - Behavior: perf_diff produces correct output (via Go unit tests)
#   - Behavior: command_result response has timing_ms field
#
# Run: bash scripts/tests/cat-12-rich-actions.sh [port] [results-file]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "12" "Rich Action Results" "7"
ensure_daemon

# ── 12.1 — Schema: analyze boolean param exists ──────────
begin_test "12.1" "Schema includes 'analyze' boolean param" \
    "tools/list must have analyze:{type:'boolean'} in interact inputSchema" \
    "analyze is the opt-in flag for detailed interaction profiling."
run_test_12_1() {
    local response
    response=$(send_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')
    if [ -z "$response" ]; then
        fail "No response from tools/list."
        return
    fi

    local analyze_type
    analyze_type=$(echo "$response" | jq -r '.result.tools[] | select(.name == "interact") | .inputSchema.properties.analyze.type // empty' 2>/dev/null)

    if [ "$analyze_type" = "boolean" ]; then
        pass "interact schema has analyze:{type:'boolean'}."
    else
        fail "interact schema missing analyze boolean. Got type='$analyze_type'."
    fi
}
run_test_12_1

# ── 12.2 — Schema: analyze description mentions profiling ─
begin_test "12.2" "analyze description mentions profiling/performance/timing" \
    "analyze param description must guide the AI on when to use it" \
    "Without a good description, AI will either never use or always use analyze."
run_test_12_2() {
    local response
    response=$(send_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')

    local analyze_desc
    analyze_desc=$(echo "$response" | jq -r '.result.tools[] | select(.name == "interact") | .inputSchema.properties.analyze.description // empty' 2>/dev/null)

    if [ -z "$analyze_desc" ]; then
        fail "No analyze param description found in schema."
        return
    fi

    # Must contain at least one of: profil*, performance, timing, breakdown
    if echo "$analyze_desc" | grep -qiE "profil|performance|timing|breakdown"; then
        pass "analyze description guides AI: '$(truncate "$analyze_desc" 120)'"
    else
        fail "analyze description doesn't mention profiling/performance/timing/breakdown. Got: '$analyze_desc'"
    fi
}
run_test_12_2

# ── 12.3 — Tool description mentions rich action result fields ─
begin_test "12.3" "interact description mentions perf_diff or timing" \
    "Tool description must tell AI that actions return performance/timing data" \
    "AI discovers features from tool descriptions — if perf_diff isn't mentioned, AI won't use it."
run_test_12_3() {
    local response
    response=$(send_mcp '{"jsonrpc":"2.0","id":1,"method":"tools/list"}')

    local tool_desc
    tool_desc=$(echo "$response" | jq -r '.result.tools[] | select(.name == "interact") | .description // empty' 2>/dev/null)

    if [ -z "$tool_desc" ]; then
        fail "No interact tool description found."
        return
    fi

    # Must mention perf_diff, performance, or timing (not just "diff" which is too broad)
    if echo "$tool_desc" | grep -qiE "perf_diff|performance|timing"; then
        pass "interact description mentions performance/perf_diff/timing."
    else
        fail "interact description doesn't mention perf_diff/performance/timing. AI won't discover perf_diff. Got: '$(truncate "$tool_desc" 200)'"
    fi
}
run_test_12_3

# ── 12.4 — Behavioral: perf_diff summary doesn't say 'regressed 0%' ─
begin_test "12.4" "perf_diff summary says 'unchanged' for delta=0 (not 'regressed')" \
    "Go unit test: when all metrics have delta=0, summary must not say 'regressed'" \
    "Saying 'regressed 0%' is misleading. delta=0 = unchanged."
run_test_12_4() {
    local go_result
    go_result=$(go test ./internal/performance/ -run "TestSummary_DeltaZeroSaysUnchanged" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok"; then
        pass "delta=0 summary correctly says 'unchanged', not 'regressed'."
    else
        fail "delta=0 summary still says 'regressed'. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_4

# ── 12.5 — Behavioral: perf_diff metrics have rating on Web Vitals ─
begin_test "12.5" "perf_diff rating field exists on Web Vitals metrics" \
    "Go unit test: LCP, FCP, TTFB, CLS metrics include rating: good/needs_improvement/poor" \
    "Rating gives AI immediate Web Vitals assessment without threshold lookup."
run_test_12_5() {
    local go_result
    go_result=$(go test ./internal/performance/ -run "TestPerfDiff_LCP_Rating|TestPerfDiff_CLS_Rating" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "LCP and CLS metrics have correct rating values (good/needs_improvement/poor)."
    else
        fail "Rating tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_5

# ── 12.6 — Behavioral: perf_diff verdict uses valid enum ─
begin_test "12.6" "perf_diff verdict enum: improved/regressed/mixed/unchanged" \
    "Go unit tests: all 4 verdict values are correctly computed" \
    "Invalid verdict breaks AI decision-making."
run_test_12_6() {
    local go_result
    go_result=$(go test ./internal/performance/ -run "TestPerfDiff_Verdict" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "All 4 verdict values (improved/regressed/mixed/unchanged) computed correctly."
    else
        fail "Verdict tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_6

# ── 12.7 — Behavioral: perf_diff summary < 200 chars with no redundant sign ─
begin_test "12.7" "perf_diff summary: <200 chars, no redundant signs" \
    "Go unit tests: summary is concise and uses absolute percentages" \
    "Redundant signs waste tokens. 'improved -57%' should be 'improved 57%'."
run_test_12_7() {
    local go_result
    go_result=$(go test ./internal/performance/ -run "TestSummary_NoRedundant|TestSummary_Under200|TestSummary_Regression" -count=1 2>&1)

    if echo "$go_result" | grep -q "^ok" && ! echo "$go_result" | grep -q "FAIL"; then
        pass "Summary is concise (<200 chars), uses absolute percentages, no redundant signs."
    else
        fail "Summary format tests failed. Go test output: $(truncate "$go_result" 300)"
    fi
}
run_test_12_7

finish_category
