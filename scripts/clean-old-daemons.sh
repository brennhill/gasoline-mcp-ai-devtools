#!/bin/bash
# Clean up old gasoline daemons before upgrading
# Usage: ./scripts/clean-old-daemons.sh
# Or: gasoline --force

set -e

echo "🧹 Gasoline Daemon Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

KILLED=0
FAILED=0

# Function to kill a process safely
kill_process() {
  local pid=$1
  local name=$2

  if kill -0 "$pid" 2>/dev/null; then
    echo "  → Killing $name (PID $pid)"
    if kill -TERM "$pid" 2>/dev/null; then
      # Wait for graceful exit
      local waited=0
      while kill -0 "$pid" 2>/dev/null && [ $waited -lt 10 ]; do
        sleep 0.1
        ((waited++))
      done
      # If still running, force kill
      if kill -0 "$pid" 2>/dev/null; then
        kill -9 "$pid" 2>/dev/null || true
      fi
      ((KILLED++))
    fi
  fi
}

# Platform-specific cleanup
if [[ "$OSTYPE" == "darwin"* ]]; then
  # macOS: use lsof and pkill
  echo "Platform: macOS"
  echo ""
  echo "Searching for gasoline processes..."

  # Get all gasoline processes
  PIDS=$(lsof -c gasoline -a -d cwd 2>/dev/null | tail -n +2 | awk '{print $2}' | sort -u || true)

  if [ -z "$PIDS" ]; then
    echo "  No gasoline processes found"
  else
    for pid in $PIDS; do
      kill_process "$pid" "gasoline"
    done
  fi

  # Also try pkill as fallback
  pkill -9 -f "gasoline.*--daemon" 2>/dev/null || true

elif [[ "$OSTYPE" == "linux"* ]]; then
  # Linux: use pgrep/pkill
  echo "Platform: Linux"
  echo ""
  echo "Searching for gasoline processes..."

  PIDS=$(pgrep -f "gasoline.*--daemon" || true)

  if [ -z "$PIDS" ]; then
    echo "  No gasoline processes found"
  else
    for pid in $PIDS; do
      kill_process "$pid" "gasoline"
    done
  fi

elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
  # Windows
  echo "Platform: Windows"
  echo ""
  echo "Searching for gasoline processes..."

  taskkill /F /IM gasoline.exe 2>/dev/null || echo "  No gasoline.exe processes found"
  ((KILLED++))
fi

# Clean up PID files
echo ""
echo "Cleaning up PID files..."
for port in {7890..7910}; do
  pid_file="$HOME/.gasoline-$port.pid"
  if [ -f "$pid_file" ]; then
    rm -f "$pid_file"
    echo "  Removed $pid_file"
  fi
done

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$KILLED" -gt 0 ]; then
  echo "✓ Killed $KILLED gasoline process(es)"
else
  echo "✓ No running gasoline processes found"
fi
echo ""
echo "Safe to install or upgrade gasoline now:"
echo "  npm install -g gasoline-mcp@latest"
echo ""
