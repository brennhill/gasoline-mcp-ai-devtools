#!/bin/bash
# Automated Test Generation Validation Script
# Prerequisites:
#   - Demo site running on http://localhost:3000
#   - Gasoline MCP server running
#   - Chrome with Gasoline extension connected

set -e

MCP_SERVER="http://localhost:8080"  # Adjust if needed
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="$SCRIPT_DIR/../validation-results"
mkdir -p "$RESULTS_DIR"

echo "=== Gasoline Test Generation Validation ==="
echo ""
echo "This script will:"
echo "1. Use interact to navigate to demo site and trigger bugs"
echo "2. Use observe to verify errors were captured"
echo "3. Use generate to create tests from captured context"
echo "4. Validate the generated test quality"
echo ""
echo "Press Enter to start, or Ctrl+C to cancel..."
read

# Function to send JSON-RPC request to MCP server via stdin
function mcp_call() {
    local method=$1
    local args=$2
    local id=${3:-1}

    echo "{\"jsonrpc\":\"2.0\",\"id\":$id,\"method\":\"tools/call\",\"params\":{\"name\":\"$method\",\"arguments\":$args}}"
}

# Step 1: Navigate to demo site
echo ""
echo "Step 1: Navigating to demo site using interact tool..."
mcp_call "interact" '{"action":"navigate","url":"http://localhost:3000"}' 1 | \
    ~/dev/gasoline/dist/gasoline > "$RESULTS_DIR/01-navigate.json"

echo "✓ Navigated to demo site"
sleep 2

# Step 2: Click Chat button to trigger WebSocket bugs
echo ""
echo "Step 2: Clicking Chat button to trigger WebSocket errors..."
mcp_call "interact" '{"action":"click","selector":"[data-testid=\"chat-button\"]","description":"Open chat widget"}' 2 | \
    ~/dev/gasoline/dist/gasoline > "$RESULTS_DIR/02-click-chat.json"

echo "✓ Clicked Chat button"
sleep 3  # Wait for WebSocket errors to occur

# Step 3: Check WebSocket capture (unique to Gasoline!)
echo ""
echo "Step 3: Verifying WebSocket frames were captured (UNIQUE TO GASOLINE)..."
mcp_call "observe" '{"mode":"websocket"}' 3 | \
    ~/dev/gasoline/dist/gasoline > "$RESULTS_DIR/03-observe-websocket.json"

# Check if WebSocket data was captured
if grep -q "framereceived\|framesent\|ws" "$RESULTS_DIR/03-observe-websocket.json"; then
    echo "✓ WebSocket frames captured!"
else
    echo "⚠ Warning: No WebSocket frames found (may need more time)"
fi

# Step 4: Check console errors
echo ""
echo "Step 4: Verifying console errors were captured..."
mcp_call "observe" '{"mode":"errors"}' 4 | \
    ~/dev/gasoline/dist/gasoline > "$RESULTS_DIR/04-observe-errors.json"

if grep -q "error\|Error" "$RESULTS_DIR/04-observe-errors.json"; then
    echo "✓ Console errors captured!"
else
    echo "⚠ Warning: No console errors found"
fi

# Step 5: Generate test from error context
echo ""
echo "Step 5: Generating Playwright test from captured error context..."
mcp_call "generate" '{"format":"test_from_context","context":"error","framework":"playwright","base_url":"http://localhost:3000"}' 5 | \
    ~/dev/gasoline/dist/gasoline > "$RESULTS_DIR/05-generate-from-error.json"

if grep -q "test\|spec\|playwright" "$RESULTS_DIR/05-generate-from-error.json"; then
    echo "✓ Test generated from error!"

    # Extract test content and save to file
    cat "$RESULTS_DIR/05-generate-from-error.json" | \
        grep -o '"content":"[^"]*"' | \
        sed 's/"content":"//;s/"$//' | \
        sed 's/\\n/\n/g' > "$RESULTS_DIR/generated-error-test.spec.ts"

    echo "  Saved to: $RESULTS_DIR/generated-error-test.spec.ts"
