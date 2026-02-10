#!/bin/bash
# cat-20-noise-persistence.sh — UAT for Noise Rule Persistence (v5.8.1+)
# Tests that user rules persist, filter correctly, and survive restarts.
# Integrated into comprehensive test suite as Category 20.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/framework.sh"

init_framework "$1" "$2"

begin_category "20" "Noise Persistence" "5"

# Clean up any previous state
rm -rf ".gasoline/noise" 2>/dev/null || true

# ── Test 20.1: Rules persist across server restarts ──────────────
begin_test "20.1" "Rules persist to disk and survive restart" \
    "Add rule, restart daemon, verify rule still exists" \
    "Persistence must survive crashes/kills"

# Start daemon and add a rule
start_daemon

# Add rule via stdin invocation (single call, not send_mcp)
request='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"test","match_spec":{"message_regex":"test.*pattern"}}]}}}'
response=$(echo "$request" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

# Verify response indicates success
if echo "$response" | jq -e '.result' >/dev/null 2>&1; then
    pass "Rule added to configure tool"
else
    fail "Failed to add rule" "Response: $response"
fi

sleep 0.5

# Verify file exists
if [ -f ".gasoline/noise/rules.json" ]; then
    if jq . ".gasoline/noise/rules.json" > /dev/null 2>&1; then
        pass "Rules persisted to .gasoline/noise/rules.json"
    else
        fail "Persisted file invalid JSON"
    fi
else
    fail "Persisted file not created"
fi

# Kill and restart daemon
kill_server
sleep 0.5
start_daemon
sleep 0.5

# List rules via stdin - should load the persisted rule
request2='{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"list"}}}'
response2=$(echo "$request2" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

# Check if user_1 rule is present in response
if echo "$response2" | jq -e '.result.content[0].text | contains("user_1")' >/dev/null 2>&1; then
    pass "User rules reloaded after restart"
else
    fail "User rules not reloaded after restart"
fi

# ── Test 20.2: ID counter prevents collisions ──────────────────
begin_test "20.2" "ID counter persisted - next rule gets user_2" \
    "Add another rule after restart, verify ID increments" \
    "Counter must prevent collisions (user_1, user_2, not user_1 twice)"

request3='{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"add","rules":[{"category":"console","classification":"test2","match_spec":{"message_regex":"second.*pattern"}}]}}}'
response3=$(echo "$request3" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

# Verify the new rule was added
if echo "$response3" | jq -e '.result' >/dev/null 2>&1; then
    pass "Second rule added successfully"
else
    fail "Failed to add second rule"
fi

# Check persisted file for both user_1 and user_2
if [ -f ".gasoline/noise/rules.json" ]; then
    user1_count=$(jq '[.rules[] | select(.id == "user_1")] | length' ".gasoline/noise/rules.json")
    user2_count=$(jq '[.rules[] | select(.id == "user_2")] | length' ".gasoline/noise/rules.json")

    if [ "$user1_count" = "1" ] && [ "$user2_count" = "1" ]; then
        pass "Both user_1 and user_2 rules persisted (no collision)"
    else
        fail "ID collision detected" "user_1=$user1_count, user_2=$user2_count (expected 1,1)"
    fi
else
    fail "Persisted file missing after second add"
fi

# ── Test 20.3: RemoveRule persists across restart ────────────────
begin_test "20.3" "RemoveRule persists - deleted rule stays gone" \
    "Remove a rule, restart, verify it's gone permanently" \
    "Deletions must persist without re-adding"

# Remove user_1
request4='{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"remove","rule_id":"user_1"}}}'
response4=$(echo "$request4" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

if echo "$response4" | jq -e '.result' >/dev/null 2>&1; then
    pass "RemoveRule executed"
else
    fail "RemoveRule failed"
fi

sleep 0.5

# Kill and restart
kill_server
sleep 0.5
start_daemon
sleep 0.5

# List rules - user_1 should be gone
request5='{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"list"}}}'
response5=$(echo "$request5" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

if echo "$response5" | jq -e '.result.content[0].text | contains("user_2")' >/dev/null 2>&1 && ! echo "$response5" | jq -e '.result.content[0].text | contains("user_1")' >/dev/null 2>&1; then
    pass "Removed rule user_1 stays deleted after restart"
else
    fail "Removed rule reappeared or user_2 missing"
fi

# ── Test 20.4: Reset clears all user rules ────────────────────
begin_test "20.4" "Reset persists empty state" \
    "Call reset, restart, verify only built-ins remain" \
    "Reset must clear persistence completely"

request6='{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"reset"}}}'
response6=$(echo "$request6" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

if echo "$response6" | jq -e '.result' >/dev/null 2>&1; then
    pass "Reset executed"
else
    fail "Reset failed"
fi

sleep 0.5

# Kill and restart
kill_server
sleep 0.5
start_daemon
sleep 0.5

# List rules - should only have built-ins
request7='{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"list"}}}'
response7=$(echo "$request7" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

# Check that user rules are gone and built-ins are present
if echo "$response7" | jq -e '.result.content[0].text | contains("builtin_")' >/dev/null 2>&1 && ! echo "$response7" | jq -e '.result.content[0].text | contains("user_")' >/dev/null 2>&1; then
    pass "Reset cleared all user rules, built-ins remain"
else
    fail "Reset didn't persist or built-ins missing"
fi

# ── Test 20.5: Corrupted file recovery ──────────────────────────
begin_test "20.5" "Corrupted file doesn't crash server" \
    "Corrupt file, restart, verify graceful recovery" \
    "Corruption must not crash server, built-ins must reload"

# Corrupt the file
if [ -f ".gasoline/noise/rules.json" ]; then
    echo "{invalid json}" > ".gasoline/noise/rules.json"
fi

# Kill and try to restart
kill_server
sleep 0.5

# Attempt to start daemon - should succeed despite corruption
if start_daemon; then
    pass "Server started despite corrupted persistence file"

    # Verify built-ins still load
    request8='{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"configure","arguments":{"action":"noise_rule","noise_action":"list"}}}'
    response8=$(echo "$request8" | $TIMEOUT_CMD 8 $WRAPPER --port "$PORT" 2>&1 | grep '^{' | head -1)

    if echo "$response8" | jq -e '.result.content[0].text | contains("builtin_")' >/dev/null 2>&1; then
        pass "Built-in rules reloaded fresh after corruption"
    else
        fail "Built-in rules not reloaded"
    fi
else
    fail "Server crashed on corrupted file"
fi

# ── Cleanup ──────────────────────────────────────────────────────
finish_category
