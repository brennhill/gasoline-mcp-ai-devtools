#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "Checking bridge/wrapper stdout invariant..."

PATTERN='fmt\.Print(f|ln)?\(|fmt\.Fprintf\(os\.Stdout|fmt\.Fprint\(os\.Stdout|io\.WriteString\(os\.Stdout|os\.Stdout\.(Write|WriteString|Sync)\('

TARGET_FILES=(
  "cmd/dev-console/bridge.go"
  "cmd/dev-console/bridge_fastpath.go"
  "cmd/dev-console/bridge_forward.go"
  "cmd/dev-console/bridge_io_isolation.go"
  "cmd/dev-console/bridge_io_isolation_unix.go"
  "cmd/dev-console/bridge_io_isolation_windows.go"
  "cmd/dev-console/main_connection.go"
  "cmd/dev-console/main_connection_mcp.go"
  "cmd/dev-console/mcp_stdout.go"
  "cmd/dev-console/stdout_sync.go"
  "cmd/dev-console/stderr.go"
)

VIOLATIONS=0
for file in "${TARGET_FILES[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "Missing required file: $file"
    VIOLATIONS=1
    continue
  fi
  if rg -n "$PATTERN" "$file" >/tmp/gasoline-stdout-invariant.tmp 2>/dev/null; then
    # mcp_stdout.go is the only approved bridge transport writer.
    if [[ "$file" == "cmd/dev-console/mcp_stdout.go" ]]; then
      continue
    fi
    echo "INVARIANT VIOLATION: direct stdout write in $file"
    cat /tmp/gasoline-stdout-invariant.tmp
    VIOLATIONS=1
  fi
done
rm -f /tmp/gasoline-stdout-invariant.tmp

if ! rg -n 'ensureBridgeIOIsolation\(cfg\.logFile\)' cmd/dev-console/main.go >/dev/null 2>&1; then
  echo "INVARIANT VIOLATION: bridge mode must initialize IO isolation in main.go"
  VIOLATIONS=1
fi

if ! rg -n 'sendStartupError\(\"Bridge stdio isolation failed:' cmd/dev-console/main.go >/dev/null 2>&1; then
  echo "INVARIANT VIOLATION: bridge isolation failures must be surfaced as JSON-RPC startup errors"
  VIOLATIONS=1
fi

if ! rg -n 'syscall\.CloseOnExec\(fd\)' cmd/dev-console/bridge_io_isolation_unix.go >/dev/null 2>&1; then
  echo "INVARIANT VIOLATION: duplicated MCP transport fd must be marked close-on-exec"
  VIOLATIONS=1
fi

if [[ $VIOLATIONS -ne 0 ]]; then
  echo "❌ Bridge/wrapper stdout invariant failed"
  exit 1
fi

echo "✅ Bridge/wrapper stdout invariant OK"

