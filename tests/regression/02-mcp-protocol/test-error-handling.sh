#!/usr/bin/env bash
# Test: MCP error handling
#
# Verifies:
# - Invalid JSON returns proper error
# - Unknown method returns proper error
# - Missing parameters return proper error
# - Error responses have correct JSON-RPC structure

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# Start server for this test
start_server || exit 1

echo "Testing MCP error handling..."

# Test 1: Invalid JSON returns error
test_invalid_json() {
    local response
    response=$(curl -s -X POST "${GASOLINE_URL}/mcp" \
        -H "Content-Type: application/json" \
        -d 'not valid json')

    assert_valid_json "$response" "Error response should be valid JSON"
    assert_mcp_error "$response" "Invalid JSON should return error"
}

# Test 2: Unknown method returns error
test_unknown_method() {
    local response
    response=$(mcp_call "unknown/method" "{}")

    assert_mcp_error "$response" "Unknown method should return error"
}

# Test 3: Missing 'what' parameter for observe returns error
test_observe_missing_what() {
    local response
    response=$(mcp_tool "observe" "{}")

    assert_mcp_success "$response" "observe without 'what' should still return success response"

    # But the content should indicate an error
    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "what" "Error should mention missing 'what' parameter"
}

# Test 4: Error response has jsonrpc field
test_error_has_jsonrpc() {
    local response
    response=$(mcp_call "unknown/method" "{}")

    assert_json_equals "$response" ".jsonrpc" "2.0" "Error should have jsonrpc: 2.0"
}

# Test 5: Error response has id field (matching request)
test_error_has_id() {
    local response
    response=$(curl -s -X POST "${GASOLINE_URL}/mcp" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":42,"method":"unknown/method","params":{}}')

    local id
    id=$(echo "$response" | jq '.id')
    assert_equals "42" "$id" "Error response id should match request id"
}

# Test 6: Invalid observe mode returns structured error
test_invalid_observe_mode() {
    local response
    response=$(mcp_tool "observe" '{"what":"invalid_mode_xyz"}')

    assert_mcp_success "$response" "Invalid mode should still return success response"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "invalid_mode_xyz" "Error should mention the invalid mode"
}

# Run tests
FAILED=0

run_test "Invalid JSON returns error" test_invalid_json || ((FAILED++))
run_test "Unknown method returns error" test_unknown_method || ((FAILED++))
run_test "observe() without 'what' mentions error" test_observe_missing_what || ((FAILED++))
run_test "Error has jsonrpc field" test_error_has_jsonrpc || ((FAILED++))
run_test "Error id matches request" test_error_has_id || ((FAILED++))
run_test "Invalid observe mode returns error" test_invalid_observe_mode || ((FAILED++))

exit $FAILED
