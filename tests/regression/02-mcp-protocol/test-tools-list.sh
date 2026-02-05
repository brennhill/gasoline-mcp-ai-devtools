#!/usr/bin/env bash
# Test: MCP tools/list endpoint
#
# Verifies:
# - tools/list returns valid JSON-RPC response
# - Response contains expected tools (observe, configure, generate, interact)
# - Tool schemas are present

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# Start server for this test
start_server || exit 1

echo "Testing MCP tools/list..."

# Test 1: tools/list returns success
test_tools_list_success() {
    local response
    response=$(mcp_call "tools/list" "{}")
    assert_mcp_success "$response" "tools/list should return success"
}

# Test 2: Response has tools array
test_tools_list_has_tools() {
    local response
    response=$(mcp_call "tools/list" "{}")
    assert_json_path "$response" ".result.tools" "Response should have .result.tools"
}

# Test 3: Tools array is not empty
test_tools_list_not_empty() {
    local response
    response=$(mcp_call "tools/list" "{}")
    assert_json_array_not_empty "$response" ".result.tools" "Tools array should not be empty"
}

# Test 4: Has 'observe' tool
test_has_observe_tool() {
    local response
    response=$(mcp_call "tools/list" "{}")
    local has_observe
    has_observe=$(echo "$response" | jq '[.result.tools[].name] | contains(["observe"])')
    assert_equals "true" "$has_observe" "Should have 'observe' tool"
}

# Test 5: Has 'configure' tool
test_has_configure_tool() {
    local response
    response=$(mcp_call "tools/list" "{}")
    local has_configure
    has_configure=$(echo "$response" | jq '[.result.tools[].name] | contains(["configure"])')
    assert_equals "true" "$has_configure" "Should have 'configure' tool"
}

# Test 6: Has 'generate' tool
test_has_generate_tool() {
    local response
    response=$(mcp_call "tools/list" "{}")
    local has_generate
    has_generate=$(echo "$response" | jq '[.result.tools[].name] | contains(["generate"])')
    assert_equals "true" "$has_generate" "Should have 'generate' tool"
}

# Test 7: Has 'interact' tool
test_has_interact_tool() {
    local response
    response=$(mcp_call "tools/list" "{}")
    local has_interact
    has_interact=$(echo "$response" | jq '[.result.tools[].name] | contains(["interact"])')
    assert_equals "true" "$has_interact" "Should have 'interact' tool"
}

# Test 8: Each tool has inputSchema
test_tools_have_schema() {
    local response
    response=$(mcp_call "tools/list" "{}")
    local tools_without_schema
    tools_without_schema=$(echo "$response" | jq '[.result.tools[] | select(.inputSchema == null)] | length')
    assert_equals "0" "$tools_without_schema" "All tools should have inputSchema"
}

# Run tests
FAILED=0

run_test "tools/list returns success" test_tools_list_success || ((FAILED++))
run_test "Response has tools array" test_tools_list_has_tools || ((FAILED++))
run_test "Tools array is not empty" test_tools_list_not_empty || ((FAILED++))
run_test "Has 'observe' tool" test_has_observe_tool || ((FAILED++))
run_test "Has 'configure' tool" test_has_configure_tool || ((FAILED++))
run_test "Has 'generate' tool" test_has_generate_tool || ((FAILED++))
run_test "Has 'interact' tool" test_has_interact_tool || ((FAILED++))
run_test "All tools have inputSchema" test_tools_have_schema || ((FAILED++))

exit $FAILED
