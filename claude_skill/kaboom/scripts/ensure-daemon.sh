#!/bin/bash
# ensure-daemon.sh — Ensure Kaboom daemon is running. Launches if needed.
KABOOM_PORT="${KABOOM_PORT:-7890}"
KABOOM_URL="http://127.0.0.1:${KABOOM_PORT}"

# Check if already running
if curl -s --max-time 1 "${KABOOM_URL}/health" > /dev/null 2>&1; then
  echo '{"status":"already_running"}'
  curl -s --max-time 2 "${KABOOM_URL}/health"
  exit 0
fi

# Find binary
KABOOM_BIN=$(which kaboom 2>/dev/null || which kaboom-agentic-browser 2>/dev/null)
if [ -z "$KABOOM_BIN" ]; then
  echo '{"status":"error","message":"kaboom binary not found on PATH. Install: npm install -g kaboom-agentic-browser"}' >&2
  exit 1
fi

# Launch daemon in background
"$KABOOM_BIN" --port "${KABOOM_PORT}" > /dev/null 2>&1 &

# Wait for ready (up to 3s)
for i in $(seq 1 30); do
  if curl -s --max-time 0.5 "${KABOOM_URL}/health" > /dev/null 2>&1; then
    echo '{"status":"started"}'
    curl -s --max-time 2 "${KABOOM_URL}/health"
    exit 0
  fi
  sleep 0.1
done

echo '{"status":"error","message":"daemon failed to start within 3s"}' >&2
exit 1
