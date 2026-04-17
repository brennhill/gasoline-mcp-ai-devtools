#!/bin/bash
# Clean up old Kaboom and legacy daemons before upgrading
# Usage: ./scripts/clean-old-daemons.sh
# Or: kaboom --force

set -euo pipefail

echo "🧹 Kaboom Daemon Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

KILLED=0
_FAILED=0  # reserved for future error reporting

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
  echo "Searching for Kaboom/legacy processes..."

  # Get all Kaboom/legacy processes
  PIDS=$(
    for legacy_name in kaboom gasoline strum; do
      lsof -c "$legacy_name" -a -d cwd 2>/dev/null | tail -n +2 | awk '{print $2}'
    done | sort -u || true
  )

  if [ -z "$PIDS" ]; then
    echo "  No Kaboom/legacy processes found"
  else
    for pid in $PIDS; do
      kill_process "$pid" "Kaboom/legacy process"
    done
  fi

  # Also try pkill as fallback
  pkill -9 -f "kaboom.*--daemon" 2>/dev/null || true
  pkill -9 -f "gasoline.*--daemon" 2>/dev/null || true
  pkill -9 -f "strum.*--daemon" 2>/dev/null || true

elif [[ "$OSTYPE" == "linux"* ]]; then
  # Linux: use pgrep/pkill
  echo "Platform: Linux"
  echo ""
  echo "Searching for Kaboom/legacy processes..."

  PIDS=$(
    {
      pgrep -f "kaboom.*--daemon"
      pgrep -f "gasoline.*--daemon"
      pgrep -f "strum.*--daemon"
    } 2>/dev/null | sort -u || true
  )

  if [ -z "$PIDS" ]; then
    echo "  No Kaboom/legacy processes found"
  else
    for pid in $PIDS; do
      kill_process "$pid" "Kaboom/legacy process"
    done
  fi

elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
  # Windows
  echo "Platform: Windows"
  echo ""
  echo "Searching for Kaboom/legacy processes..."

  for legacy_image in kaboom.exe gasoline.exe strum.exe; do
    if taskkill /F /IM "$legacy_image" 2>/dev/null; then
      ((KILLED++))
    else
      echo "  No $legacy_image processes found"
    fi
  done
fi

# Clean up PID files
echo ""
echo "Cleaning up PID files..."
for legacy_name in kaboom gasoline strum; do
  for port in {7890..7910}; do
    pid_file="$HOME/.${legacy_name}-$port.pid"
    if [ -f "$pid_file" ]; then
      rm -f "$pid_file"
      echo "  Removed $pid_file"
    fi
  done
done

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$KILLED" -gt 0 ]; then
  echo "✓ Killed $KILLED Kaboom/legacy process(es)"
  else
  echo "✓ No running Kaboom/legacy processes found"
  fi

  echo "Safe to install or upgrade Kaboom now:"
  echo "  npm install -g kaboom-agentic-browser@latest"
echo ""
