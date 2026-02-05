#!/usr/bin/env bash
# Test: Server startup and health endpoint
#
# Verifies:
# - Server starts within 2 seconds
# - Health endpoint responds with 200
# - Health response contains expected fields

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# Start server for this test
start_server || exit 1

echo "Testing health endpoint..."

# Test 1: Health endpoint returns 200
test_health_status() {
    assert_http_status "${GASOLINE_URL}/health" "200" "Health endpoint should return 200"
}

# Test 2: Health response is valid JSON
test_health_json() {
    local response
    response=$(get_data "/health")
    assert_valid_json "$response" "Health response should be valid JSON"
}

# Test 3: Health response has version field
test_health_has_version() {
    local response
    response=$(get_data "/health")
    assert_json_has_key "$response" "version" "Health response should have 'version' field"
}

# Test 4: Health response has status field
test_health_has_status() {
    local response
    response=$(get_data "/health")
    assert_json_has_key "$response" "status" "Health response should have 'status' field"
}

# Test 5: Status is "ok"
test_health_status_ok() {
    local response
    response=$(get_data "/health")
    assert_json_equals "$response" ".status" "ok" "Health status should be 'ok'"
}

# Run tests
FAILED=0

run_test "Health returns 200" test_health_status || ((FAILED++))
run_test "Health returns valid JSON" test_health_json || ((FAILED++))
run_test "Health has version" test_health_has_version || ((FAILED++))
run_test "Health has status" test_health_has_status || ((FAILED++))
run_test "Health status is ok" test_health_status_ok || ((FAILED++))

exit $FAILED
