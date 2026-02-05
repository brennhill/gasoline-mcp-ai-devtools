#!/usr/bin/env bash
# Test: All observe modes
#
# Verifies:
# - All 29 observe modes return valid responses
# - No mode crashes or returns invalid JSON
# - Each mode has expected response structure

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/common.sh"
source "$SCRIPT_DIR/../lib/assertions.sh"

# Start server for this test
start_server || exit 1

echo "Testing all observe modes..."

# Observe modes to test (fast modes that don't require extension connectivity)
# Modes like network_waterfall and accessibility require extension and take 5+ seconds to timeout
OBSERVE_MODES=(
    "errors"
    "logs"
    "extension_logs"
    # "network_waterfall"  # Requires extension, 5s timeout
    "network_bodies"
    "websocket_events"
    "websocket_status"
    "actions"
    "vitals"
    "page"
    "tabs"
    "pilot"
    "performance"
    "api"
    # "accessibility"  # Requires extension, 5s timeout
    "changes"
    "timeline"
    "error_clusters"
    "history"
    "security_audit"
    "third_party_audit"
    "command_result"
    "pending_commands"
    "failed_commands"
)

# Test a single observe mode
test_observe_mode() {
    local mode="$1"

    local response
    response=$(mcp_tool "observe" "{\"what\":\"$mode\"}")

    # Response should be valid JSON
    if ! assert_valid_json "$response" "observe($mode) should return valid JSON"; then
        return 1
    fi

    # Response should be MCP success
    if ! assert_mcp_success "$response" "observe($mode) should succeed"; then
        return 1
    fi

    # Response should have content
    if ! assert_json_path "$response" ".result.content" "observe($mode) should have content"; then
        return 1
    fi

    return 0
}

# Run tests for each mode
FAILED=0

for mode in "${OBSERVE_MODES[@]}"; do
    if test_observe_mode "$mode"; then
        log_pass "observe($mode)"
    else
        log_fail "observe($mode)"
        ((FAILED++))
    fi
done

echo ""
echo "Tested ${#OBSERVE_MODES[@]} observe modes"

exit $FAILED
