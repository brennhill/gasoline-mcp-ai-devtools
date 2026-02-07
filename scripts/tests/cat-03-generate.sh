#!/bin/bash
# cat-03-generate.sh — Category 3: Generate Tool (9 tests).
# Tests all generate formats plus negative cases.
# Each format must return a valid response shape, even with no captured data.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"
begin_category "3" "Generate Tool" "9"

ensure_daemon

# ── 3.1 — generate(reproduction) ─────────────────────────
begin_test "3.1" "generate(reproduction) returns script" \
    "Call generate with format:reproduction. Verify not error, content has script data." \
    "Reproduction scripts are the primary debugging output."
run_test_3_1() {
    RESPONSE=$(call_tool "generate" '{"format":"reproduction"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! echo "$text" | grep -qi "reproduction" && ! echo "$text" | grep -qi "playwright"; then
        fail "generate(reproduction) content must mention 'reproduction' or 'playwright'. Content: $(truncate "$text" 300)"
        return
    fi
    pass "Sent generate(reproduction), got valid response mentioning reproduction/playwright. Content: $(truncate "$text" 200)"
}
run_test_3_1

# ── 3.2 — generate(test) ─────────────────────────────────
begin_test "3.2" "generate(test) returns Playwright test" \
    "Call generate with format:test. Verify not error, content has test code." \
    "Test generation is a core feature."
run_test_3_2() {
    RESPONSE=$(call_tool "generate" '{"format":"test"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! echo "$text" | grep -qi "test" && ! echo "$text" | grep -qi "playwright"; then
        fail "generate(test) content must mention 'test' or 'playwright'. Content: $(truncate "$text" 300)"
        return
    fi
    pass "Sent generate(test), got valid response mentioning test/playwright. Content: $(truncate "$text" 200)"
}
run_test_3_2

# ── 3.3 — generate(pr_summary) ───────────────────────────
begin_test "3.3" "generate(pr_summary) returns summary" \
    "Call generate with format:pr_summary. Verify not error, content has summary text." \
    "PR summaries are used in CI workflows."
run_test_3_3() {
    RESPONSE=$(call_tool "generate" '{"format":"pr_summary"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "summary" && ! check_contains "$text" "Summary"; then
        fail "generate(pr_summary) content must mention 'summary'. Content: $(truncate "$text" 300)"
        return
    fi
    pass "Sent generate(pr_summary), got valid response mentioning summary. Content: $(truncate "$text" 200)"
}
run_test_3_3

# ── 3.4 — generate(sarif) ────────────────────────────────
begin_test "3.4" "generate(sarif) returns valid SARIF data" \
    "Call generate with format:sarif. Verify response mentions SARIF or has status field." \
    "SARIF is consumed by GitHub Code Scanning. Invalid format means silent CI failure."
run_test_3_4() {
    RESPONSE=$(call_tool "generate" '{"format":"sarif"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "SARIF" && ! check_contains "$text" "sarif"; then
        fail "generate(sarif) content must mention 'SARIF'. Content: $(truncate "$text")"
        return
    fi
    pass "Sent generate(sarif), got valid response mentioning SARIF. Content: ${#text} chars."
}
run_test_3_4

# ── 3.5 — generate(har) ──────────────────────────────────
begin_test "3.5" "generate(har) returns HAR structure" \
    "Call generate with format:har. Verify not error, content has HAR data." \
    "HAR is consumed by Chrome DevTools, Charles Proxy, etc. Invalid format means import fails."
run_test_3_5() {
    RESPONSE=$(call_tool "generate" '{"format":"har"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "HAR" && ! check_contains "$text" "har" && ! check_contains "$text" "entries"; then
        fail "generate(har) content must mention 'HAR' or 'entries'. Content: $(truncate "$text" 300)"
        return
    fi
    pass "Sent generate(har), got valid response mentioning HAR/entries. Content: $(truncate "$text" 200)"
}
run_test_3_5

# ── 3.6 — generate(csp) ──────────────────────────────────
begin_test "3.6" "generate(csp) returns policy data" \
    "Call generate with format:csp. Verify response has status and mode fields." \
    "CSP generation is security-critical. Wrong policy means XSS or broken site."
run_test_3_6() {
    RESPONSE=$(call_tool "generate" '{"format":"csp"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "status"; then
        fail "Expected content to contain 'status' field. Got: $(truncate "$text")"
        return
    fi
    if ! check_contains "$text" "mode"; then
        fail "Expected content to contain 'mode' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent generate(csp), got valid response with 'status' and 'mode'. Content: ${#text} chars."
}
run_test_3_6

# ── 3.7 — generate(sri) ──────────────────────────────────
begin_test "3.7" "generate(sri) returns hashes" \
    "Call generate with format:sri. Verify response has resources array." \
    "SRI hashes prevent supply-chain attacks."
run_test_3_7() {
    RESPONSE=$(call_tool "generate" '{"format":"sri"}')
    if ! check_not_error "$RESPONSE"; then
        fail "Expected success but got isError. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "Response had no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "resources"; then
        fail "Expected content to contain 'resources' field. Got: $(truncate "$text")"
        return
    fi
    pass "Sent generate(sri), got valid response with 'resources'. Content: ${#text} chars."
}
run_test_3_7

# ── 3.8 — generate with invalid format ───────────────────
begin_test "3.8" "generate with invalid format returns error" \
    "Call generate with format:docx. Verify isError:true with helpful message listing valid formats." \
    "Invalid format must not silently return empty success."
run_test_3_8() {
    RESPONSE=$(call_tool "generate" '{"format":"docx"}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true but got success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "nknown" && ! check_contains "$text" "format"; then
        fail "Error message does not mention unknown format. Got: $(truncate "$text")"
        return
    fi
    pass "Sent generate(docx), got isError:true. Error mentions invalid format. Content: $(truncate "$text" 150)"
}
run_test_3_8

# ── 3.9 — generate with missing format ───────────────────
begin_test "3.9" "generate with missing format returns error" \
    "Call generate with empty params {}. Verify error about missing required parameter." \
    "Missing required params must fail loudly."
run_test_3_9() {
    RESPONSE=$(call_tool "generate" '{}')
    if ! check_is_error "$RESPONSE"; then
        fail "Expected isError:true but got success. Content: $(truncate "$(extract_content_text "$RESPONSE")")"
        return
    fi
    local text
    text=$(extract_content_text "$RESPONSE")
    if [ -z "$text" ]; then
        fail "isError was true but no content text. Full response: $(truncate "$RESPONSE")"
        return
    fi
    if ! check_contains "$text" "format"; then
        fail "Error message does not mention missing 'format' parameter. Got: $(truncate "$text")"
        return
    fi
    pass "Sent generate({}), got isError:true. Error mentions missing 'format' parameter. Content: $(truncate "$text" 150)"
}
run_test_3_9

finish_category
