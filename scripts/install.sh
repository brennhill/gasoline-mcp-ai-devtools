#!/bin/bash
# Gasoline - The Ultimate One-liner Installer
# https://github.com/brennhill/gasoline-mcp-ai-devtools
#
# PURPOSE:
# This script provides a zero-dependency, platform-aware installation flow for Gasoline.
# It handles binary acquisition, extension staging, and native configuration in one go.
#
# USAGE:
#   curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-mcp-ai-devtools/STABLE/scripts/install.sh | bash

# Fail immediately if a command fails (-e), an unset variable is used (-u),
# or a command in a pipeline fails (-o pipefail). This is critical for installer safety.
set -euo pipefail

# Configuration: Define the single source of truth for paths and repository metadata.
REPO="brennhill/gasoline-mcp-ai-devtools"
INSTALL_DIR="$HOME/.gasoline"
BIN_DIR="$INSTALL_DIR/bin"
EXT_DIR="$INSTALL_DIR/extension"
# The VERSION file on the STABLE branch is the source of truth for the latest release.
VERSION_URL="https://raw.githubusercontent.com/$REPO/STABLE/VERSION"

# UI: Define colors for high-visibility terminal output.
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
ORANGE='\033[38;5;208m'
BOLD='\033[1m'
NC='\033[0m' # No Color (Reset)

# Cleanup: Ensure temporary files are removed even if the script crashes or is interrupted.
# Uses mktemp to prevent predictable filename attacks.
TEMP_ROOT=$(mktemp -d)
cleanup() {
    rm -rf "$TEMP_ROOT"
}
trap cleanup EXIT

