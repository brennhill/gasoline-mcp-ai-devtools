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
VERSION_RAW="$(tr -d '[:space:]' < "$PROJECT_ROOT/VERSION" 2>/dev/null || true)"
VERSION_TAG="$(echo "$VERSION_RAW" | tr -cd '0-9')"
if [ -z "$VERSION_TAG" ]; then
    VERSION_TAG="dev"
fi
VERSIONED_BIN_NAME="gasoline-mcp-$VERSION_TAG"
VERSIONED_LOCAL_PATH="$PROJECT_ROOT/$VERSIONED_BIN_NAME"
VERSIONED_INSTALL_PATH="/usr/local/bin/$VERSIONED_BIN_NAME"

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

# Local versioned binary
if [ -f "$VERSIONED_LOCAL_PATH" ]; then
    rm -f "$VERSIONED_LOCAL_PATH"
    ok "Removed $VERSIONED_LOCAL_PATH"
fi

# System binary
if [ -f "/usr/local/bin/gasoline-mcp" ]; then
    rm -f "/usr/local/bin/gasoline-mcp"
    ok "Removed /usr/local/bin/gasoline-mcp"
fi

# Remove stale system versioned binaries
for stale_bin in /usr/local/bin/gasoline-mcp-[0-9]* /usr/local/bin/gasoline-mcp-dev; do
    if [ -e "$stale_bin" ]; then
        rm -f "$stale_bin"
        ok "Removed $stale_bin"
    fi
done

# ── Step 3: Compile TypeScript (if src/ changed) ─────────
if [ -d "src" ]; then
    src_newest_ts=$(find src -name '*.ts' -newer extension/background.js 2>/dev/null | head -1)
    if [ -n "$src_newest_ts" ]; then
        step "TypeScript sources changed, recompiling..."
        if make compile-ts >/dev/null 2>&1; then
            ok "TypeScript compiled"
        else
            warn "TypeScript compilation failed (non-fatal)"
        fi
    else
        ok "TypeScript up to date"
    fi
fi

# ── Step 4: Rebuild from source ──────────────────────────
step "Building from source..."
if ! go build -o gasoline-mcp "$CMD_PKG"; then
    err "Build failed!"
    exit 1
fi

cp ./gasoline-mcp "$VERSIONED_LOCAL_PATH"
ok "Created ./$VERSIONED_BIN_NAME"

# Verify the binary runs
build_version=$(./gasoline-mcp --version 2>&1 || true)
ok "Built ./gasoline-mcp — ${build_version}"

# ── Step 5: Install to PATH ─────────────────────────────
if [ "$INSTALL" = "true" ]; then
    step "Installing to /usr/local/bin..."
    # Symlink directly to the project binary — no copy needed on rebuild.
    ABSOLUTE_BIN="$(cd "$PROJECT_ROOT" && pwd)/$VERSIONED_BIN_NAME"
    ln -sfn "$ABSOLUTE_BIN" /usr/local/bin/gasoline-mcp
    ok "Symlinked /usr/local/bin/gasoline-mcp -> $ABSOLUTE_BIN"
fi

# ── Step 6: Verify ──────────────────────────────────────
step "Verifying..."
ok "Binary: $(which gasoline-mcp 2>/dev/null || echo './gasoline-mcp')"
ok "Points to: $(readlink /usr/local/bin/gasoline-mcp 2>/dev/null || echo 'direct binary')"

# Source vs binary timestamp check
src_newest=$(find "$CMD_DIR" -name '*.go' -newer ./gasoline-mcp 2>/dev/null | head -1)
if [ -n "$src_newest" ]; then
    warn "Source file newer than binary: $src_newest (this should not happen after fresh build)"
else
    ok "Binary is up to date with source"
fi

# ── Step 7: Restart daemon ──────────────────────────────
step "Restarting daemon..."
gasoline-mcp --stop 2>/dev/null || true
sleep 0.5
gasoline-mcp --daemon &
sleep 1
if pgrep -f "gasoline-mcp" >/dev/null 2>&1; then
    ok "Daemon running (PID $(pgrep -f 'gasoline-mcp' | head -1))"
else
    warn "Daemon may not have started — check logs"
fi

echo ""
echo -e "${G}Done.${X} Rebuilt, installed, and restarted."
