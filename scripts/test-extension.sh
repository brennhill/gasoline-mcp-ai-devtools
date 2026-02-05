#!/bin/bash
# Test Gasoline extension connection and MCP tools
# Run this script directly in your terminal (not through Claude Code)

set -e

cd "$(dirname "$0")/.."

echo "=== Gasoline Extension Test ==="
echo ""

# 1. Build server if needed
if [ ! -f "./gasoline-mcp" ]; then
    echo "Building server..."
    go build -o gasoline-mcp ./cmd/dev-console/
fi

# 2. Compile TypeScript if needed
echo "Compiling TypeScript..."
make compile-ts

# 3. Kill any existing server
echo "Cleaning up existing servers..."
pkill -f "gasoline-mcp" 2>/dev/null || true
sleep 1

# 4. Start server (keep stdin open to prevent early exit)
echo "Starting server..."
(sleep 999999 | ./gasoline-mcp) &
SERVER_PID=$!
sleep 3

# 5. Test health endpoint
echo ""
echo "=== Health Check ==="
if curl -s http://127.0.0.1:7890/health | jq .; then
    echo "✅ Server is healthy"
else
    echo "❌ Server health check failed"
    exit 1
fi

# 6. Test MCP tools/list
echo ""
echo "=== MCP Tools ==="
TOOLS=$(curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
    http://127.0.0.1:7890/mcp)
echo "$TOOLS" | jq -r '.result.tools[].name' 2>/dev/null || echo "❌ Failed to list tools"

# 7. Test observe tool
echo ""
echo "=== Test observe (logs) ==="
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"logs","limit":5}}}' \
    http://127.0.0.1:7890/mcp | jq '.result.content[0].text | fromjson | .logs | length' 2>/dev/null && echo "logs returned" || echo "❌ observe failed"

# 8. Test extension connection (check /sync endpoint)
echo ""
echo "=== Extension Connection ==="
SYNC=$(curl -s http://127.0.0.1:7890/sync 2>/dev/null)
if echo "$SYNC" | jq -e '.pending_queries' > /dev/null 2>&1; then
    echo "✅ /sync endpoint working"
else
    echo "⚠️  Extension may not be connected yet"
    echo "   Load extension from: chrome://extensions -> Load unpacked -> select 'extension/' folder"
fi

echo ""
echo "=== Server Running ==="
echo "Server PID: $SERVER_PID"
echo "Stop with: kill $SERVER_PID"
echo ""
echo "To test with Chrome:"
echo "1. Open chrome://extensions"
echo "2. Enable Developer mode"
echo "3. Click 'Load unpacked' and select: $(pwd)/extension"
echo "4. Open any webpage and check console for 'Gasoline' messages"