echo -e "${ORANGE}${BOLD}"
cat <<'EOF'
   ____                 _ _            
  / ___| __ _ ___  ___ | (_)_ __   ___ 
 | |  _ / _` / __|/ _ \| | | '_ \ / _ \
 | |_| | (_| \__ \ (_) | | | | | |  __/
  \____|\__,_|___/\___/|_|_|_| |_|\___|
EOF
echo -e "${NC}"
echo -e "${ORANGE}${BOLD}🔥 Gasoline Installer${NC}"
echo -e "${BLUE}--------------------------------------------------${NC}"
reset_extension_dir() {
    rm -rf "$EXT_DIR"
    mkdir -p "$EXT_DIR"
}

validate_extension_stage() {
    [ -f "$EXT_DIR/manifest.json" ] &&
    [ -f "$EXT_DIR/background/init.js" ] &&
    [ -f "$EXT_DIR/content/script-injection.js" ] &&
    [ -f "$EXT_DIR/inject/index.js" ]
}

# 1. Platform Detection: Identify the OS and CPU architecture to download the correct binary.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin)  PLATFORM="darwin" ;; # macOS
  linux)   PLATFORM="linux" ;;  # Linux
  mingw*|cygwin*) PLATFORM="win32" ;; # Windows (Git Bash/Cygwin)
  *) echo -e "${RED}❌ Unsupported OS: $OS${NC}"; exit 1 ;;
esac

# Normalize architecture strings to match release asset naming conventions (x64 vs arm64).
case "$ARCH" in
  x86_64|amd64) E_ARCH="x64" ;;
  arm64|aarch64) E_ARCH="arm64" ;;
  *) echo -e "${RED}❌ Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

# Windows-specific binary suffix and architecture enforcement.
if [ "$PLATFORM" == "win32" ]; then
    E_ARCH="x64"
    BINARY_EXT=".exe"
else
    BINARY_EXT=""
fi

# 2. Version Check: Fetch the latest stable version number from GitHub.
echo -e "🔍 Checking for updates..."
VERSION=$(curl -sSL --fail "$VERSION_URL" | tr -d '[:space:]' || true)
if [ -z "$VERSION" ]; then
    echo -e "${RED}❌ Failed to fetch latest version info.${NC}"
    exit 1
fi
echo -e "✨ Version: v$VERSION ($PLATFORM-$E_ARCH)"

# 3. Directory Setup: Ensure the installation folders exist.
mkdir -p "$BIN_DIR"
reset_extension_dir
echo -e "📁 Install root: $INSTALL_DIR"

# 4. Binary Installation: Download the pre-compiled Go binary from GitHub Releases.
GASOLINE_BIN="$BIN_DIR/gasoline$BINARY_EXT"
BINARY_NAME="gasoline-$PLATFORM-$E_ARCH$BINARY_EXT"
BINARY_URL="https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
CHECKSUM_URL="https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"

echo -e "⬇️  Downloading latest binary..."
# Download to a temporary location first to ensure an atomic installation.
if ! curl -fsSL "$BINARY_URL" -o "$TEMP_ROOT/gasoline_dl"; then
    echo -e "${RED}❌ Download failed.${NC}"
    exit 1
fi

# 5. Integrity Verification: Check the SHA-256 hash against the official manifest.
# This prevents man-in-the-middle attacks or corrupted downloads.
if curl -fsSL "$CHECKSUM_URL" -o "$TEMP_ROOT/checksums.txt" 2>/dev/null; then
    EXPECTED_HASH=$(grep "$BINARY_NAME" "$TEMP_ROOT/checksums.txt" | awk '{print $1}' || true)
    if [ -n "$EXPECTED_HASH" ]; then
        if command -v shasum >/dev/null 2>&1; then
            ACTUAL_HASH=$(shasum -a 256 "$TEMP_ROOT/gasoline_dl" | awk '{print $1}')
        else
            ACTUAL_HASH=$(sha256sum "$TEMP_ROOT/gasoline_dl" | awk '{print $1}')
        fi
        if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
            echo -e "${RED}❌ Checksum verification failed! The binary may be corrupted or tampered with.${NC}"
            exit 1
        fi
    fi
fi

# Move the verified binary to its final path and set executable permissions.
mv "$TEMP_ROOT/gasoline_dl" "$GASOLINE_BIN"
chmod 755 "$GASOLINE_BIN"

# 6. Extension Staging: Download and extract the browser extension.
# We try the optimized extension-only zip first, falling back to the full source zip if needed.
echo -e "⬇️  Refreshing browser extension..."
EXT_ZIP_NAME="gasoline-extension-v$VERSION.zip"
EXT_ZIP_URL="https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
TEMP_ZIP="$TEMP_ROOT/extension.zip"

if curl -fsSL "$EXT_ZIP_URL" -o "$TEMP_ZIP"; then
    # Dedicated extension zip exists (faster); validate required module files after extract.
    reset_extension_dir
    if unzip -q -o "$TEMP_ZIP" -d "$EXT_DIR" && validate_extension_stage; then
        echo -e "✅ Staged extension directory: $EXT_DIR"
    else
        echo -e "${YELLOW}⚠️  Release extension zip missing required modules; falling back to source zip...${NC}"
        SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/tags/v$VERSION.zip"
        TEMP_EXTRACT="$TEMP_ROOT/ext_extract"
        mkdir -p "$TEMP_EXTRACT"
        if curl -fsSL "$SOURCE_ZIP_URL" -o "$TEMP_ZIP"; then
            reset_extension_dir
            unzip -q "$TEMP_ZIP" -d "$TEMP_EXTRACT"
            # The source zip root folder is typically 'repo-version'.
            EXTRACT_ROOT=$(ls -d "$TEMP_EXTRACT"/* | head -n 1)
            cp -r "$EXTRACT_ROOT/extension/"* "$EXT_DIR/"
            if ! validate_extension_stage; then
                echo -e "${RED}❌ Extension staging failed: required module files are missing.${NC}"
                exit 1
            fi
            echo -e "✅ Staged extension directory: $EXT_DIR"
        else
            echo -e "${RED}❌ Failed to download extension source archive.${NC}"
            exit 1
        fi
    fi
else
    # Fallback to source zip extraction (covers older releases and bad extension zip assets)
    SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/tags/v$VERSION.zip"
    TEMP_EXTRACT="$TEMP_ROOT/ext_extract"
    mkdir -p "$TEMP_EXTRACT"
    if curl -fsSL "$SOURCE_ZIP_URL" -o "$TEMP_ZIP"; then
        reset_extension_dir
        unzip -q "$TEMP_ZIP" -d "$TEMP_EXTRACT"
        # The source zip root folder is typically 'repo-version'.
        EXTRACT_ROOT=$(ls -d "$TEMP_EXTRACT"/* | head -n 1)
        cp -r "$EXTRACT_ROOT/extension/"* "$EXT_DIR/"
        if ! validate_extension_stage; then
            echo -e "${RED}❌ Extension staging failed: required module files are missing.${NC}"
            exit 1
        fi
        echo -e "✅ Staged extension directory: $EXT_DIR"
    else
        echo -e "${RED}❌ Failed to download extension source archive.${NC}"
        exit 1
    fi
fi

# 7. Native Configuration: Hand off the complex logic to the Go binary.
# The binary's --install flag handles:
#   - Killing stale Gasoline processes (resetting the state).
#   - Detecting 9+ different MCP clients (Claude, Cursor, Zed, etc.).
#   - Safely merging JSON configuration for each client.
#   - Displaying final success message and extension instructions.
echo -e "⚙️  Finalizing configuration..."
"$GASOLINE_BIN" --install
