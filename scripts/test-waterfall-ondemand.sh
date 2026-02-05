#!/bin/bash
# UAT: Test on-demand waterfall fetching
# This script verifies that observe(network_waterfall) fetches fresh data from extension

set -e

PORT=${GASOLINE_PORT:-47152}
SERVER_URL="http://localhost:$PORT"

echo "=== On-Demand Waterfall UAT ==="
echo ""

# Check if server is running
echo "1. Checking server health..."
HEALTH=$(curl -s "$SERVER_URL/health" 2>/dev/null || echo "FAILED")
if echo "$HEALTH" | grep -q "extension_connected"; then
    EXT_CONNECTED=$(echo "$HEALTH" | grep -o '"extension_connected":[^,}]*' | cut -d: -f2)
    echo "   Server: ✅ Running"
    echo "   Extension connected: $EXT_CONNECTED"
else
    echo "   ❌ Server not running at $SERVER_URL"
    echo "   Start with: go run ./cmd/dev-console"
    exit 1
fi

# Check if extension is connected
if echo "$HEALTH" | grep -q '"extension_connected":true'; then
    echo ""
    echo "2. Extension is connected - proceeding with test"
else
    echo ""
    echo "   ⚠️  Extension not connected"
    echo "   Make sure Chrome extension is loaded and tracking is enabled"
    exit 1
fi

# Test 1: Call observe waterfall and check for pending query
echo ""
echo "3. Testing on-demand waterfall fetch..."
echo "   Sending MCP request: observe({what: 'network_waterfall'})"

# Use MCP HTTP endpoint
RESULT=$(curl -s -X POST "$SERVER_URL/mcp" \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": "observe",
            "arguments": {"what": "network_waterfall"}
        }
    }')

# Check for entries
if echo "$RESULT" | grep -q '"entries"'; then
    ENTRY_COUNT=$(echo "$RESULT" | grep -o '"count":[0-9]*' | cut -d: -f2 | head -1)
    echo "   ✅ Received waterfall data: $ENTRY_COUNT entries"

    # Show sample entry
    echo ""
    echo "4. Sample entry:"
    echo "$RESULT" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    content = data.get('result', {}).get('content', [{}])[0].get('text', '')
    # Find JSON in text
    start = content.find('{')
    if start >= 0:
        parsed = json.loads(content[start:])
        entries = parsed.get('entries', [])
        if entries:
            print(json.dumps(entries[0], indent=2))
        else:
            print('   (No entries - try loading a page with network activity)')
except:
    print('   (Could not parse response)')
" 2>/dev/null || echo "   (Parse failed)"
else
    echo "   ⚠️  No entries in response"
    echo "   Response: ${RESULT:0:200}..."
fi

echo ""
echo "=== UAT Complete ==="
echo ""
echo "To verify on-demand behavior:"
echo "1. Wait 2 seconds (data becomes stale)"
echo "2. Run this script again"
echo "3. Check extension logs for 'waterfall' query handling"
