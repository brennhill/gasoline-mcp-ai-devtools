#!/usr/bin/env bash
# Test: Extension data flow
#
# Verifies:
# - Data POSTed to endpoints appears via observe tool
# - Logs flow: POST /logs -> observe({what:"logs"})
# - Actions flow: POST /enhanced-actions -> observe({what:"actions"})
# - WebSocket events flow: POST /websocket-events -> observe({what:"websocket_events"})

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# Start server for this test
start_server || exit 1

echo "Testing extension data flow..."

# Unique marker for this test run
TEST_MARKER="REGRESSION_TEST_$(date +%s)"

# Test 1: Logs data flow
test_logs_data_flow() {
    # POST a log entry
    post_data "/logs" "{
        \"entries\": [{
            \"type\": \"console\",
            \"level\": \"error\",
            \"message\": \"Test error: $TEST_MARKER\",
            \"url\": \"http://test.com/page\",
            \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
        }]
    }"

    # Small delay for processing
    sleep 0.5

    # Retrieve via observe
    local response
    response=$(mcp_tool "observe" '{"what":"logs"}')

    assert_mcp_success "$response" "observe logs should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "$TEST_MARKER" "Logs should contain test marker"
}

# Test 2: Actions data flow
test_actions_data_flow() {
    local action_marker="ACTION_$TEST_MARKER"

    # POST an action
    post_data "/enhanced-actions" "{
        \"actions\": [{
            \"type\": \"click\",
            \"timestamp\": $(date +%s000),
            \"url\": \"http://test.com/actions\",
            \"selectors\": {\"css\": \"button.$action_marker\"}
        }]
    }"

    sleep 0.5

    # Retrieve via observe
    local response
    response=$(mcp_tool "observe" '{"what":"actions"}')

    assert_mcp_success "$response" "observe actions should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "$action_marker" "Actions should contain test marker"
}

# Test 3: WebSocket events data flow
test_websocket_data_flow() {
    local ws_marker="WS_$TEST_MARKER"

    # POST a WebSocket event
    post_data "/websocket-events" "{
        \"events\": [{
            \"event\": \"message\",
            \"id\": \"ws-$ws_marker\",
            \"direction\": \"incoming\",
            \"data\": \"Test message: $ws_marker\",
            \"ts\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"
        }]
    }"

    sleep 0.5

    # Retrieve via observe
    local response
    response=$(mcp_tool "observe" '{"what":"websocket_events"}')

    assert_mcp_success "$response" "observe websocket_events should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "$ws_marker" "WebSocket events should contain test marker"
}

# Test 4: Network bodies data flow
test_network_bodies_data_flow() {
    local net_marker="NET_$TEST_MARKER"

    # POST network body data
    post_data "/network-bodies" "{
        \"bodies\": [{
            \"url\": \"http://api.test.com/$net_marker\",
            \"method\": \"GET\",
            \"status\": 200,
            \"requestBody\": \"\",
            \"responseBody\": \"{\\\"test\\\": \\\"$net_marker\\\"}\"
        }]
    }"

    sleep 0.5

    # Retrieve via observe
    local response
    response=$(mcp_tool "observe" '{"what":"network_bodies"}')

    assert_mcp_success "$response" "observe network_bodies should succeed"

    local content
    content=$(echo "$response" | jq -r '.result.content[0].text // ""')
    assert_contains "$content" "$net_marker" "Network bodies should contain test marker"
}

# Run tests
FAILED=0

run_test "Logs data flows through" test_logs_data_flow || ((FAILED++))
run_test "Actions data flows through" test_actions_data_flow || ((FAILED++))
run_test "WebSocket data flows through" test_websocket_data_flow || ((FAILED++))
run_test "Network bodies data flows through" test_network_bodies_data_flow || ((FAILED++))

exit $FAILED
