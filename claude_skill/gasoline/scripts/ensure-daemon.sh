#!/bin/bash
# ensure-daemon.sh — Ensure Gasoline daemon is running. Launches if needed.
GASOLINE_PORT="${GASOLINE_PORT:-7890}"
GASOLINE_URL="http://127.0.0.1:${GASOLINE_PORT}"

# Check if already running
if curl -s --max-time 1 "${GASOLINE_URL}/health" > /dev/null 2>&1; then
  echo '{"status":"already_running"}'
  curl -s --max-time 2 "${GASOLINE_URL}/health"
  exit 0
fi

# Find binary
GASOLINE_BIN=$(which gasoline 2>/dev/null || which gasoline-agentic-devtools 2>/dev/null)
if [ -z "$GASOLINE_BIN" ]; then
  echo '{"status":"error","message":"gasoline binary not found on PATH. Install: npm install -g gasoline-agentic-browser"}' >&2
  exit 1
fi

# Launch daemon in background
"$GASOLINE_BIN" --port "${GASOLINE_PORT}" > /dev/null 2>&1 &

# Wait for ready (up to 3s)
for i in $(seq 1 30); do
  if curl -s --max-time 0.5 "${GASOLINE_URL}/health" > /dev/null 2>&1; then
    echo '{"status":"started"}'
    curl -s --max-time 2 "${GASOLINE_URL}/health"
    exit 0
  fi
  sleep 0.1
done

echo '{"status":"error","message":"daemon failed to start within 3s"}' >&2
exit 1
