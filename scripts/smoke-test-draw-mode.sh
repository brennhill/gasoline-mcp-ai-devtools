#!/bin/bash
# smoke-test-draw-mode.sh — Human smoke test for Draw Mode features.
# Walks a human tester through all draw mode features with verification prompts.
#
# Requires:
#   - Gasoline MCP daemon running (gasoline-mcp or make dev)
#   - Chrome with Gasoline extension loaded and connected
#   - AI Web Pilot enabled in popup
#   - A tracked tab on any website
#
# Usage:
#   bash scripts/smoke-test-draw-mode.sh          # default port 7890
#   bash scripts/smoke-test-draw-mode.sh 7891     # explicit port

set -euo pipefail

PORT="${1:-7890}"
BASE="http://localhost:${PORT}"
MCP_ID=1

# ── Colors ──
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0

# ── Helpers ──
next_id() { MCP_ID=$((MCP_ID + 1)); }

send_mcp() {
    local payload="$1"
    next_id
    echo "$payload" | sed "s/__ID__/$MCP_ID/g" | \
        curl -s --max-time 15 --connect-timeout 3 \
            -X POST -H "Content-Type: application/json" \
            -d @- "$BASE/mcp" 2>/dev/null
}

call_tool() {
    local tool="$1" args="$2"
    send_mcp "{\"jsonrpc\":\"2.0\",\"id\":__ID__,\"method\":\"tools/call\",\"params\":{\"name\":\"$tool\",\"arguments\":$args}}"
}

extract_text() {
    echo "$1" | jq -r '.result.content[0].text // empty' 2>/dev/null
}

header() {
    echo ""
    echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════${RESET}"
    echo -e "${BOLD}${CYAN}  $1${RESET}"
    echo -e "${BOLD}${CYAN}═══════════════════════════════════════════════════${RESET}"
    echo ""
}

step() {
    echo -e "${BOLD}  ▶ $1${RESET}"
}

instruction() {
    echo -e "${YELLOW}    → $1${RESET}"
}

verify() {
    echo -e "${DIM}    ✓ Verify: $1${RESET}"
    echo -en "    ${BOLD}Did this pass? [Y/n/s(kip)] ${RESET}"
    read -r answer
    case "${answer,,}" in
        n|no)
            echo -e "    ${RED}✖ FAIL${RESET}"
            FAIL_COUNT=$((FAIL_COUNT + 1))
            ;;
        s|skip)
            echo -e "    ${YELLOW}⊘ SKIP${RESET}"
            SKIP_COUNT=$((SKIP_COUNT + 1))
            ;;
        *)
            echo -e "    ${GREEN}✔ PASS${RESET}"
            PASS_COUNT=$((PASS_COUNT + 1))
            ;;
    esac
    echo ""
}

auto_pass() {
    echo -e "    ${GREEN}✔ PASS (auto)${RESET}: $1"
    PASS_COUNT=$((PASS_COUNT + 1))
    echo ""
}

auto_fail() {
    echo -e "    ${RED}✖ FAIL (auto)${RESET}: $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo ""
}

# ── Pre-flight ──
header "Draw Mode Smoke Test"
echo -e "  Port: ${BOLD}$PORT${RESET}"
echo -e "  Ensure: Chrome + Gasoline extension + AI Web Pilot enabled"
echo -e "  Ensure: A tab is tracked and active"
echo ""
echo -en "  ${BOLD}Press Enter to begin...${RESET}"
read -r

# ══════════════════════════════════════════════
# SECTION 1: Schema Verification (automated)
# ══════════════════════════════════════════════
header "1. Schema Verification"

step "1.1 — draw_mode_start in interact schema"
TOOLS=$(send_mcp '{"jsonrpc":"2.0","id":__ID__,"method":"tools/list"}')
if echo "$TOOLS" | jq -e '.result.tools[] | select(.name=="interact") | .inputSchema.properties.action.enum[] | select(.=="draw_mode_start")' >/dev/null 2>&1; then
    auto_pass "draw_mode_start in interact action enum"
else
    auto_fail "draw_mode_start NOT in interact action enum"
fi

