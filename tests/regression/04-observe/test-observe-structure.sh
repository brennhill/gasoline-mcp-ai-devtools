#!/usr/bin/env bash
# Test: Observe response structure
#
# Verifies:
# - Stub implementations return clear "not implemented" messages
# - Implemented modes return expected data structures
# - Empty state returns empty arrays, not errors

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# Start server for this test
start_server || exit 1

echo "Testing observe response structures..."

# Test 1: 'api' mode returns "not implemented" message
test_api_not_implemented() {
    local response
    response=$(mcp_tool "observe" '{"what":"api"}')

    assert_mcp_success "$response" "observe(api) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "not_implemented" "API mode should indicate not implemented"
}

# Test 2: 'changes' mode returns "not implemented" message
test_changes_not_implemented() {
    local response
    response=$(mcp_tool "observe" '{"what":"changes"}')

    assert_mcp_success "$response" "observe(changes) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "not_implemented" "Changes mode should indicate not implemented"
}

# Test 3: 'logs' mode with fresh state returns success (not MCP error)
test_logs_fresh_state() {
    # Start fresh server with clean state
    stop_server
    start_server

    local response
    response=$(mcp_tool "observe" '{"what":"logs"}')

    # Should succeed (no MCP error) even with empty state
    assert_mcp_success "$response" "observe(logs) with fresh state should succeed"

    # Response should have the expected structure
    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "logs" "Logs response should mention 'logs'"
}

# Test 4: 'pilot' mode returns extension_connected status
test_pilot_has_connection_status() {
    local response
    response=$(mcp_tool "observe" '{"what":"pilot"}')

    assert_mcp_success "$response" "observe(pilot) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "extension_connected" "Pilot should report extension_connected"
}

# Test 5: 'tabs' mode returns tabs array
test_tabs_has_tabs_array() {
    local response
    response=$(mcp_tool "observe" '{"what":"tabs"}')

    assert_mcp_success "$response" "observe(tabs) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "tabs" "Tabs response should have tabs field"
}

# Test 6: 'timeline' mode returns entries array
test_timeline_has_entries() {
    local response
    response=$(mcp_tool "observe" '{"what":"timeline"}')

    assert_mcp_success "$response" "observe(timeline) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "entries" "Timeline should have entries field"
}

# Test 7: 'error_clusters' mode returns clusters
test_error_clusters_has_clusters() {
    local response
    response=$(mcp_tool "observe" '{"what":"error_clusters"}')

    assert_mcp_success "$response" "observe(error_clusters) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "clusters" "Error clusters should have clusters field"
}

# Test 8: 'history' mode returns entries
test_history_has_entries() {
    local response
    response=$(mcp_tool "observe" '{"what":"history"}')

    assert_mcp_success "$response" "observe(history) should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "entries" "History should have entries field"
}

# Run tests
FAILED=0

run_test "API mode shows 'not implemented'" test_api_not_implemented || ((FAILED++))
run_test "Changes mode shows 'not implemented'" test_changes_not_implemented || ((FAILED++))
run_test "Logs fresh state returns success" test_logs_fresh_state || ((FAILED++))
run_test "Pilot has connection status" test_pilot_has_connection_status || ((FAILED++))
run_test "Tabs has tabs array" test_tabs_has_tabs_array || ((FAILED++))
run_test "Timeline has entries" test_timeline_has_entries || ((FAILED++))
run_test "Error clusters has clusters" test_error_clusters_has_clusters || ((FAILED++))
run_test "History has entries" test_history_has_entries || ((FAILED++))

exit $FAILED
