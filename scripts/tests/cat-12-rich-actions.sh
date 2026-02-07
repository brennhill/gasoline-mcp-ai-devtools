#!/bin/bash
# cat-12-rich-actions.sh — UAT tests for Rich Action Results schema contract.
# Validates that the interact tool schema advertises the analyze param
# and that the tool description guides AI toward perf_diff / timing features.
#
# These tests will FAIL until the schema is updated in tools_schema.go.
# They test ONLY the schema contract — behavioral tests are in Go unit tests:
#   - internal/performance/diff_test.go (ComputePerfDiff, ResourceDiff, Summary)
#   - cmd/dev-console/tools_interact_rich_test.go (analyze param passthrough)
#
# Run: bash scripts/tests/cat-12-rich-actions.sh [port] [results-file]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "12" "Rich Action Results" "3"
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

finish_category
