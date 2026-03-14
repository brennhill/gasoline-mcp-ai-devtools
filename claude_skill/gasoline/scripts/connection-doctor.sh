#!/bin/bash
# connection-doctor.sh — Diagnose Gasoline connectivity: daemon → extension → tracked tab
GASOLINE_PORT="${GASOLINE_PORT:-7890}"
GASOLINE_URL="http://127.0.0.1:${GASOLINE_PORT}"

echo "=== Gasoline Connection Doctor ==="

# 1. Daemon check
if ! curl -s --max-time 2 "${GASOLINE_URL}/health" > /dev/null 2>&1; then
  echo "FAIL: Daemon not running on port ${GASOLINE_PORT}"
  echo "Fix: bash scripts/ensure-daemon.sh"
  exit 1
fi
echo "OK: Daemon running"

# 2. Extension check
HEALTH=$(curl -s --max-time 2 "${GASOLINE_URL}/health")
EXT_CONNECTED=$(echo "$HEALTH" | python3 -c "import sys,json; print(json.load(sys.stdin).get('capture',{}).get('extension_connected',False))" 2>/dev/null)
if [ "$EXT_CONNECTED" != "True" ]; then
  echo "FAIL: Chrome extension not connected"
  echo "Fix: Install/enable Gasoline extension, open a web page"
  exit 1
fi
echo "OK: Extension connected"

# 3. Tracked tab check
TABS=$(curl -s --max-time 5 -X POST "${GASOLINE_URL}/mcp" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"tabs"}}}')
echo "OK: Connection healthy"
echo "$HEALTH" | python3 -c "import sys,json; h=json.load(sys.stdin); print(f'Version: {h.get(\"version\",\"?\")}'); print(f'Extension last seen: {h.get(\"capture\",{}).get(\"extension_last_seen\",\"?\")}')" 2>/dev/null
