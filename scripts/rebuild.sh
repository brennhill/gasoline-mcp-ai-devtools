#!/bin/bash
# rebuild.sh — Kill all daemons, remove stale binaries, rebuild from source, install.
# Usage: ./scripts/rebuild.sh [--no-install]
#   --no-install  Skip copying to /usr/local/bin (local ./gasoline-mcp only)
set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
cd "$PROJECT_ROOT"
CMD_PKG="${GASOLINE_CMD_PKG:-./cmd/dev-console}"
CMD_DIR="${CMD_PKG#./}"

# Colors (if TTY)
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    G='\033[0;32m'; R='\033[0;31m'; Y='\033[0;33m'; B='\033[0;34m'; X='\033[0m'
else
    G=''; R=''; Y=''; B=''; X=''
fi

step() { echo -e "${B}▸${X} $1"; }
ok()   { echo -e "  ${G}✓${X} $1"; }
warn() { echo -e "  ${Y}⚠${X} $1"; }
err()  { echo -e "  ${R}✗${X} $1"; }

INSTALL=true
[ "${1:-}" = "--no-install" ] && INSTALL=false

# ── Step 1: Kill all running daemons ─────────────────────
step "Killing all gasoline-mcp processes..."
killed=0

# Kill by process name
if pgrep -f "gasoline-mcp" >/dev/null 2>&1; then
    pids=$(pgrep -f "gasoline-mcp" | tr '\n' ' ')
    kill $pids 2>/dev/null || true
    sleep 0.3
    # Force-kill stragglers
    if pgrep -f "gasoline-mcp" >/dev/null 2>&1; then
        kill -9 $(pgrep -f "gasoline-mcp") 2>/dev/null || true
    fi
    killed=1
fi

# Kill anything on port 7890
if lsof -ti :7890 >/dev/null 2>&1; then
    lsof -ti :7890 | xargs kill -9 2>/dev/null || true
    killed=1
fi

if [ "$killed" = "1" ]; then
    ok "All gasoline-mcp processes killed"
else
    ok "No running processes found"
fi

# ── Step 2: Remove stale binaries ────────────────────────
step "Removing stale binaries..."

# Local project binary
if [ -f "./gasoline-mcp" ]; then
    rm -f "./gasoline-mcp"
    ok "Removed ./gasoline-mcp"
fi

# System binary
if [ -f "/usr/local/bin/gasoline-mcp" ]; then
    rm -f "/usr/local/bin/gasoline-mcp"
    ok "Removed /usr/local/bin/gasoline-mcp"
fi

# ── Step 3: Rebuild from source ──────────────────────────
step "Building from source..."
if ! go build -o gasoline-mcp "$CMD_PKG"; then
    err "Build failed!"
    exit 1
fi

# Verify the binary runs
build_version=$(./gasoline-mcp --version 2>&1 || true)
ok "Built ./gasoline-mcp — ${build_version}"

# ── Step 4: Install to PATH ─────────────────────────────
if [ "$INSTALL" = "true" ]; then
    step "Installing to /usr/local/bin..."
    cp ./gasoline-mcp /usr/local/bin/gasoline-mcp
    ok "Installed to /usr/local/bin/gasoline-mcp"
fi

# ── Step 5: Verify single binary ────────────────────────
step "Verifying..."
locations=$(which -a gasoline-mcp 2>/dev/null || true)
count=$(echo "$locations" | grep -c "gasoline-mcp" || true)

if [ "$count" -gt 1 ] && [ "$INSTALL" = "true" ]; then
    warn "Multiple binaries in PATH:"
    echo "$locations" | while read -r loc; do
        echo "    $loc"
    done
else
    ok "Single binary: $(which gasoline-mcp 2>/dev/null || echo './gasoline-mcp')"
fi

# Source vs binary timestamp check
src_newest=$(find "$CMD_DIR" -name '*.go' -newer ./gasoline-mcp 2>/dev/null | head -1)
if [ -n "$src_newest" ]; then
    warn "Source file newer than binary: $src_newest (this should not happen after fresh build)"
else
    ok "Binary is up to date with source"
fi

echo ""
echo -e "${G}Done.${X} Binary ready. Run smoke tests with:"
echo "  ./scripts/smoke-tests/framework-smoke.sh"