step "1.2 — annotations + annotation_detail in analyze schema"
HAS_ANN=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="analyze") | .inputSchema.properties.what.enum[] | select(.=="annotations")' 2>/dev/null)
HAS_DET=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="analyze") | .inputSchema.properties.what.enum[] | select(.=="annotation_detail")' 2>/dev/null)
if [ "$HAS_ANN" = "annotations" ] && [ "$HAS_DET" = "annotation_detail" ]; then
    auto_pass "annotations and annotation_detail in analyze what enum"
else
    auto_fail "Missing from analyze enum: ann=$HAS_ANN det=$HAS_DET"
fi

step "1.3 — visual_test, annotation_report, annotation_issues in generate schema"
HAS_VT=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="generate") | .inputSchema.properties.format.enum[] | select(.=="visual_test")' 2>/dev/null)
HAS_AR=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="generate") | .inputSchema.properties.format.enum[] | select(.=="annotation_report")' 2>/dev/null)
HAS_AI=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="generate") | .inputSchema.properties.format.enum[] | select(.=="annotation_issues")' 2>/dev/null)
if [ "$HAS_VT" = "visual_test" ] && [ "$HAS_AR" = "annotation_report" ] && [ "$HAS_AI" = "annotation_issues" ]; then
    auto_pass "All 3 annotation generate formats in schema"
else
    auto_fail "Missing generate formats: vt=$HAS_VT ar=$HAS_AR ai=$HAS_AI"
fi

step "1.4 — wait and session params in analyze schema"
HAS_WAIT=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="analyze") | .inputSchema.properties.wait.type // empty' 2>/dev/null)
HAS_SESSION=$(echo "$TOOLS" | jq -r '.result.tools[] | select(.name=="analyze") | .inputSchema.properties.session.type // empty' 2>/dev/null)
if [ "$HAS_WAIT" = "boolean" ] && [ "$HAS_SESSION" = "string" ]; then
    auto_pass "wait (boolean) and session (string) params in analyze schema"
else
    auto_fail "Missing analyze params: wait=$HAS_WAIT session=$HAS_SESSION"
fi

# ══════════════════════════════════════════════
# SECTION 2: Draw Mode Activation (MCP → Extension)
# ══════════════════════════════════════════════
header "2. Draw Mode Activation via MCP"

