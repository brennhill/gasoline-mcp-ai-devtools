#!/bin/bash
# kaboom-call.sh — Call a Kaboom tool via HTTP JSON-RPC.
# Usage: kaboom-call.sh <tool_name> '<json_arguments>'
KABOOM_PORT="${KABOOM_PORT:-7890}"
KABOOM_URL="http://127.0.0.1:${KABOOM_PORT}"
TOOL="$1"
if [ -n "$2" ]; then
  ARGS="$2"
else
  ARGS='{}'
fi

if [ -z "$TOOL" ]; then
  echo '{"error":"Usage: kaboom-call.sh <tool_name> <json_arguments>"}' >&2
  exit 1
fi

RESPONSE=$(curl -s --max-time 30 -w '\n%{http_code}' -X POST "${KABOOM_URL}/mcp" \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"${TOOL}\",\"arguments\":${ARGS}}}" 2>/dev/null) || {
  echo '{"error":"Failed to connect to Kaboom daemon at '"${KABOOM_URL}"'. Is the server running?"}' >&2
  exit 1
}

# Split response body and HTTP status code
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
RESPONSE=$(echo "$RESPONSE" | sed '$d')

if [ -z "$RESPONSE" ] || [ "$HTTP_CODE" = "000" ]; then
  echo '{"error":"Daemon unreachable at '"${KABOOM_URL}"'. Run: bash scripts/ensure-daemon.sh"}' >&2
  exit 1
fi

if [ "$HTTP_CODE" -ge 400 ] 2>/dev/null; then
  echo "{\"error\":\"HTTP ${HTTP_CODE} from daemon\",\"body\":$(echo "$RESPONSE" | head -1)}" >&2
  exit 1
fi

# Extract the content from JSON-RPC response for cleaner output
echo "$RESPONSE" | python3 -c "
import sys, json
try:
    r = json.load(sys.stdin)
    if 'error' in r:
        print(json.dumps(r['error'], indent=2))
    elif 'result' in r:
        content = r['result'].get('content', [])
        for item in content:
            if item.get('type') == 'text':
                print(item['text'])
            elif item.get('type') == 'image':
                print(f'[image: {item.get(\"mimeType\",\"unknown\")}]')
    else:
        print(json.dumps(r, indent=2))
except:
    print(sys.stdin.read())
" 2>/dev/null || echo "$RESPONSE"