else
    echo "✗ Failed to generate test from error"
fi

# Step 6: Generate test from interaction context
echo ""
echo "Step 6: Generating test from interaction context..."
mcp_call "generate" '{"format":"test_from_context","context":"interaction","framework":"playwright","base_url":"http://localhost:3000"}' 6 | \
    ~/dev/gasoline/dist/gasoline > "$RESULTS_DIR/06-generate-from-interaction.json"

if grep -q "test\|spec\|playwright" "$RESULTS_DIR/06-generate-from-interaction.json"; then
    echo "✓ Test generated from interaction!"

    # Extract test content
    cat "$RESULTS_DIR/06-generate-from-interaction.json" | \
        grep -o '"content":"[^"]*"' | \
        sed 's/"content":"//;s/"$//' | \
        sed 's/\\n/\n/g' > "$RESULTS_DIR/generated-interaction-test.spec.ts"

    echo "  Saved to: $RESULTS_DIR/generated-interaction-test.spec.ts"
else
    echo "✗ Failed to generate test from interaction"
fi

# Step 7: Validate generated test quality
echo ""
echo "Step 7: Validating generated test quality..."

if [ -f "$RESULTS_DIR/generated-error-test.spec.ts" ]; then
    # Check for key Playwright patterns
    CHECKS_PASSED=0
    TOTAL_CHECKS=6

    echo "  Checking generated test structure..."

    if grep -q "import.*@playwright/test" "$RESULTS_DIR/generated-error-test.spec.ts"; then
        echo "    ✓ Has Playwright imports"
        ((CHECKS_PASSED++))
    else
        echo "    ✗ Missing Playwright imports"
    fi

    if grep -q "test(" "$RESULTS_DIR/generated-error-test.spec.ts"; then
        echo "    ✓ Has test() declaration"
        ((CHECKS_PASSED++))
    else
        echo "    ✗ Missing test() declaration"
    fi

    if grep -q "page.goto" "$RESULTS_DIR/generated-error-test.spec.ts"; then
        echo "    ✓ Navigates to page"
        ((CHECKS_PASSED++))
    else
        echo "    ✗ Missing page.goto()"
    fi

    if grep -q "page.locator\|page.getByTestId\|page.getByRole" "$RESULTS_DIR/generated-error-test.spec.ts"; then
        echo "    ✓ Uses stable selectors"
        ((CHECKS_PASSED++))
    else
        echo "    ✗ No selector methods found"
    fi

    if grep -q "expect(" "$RESULTS_DIR/generated-error-test.spec.ts"; then
        echo "    ✓ Has assertions"
        ((CHECKS_PASSED++))
    else
        echo "    ✗ No assertions found"
    fi

    if grep -q "websocket\|WebSocket" "$RESULTS_DIR/generated-error-test.spec.ts"; then
        echo "    ✓ Includes WebSocket monitoring (UNIQUE!)"
        ((CHECKS_PASSED++))
    else
        echo "    ⚠ No WebSocket monitoring (may not be relevant)"
    fi

    echo ""
    echo "  Quality score: $CHECKS_PASSED/$TOTAL_CHECKS checks passed"

    if [ $CHECKS_PASSED -ge 4 ]; then
        echo "  ✓ Generated test is high quality!"
    else
        echo "  ⚠ Generated test needs review"
    fi
fi

# Final summary
echo ""
echo "=== Validation Complete ==="
echo ""
echo "Results saved to: $RESULTS_DIR"
echo ""
echo "Files created:"
ls -lh "$RESULTS_DIR" | tail -n +2
echo ""
echo "Next steps:"
echo "1. Review generated tests in $RESULTS_DIR"
echo "2. Copy tests to demo site: cp $RESULTS_DIR/*.spec.ts ~/dev/gasoline-demos/tests/"
echo "3. Run tests: cd ~/dev/gasoline-demos && npx playwright test"
echo ""
echo "To view results:"
echo "  cat $RESULTS_DIR/generated-error-test.spec.ts"
echo "  cat $RESULTS_DIR/generated-interaction-test.spec.ts"