step "2.1 — interact(draw_mode_start)"
instruction "Calling interact({action: 'draw_mode_start'})..."
RESP=$(call_tool "interact" '{"action":"draw_mode_start"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response: $(echo "$TEXT" | head -3)${RESET}"
verify "Page shows red edge glow + 'Draw Mode' badge in top-right"

step "2.2 — Draw a rectangle"
instruction "Click and drag to draw a rectangle around any element on the page"
instruction "You should see a red dashed rectangle while dragging"
verify "Red rectangle appears while dragging, text input appears on mouseup"

step "2.3 — Type annotation text"
instruction "Type 'make this darker' and press Enter"
verify "Annotation is saved — you see a red semi-transparent overlay with number badge and your text"

step "2.4 — Draw a second annotation"
instruction "Draw another rectangle around a different element"
instruction "Type 'font too small' and press Enter"
verify "Two annotations visible with numbers 1 and 2"

# ══════════════════════════════════════════════
# SECTION 3: Exit Draw Mode & Results
# ══════════════════════════════════════════════
header "3. Exit Draw Mode (ESC)"

step "3.1 — Press ESC to exit draw mode"
instruction "Press ESC (without any active text input)"
verify "Red glow and overlay disappear. Draw mode badge is gone."

step "3.2 — Retrieve annotations"
instruction "Checking annotations..."
RESP=$(call_tool "analyze" '{"what":"annotations"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response: $(echo "$TEXT" | head -10)${RESET}"
if echo "$TEXT" | grep -q "make this darker" 2>/dev/null; then
    auto_pass "Annotation text 'make this darker' found in analyze response"
else
    echo -e "${YELLOW}    Note: Annotations may not match if you typed different text${RESET}"
    verify "Response contains your annotation text and screenshot path"
fi

step "3.3 — Annotation detail drill-down"
CORR_ID=$(echo "$TEXT" | grep -oE '"correlation_id":"[^"]+"' | head -1 | sed 's/"correlation_id":"//' | sed 's/"//')
if [ -n "$CORR_ID" ]; then
    instruction "Fetching detail for correlation_id=$CORR_ID..."
    RESP=$(call_tool "analyze" "{\"what\":\"annotation_detail\",\"correlation_id\":\"$CORR_ID\"}")
    DETAIL_TEXT=$(extract_text "$RESP")
    echo -e "${DIM}    Response: $(echo "$DETAIL_TEXT" | head -8)${RESET}"
    if echo "$DETAIL_TEXT" | grep -q "selector\|tag\|computed_styles" 2>/dev/null; then
        auto_pass "Detail response contains selector, tag, computed_styles"
    else
        verify "Detail response shows element selector, tag, and computed styles"
    fi
else
    echo -e "${YELLOW}    No correlation_id found — skipping detail test${RESET}"
    SKIP_COUNT=$((SKIP_COUNT + 1))
fi

step "3.4 — A11y auto-enrichment"
if echo "$DETAIL_TEXT" | grep -q "a11y_flags" 2>/dev/null; then
    auto_pass "a11y_flags field present in annotation detail"
else
    echo -e "${DIM}    Note: a11y_flags may be empty [] if no a11y issues detected${RESET}"
    verify "Annotation detail includes a11y_flags field (may be empty array)"
fi

# ══════════════════════════════════════════════
# SECTION 4: Blocking Wait
# ══════════════════════════════════════════════
header "4. Blocking Wait (wait: true)"

step "4.1 — Start draw mode + issue blocking wait"
instruction "This will start draw mode, then issue a blocking analyze call."
instruction "Draw at least 1 annotation, then press ESC to unblock."
echo ""

# Start draw mode
call_tool "interact" '{"action":"draw_mode_start"}' >/dev/null 2>&1

instruction "Draw mode started. Now issuing blocking wait (5 second timeout for test)..."
instruction "Draw an annotation and press ESC within 5 seconds."
echo ""

# Issue blocking wait with short timeout
RESP=$(call_tool "analyze" '{"what":"annotations","wait":true,"timeout_ms":5000}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response: $(echo "$TEXT" | head -5)${RESET}"

if echo "$TEXT" | grep -q "waiting\|Timed out" 2>/dev/null; then
    echo -e "${YELLOW}    Timed out (that's OK if you didn't finish in time)${RESET}"
    verify "Response shows status='waiting' — expected if you didn't finish in 5s"
else
    verify "Blocking call returned your annotations immediately after ESC"
fi

# ══════════════════════════════════════════════
# SECTION 5: Multi-Page Sessions
# ══════════════════════════════════════════════
header "5. Multi-Page Named Sessions"

step "5.1 — Start named session on current page"
instruction "Starting named session 'smoke-test'..."
call_tool "interact" '{"action":"draw_mode_start","session":"smoke-test"}' >/dev/null 2>&1
instruction "Draw 1 annotation on this page, then press ESC"
verify "Draw mode activated, annotation drawn, ESC exits"

step "5.2 — Navigate to a different page and continue session"
instruction "Navigate to a different page (e.g. click a link)"
instruction "Wait for page load, then we'll start draw mode again on the new page"
echo -en "    ${BOLD}Press Enter when you're on the new page...${RESET}"
read -r

call_tool "interact" '{"action":"draw_mode_start","session":"smoke-test"}' >/dev/null 2>&1
instruction "Draw 1 annotation on this page, then press ESC"
verify "Draw mode activated on new page, annotation drawn"

step "5.3 — Retrieve named session"
RESP=$(call_tool "analyze" '{"what":"annotations","session":"smoke-test"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response: $(echo "$TEXT" | head -10)${RESET}"
if echo "$TEXT" | grep -q "page_count" 2>/dev/null; then
    PAGE_COUNT=$(echo "$TEXT" | grep -oE '"page_count":[0-9]+' | head -1 | sed 's/"page_count"://')
    if [ "$PAGE_COUNT" = "2" ]; then
        auto_pass "Named session 'smoke-test' has 2 pages"
    else
        verify "Named session should have 2 pages (got $PAGE_COUNT)"
    fi
else
    verify "Response contains page_count and both pages' annotations"
fi

# ══════════════════════════════════════════════
# SECTION 6: Generate Artifacts
# ══════════════════════════════════════════════
header "6. Generate Artifacts from Annotations"

step "6.1 — generate(visual_test)"
RESP=$(call_tool "generate" '{"format":"visual_test"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response (first 8 lines):${RESET}"
echo "$TEXT" | head -8 | sed 's/^/    /'
if echo "$TEXT" | grep -q "test(" 2>/dev/null && echo "$TEXT" | grep -q "page.goto" 2>/dev/null; then
    auto_pass "visual_test output contains Playwright test() and page.goto()"
else
    verify "Response contains a valid Playwright test with test() and page.goto()"
fi

step "6.2 — generate(visual_test) with named session"
RESP=$(call_tool "generate" '{"format":"visual_test","session":"smoke-test"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response (first 10 lines):${RESET}"
echo "$TEXT" | head -10 | sed 's/^/    /'
verify "Playwright test covers both pages from the named session"

step "6.3 — generate(annotation_report)"
RESP=$(call_tool "generate" '{"format":"annotation_report"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response (first 15 lines):${RESET}"
echo "$TEXT" | head -15 | sed 's/^/    /'
if echo "$TEXT" | grep -q "# Annotation Report" 2>/dev/null; then
    auto_pass "annotation_report contains Markdown header"
else
    verify "Response is a Markdown report with header, annotations, and screenshot ref"
fi

step "6.4 — generate(annotation_issues)"
RESP=$(call_tool "generate" '{"format":"annotation_issues"}')
TEXT=$(extract_text "$RESP")
echo -e "${DIM}    Response (first 10 lines):${RESET}"
echo "$TEXT" | head -10 | sed 's/^/    /'
if echo "$TEXT" | grep -q "issues" 2>/dev/null && echo "$TEXT" | grep -q "total_count" 2>/dev/null; then
    auto_pass "annotation_issues contains issues array and total_count"
else
    verify "Response is structured JSON with issues array and counts"
fi

# ══════════════════════════════════════════════
# SECTION 7: Edge Cases
# ══════════════════════════════════════════════
header "7. Edge Cases"

step "7.1 — Double activation returns already_active"
call_tool "interact" '{"action":"draw_mode_start"}' >/dev/null 2>&1
RESP=$(call_tool "interact" '{"action":"draw_mode_start"}')
TEXT=$(extract_text "$RESP")
if echo "$TEXT" | grep -qi "already.active\|already_active" 2>/dev/null; then
    auto_pass "Second draw_mode_start returns already_active"
else
    echo -e "${DIM}    Response: $(echo "$TEXT" | head -3)${RESET}"
    verify "Second activation returns already_active status"
fi

step "7.2 — Cleanup: exit draw mode if still active"
instruction "Press ESC if draw mode is still active, or just press Enter"
echo -en "    ${BOLD}Press Enter to continue...${RESET}"
read -r
echo ""

step "7.3 — Tiny rectangle (< 5px) is ignored"
instruction "Start draw mode, then click without dragging (or drag < 5px)"
call_tool "interact" '{"action":"draw_mode_start"}' >/dev/null 2>&1
instruction "Click and release without dragging, then press ESC"
verify "No annotation created for tiny/zero-size rectangle"

# ══════════════════════════════════════════════
# Results
# ══════════════════════════════════════════════
header "Results"
TOTAL=$((PASS_COUNT + FAIL_COUNT + SKIP_COUNT))
echo -e "  ${GREEN}✔ Passed: $PASS_COUNT${RESET}"
echo -e "  ${RED}✖ Failed: $FAIL_COUNT${RESET}"
echo -e "  ${YELLOW}⊘ Skipped: $SKIP_COUNT${RESET}"
echo -e "  ${DIM}  Total: $TOTAL${RESET}"
echo ""

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "  ${GREEN}${BOLD}All tests passed!${RESET}"
else
    echo -e "  ${RED}${BOLD}$FAIL_COUNT test(s) failed.${RESET}"
fi
echo ""

exit "$FAIL_COUNT"
