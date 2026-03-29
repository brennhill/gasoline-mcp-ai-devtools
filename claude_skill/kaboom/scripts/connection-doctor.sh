#!/bin/bash
# connection-doctor.sh — Diagnose Kaboom connectivity: daemon → extension → tracked tab
KABOOM_PORT="${KABOOM_PORT:-7890}"
KABOOM_URL="http://127.0.0.1:${KABOOM_PORT}"

echo "=== Kaboom Connection Doctor ==="

# 1. Daemon check
if ! curl -s --max-time 2 "${KABOOM_URL}/health" > /dev/null 2>&1; then
  echo "FAIL: Daemon not running on port ${KABOOM_PORT}"
  echo "Fix: bash scripts/ensure-daemon.sh"
  exit 1
fi
echo "OK: Daemon running"

# 2. Extension check
HEALTH=$(curl -s --max-time 2 "${KABOOM_URL}/health")
EXT_CONNECTED=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin).get('capture',{}).get('extension_connected',False))" 2>/dev/null)
if [ "$EXT_CONNECTED" != "True" ]; then
  echo "FAIL: Chrome extension not connected"
  echo "Fix: Install/enable Kaboom extension, open a web page"
  exit 1
fi
echo "OK: Extension connected"

# 3. Tracked tab check
TABS=$(curl -s --max-time 5 -X POST "${KABOOM_URL}/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"tabs"}}}')
TAB_TRACKED=$(echo "$TABS" | python3 -c "
import sys, json
try:
    r = json.load(sys.stdin)
    content = r.get('result',{}).get('content',[])
    for item in content:
        if item.get('type') == 'text':
            text = item['text']
            if 'tracked' in text.lower() and ('true' in text.lower() or '\"tracked\":true' in text.replace(' ','')):
                print('true')
                sys.exit(0)
    print('false')
except:
    print('false')
" 2>/dev/null)
if [ "$TAB_TRACKED" != "true" ]; then
  echo "WARN: No tab is being tracked"
  echo "Fix: Open a page in Chrome, click the Kaboom extension icon, and click 'Track This Tab'"
fi

echo ""
echo "$HEALTH" | python3 -c "import sys,json; h=json.load(sys.stdin); print(f'Version: {h.get(\"version\",\"?\")}'); print(f'Extension last seen: {h.get(\"capture\",{}).get(\"extension_last_seen\",\"?\")}')" 2>/dev/null
echo "Doctor complete."
