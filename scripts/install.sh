#!/bin/bash
# Gasoline - The Ultimate One-liner Installer
# https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp
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
STAGE_EXT_DIR="$INSTALL_DIR/.extension-stage-$$"
BACKUP_EXT_DIR="$INSTALL_DIR/.extension-backup-$$"
# The VERSION file on the STABLE branch is the source of truth for the latest release.
VERSION_URL="https://raw.githubusercontent.com/$REPO/STABLE/VERSION"
STRICT_CHECKSUM="${GASOLINE_INSTALL_STRICT:-0}"

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
    rm -rf "$STAGE_EXT_DIR" "$BACKUP_EXT_DIR"
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
if [ "$STRICT_CHECKSUM" = "1" ]; then
    echo -e "🔒 Strict checksum mode enabled (GASOLINE_INSTALL_STRICT=1)"
fi

prepare_extension_stage() {
    rm -rf "$STAGE_EXT_DIR"
    mkdir -p "$STAGE_EXT_DIR"
}

validate_extension_stage() {
    local base_dir="${1:-$EXT_DIR}"
    [ -f "$base_dir/manifest.json" ] &&
    [ -f "$base_dir/background/init.js" ] &&
    [ -f "$base_dir/content/script-injection.js" ] &&
    [ -f "$base_dir/inject/index.js" ] &&
    [ -f "$base_dir/theme-bootstrap.js" ]
}

promote_extension_stage() {
    if ! validate_extension_stage "$STAGE_EXT_DIR"; then
        echo -e "${RED}❌ Extension staging failed: required module files are missing from staging.${NC}"
        exit 1
    fi

    rm -rf "$BACKUP_EXT_DIR"
    if [ -d "$EXT_DIR" ]; then
        mv "$EXT_DIR" "$BACKUP_EXT_DIR"
    fi

    if ! mv "$STAGE_EXT_DIR" "$EXT_DIR"; then
        echo -e "${RED}❌ Failed to promote staged extension directory.${NC}"
        if [ -d "$BACKUP_EXT_DIR" ]; then
            mv "$BACKUP_EXT_DIR" "$EXT_DIR" || true
        fi
        exit 1
    fi

    if ! validate_extension_stage "$EXT_DIR"; then
        echo -e "${RED}❌ Promoted extension failed validation; restoring previous extension.${NC}"
        rm -rf "$EXT_DIR"
        if [ -d "$BACKUP_EXT_DIR" ]; then
            mv "$BACKUP_EXT_DIR" "$EXT_DIR" || true
        fi
        exit 1
    fi

    rm -rf "$BACKUP_EXT_DIR"
}

stage_extension_from_source_zip() {
    local source_zip_url="$1"
    local temp_extract="$TEMP_ROOT/ext_extract"

    rm -rf "$temp_extract"
    mkdir -p "$temp_extract"

    if ! curl -fsSL "$source_zip_url" -o "$TEMP_ZIP"; then
        return 1
    fi

    prepare_extension_stage
    if ! unzip -q "$TEMP_ZIP" -d "$temp_extract"; then
        return 1
    fi

    # The source zip root folder is typically 'repo-branch'.
    local extract_root
    extract_root=$(find "$temp_extract" -mindepth 1 -maxdepth 1 -type d | head -n 1)
    if [ -z "$extract_root" ] || [ ! -d "$extract_root/extension" ]; then
        return 1
    fi

    if ! cp -r "$extract_root/extension/." "$STAGE_EXT_DIR/"; then
        return 1
    fi
    if ! validate_extension_stage "$STAGE_EXT_DIR"; then
        return 1
    fi
    return 0
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
mkdir -p "$INSTALL_DIR"
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
CHECKSUM_VERIFIED=0
if curl -fsSL "$CHECKSUM_URL" -o "$TEMP_ROOT/checksums.txt" 2>/dev/null; then
    EXPECTED_HASH=$(grep "$BINARY_NAME" "$TEMP_ROOT/checksums.txt" | awk '{print $1}' || true)
    if [ -z "$EXPECTED_HASH" ]; then
        if [ "$STRICT_CHECKSUM" = "1" ]; then
            echo -e "${RED}❌ Strict checksum mode: checksums.txt missing entry for $BINARY_NAME.${NC}"
            exit 1
        fi
        echo -e "${YELLOW}⚠️  checksums.txt did not contain $BINARY_NAME; continuing without checksum verification.${NC}"
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL_HASH=$(shasum -a 256 "$TEMP_ROOT/gasoline_dl" | awk '{print $1}')
    elif command -v sha256sum >/dev/null 2>&1; then
        ACTUAL_HASH=$(sha256sum "$TEMP_ROOT/gasoline_dl" | awk '{print $1}')
    else
        if [ "$STRICT_CHECKSUM" = "1" ]; then
            echo -e "${RED}❌ Strict checksum mode: no SHA-256 tool found (need shasum or sha256sum).${NC}"
            exit 1
        fi
        echo -e "${YELLOW}⚠️  No SHA-256 tool found (shasum/sha256sum); continuing without checksum verification.${NC}"
        ACTUAL_HASH=""
    fi

    if [ -n "${ACTUAL_HASH:-}" ]; then
        if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
            echo -e "${RED}❌ Checksum verification failed! The binary may be corrupted or tampered with.${NC}"
            exit 1
        fi
        CHECKSUM_VERIFIED=1
        echo -e "✅ Checksum verified."
    fi
else
    if [ "$STRICT_CHECKSUM" = "1" ]; then
        echo -e "${RED}❌ Strict checksum mode: failed to download checksum manifest.${NC}"
        exit 1
    fi
    echo -e "${YELLOW}⚠️  Checksum verification skipped (could not fetch manifest).${NC}"
fi

if [ "$STRICT_CHECKSUM" = "1" ] && [ "$CHECKSUM_VERIFIED" -ne 1 ]; then
    echo -e "${RED}❌ Strict checksum mode: verification did not complete successfully.${NC}"
    exit 1
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
    prepare_extension_stage
    if unzip -q -o "$TEMP_ZIP" -d "$STAGE_EXT_DIR" && validate_extension_stage "$STAGE_EXT_DIR"; then
        promote_extension_stage
        echo -e "✅ Staged extension directory: $EXT_DIR"
    else
        echo -e "${YELLOW}⚠️  Release extension zip missing required modules; falling back to source zip...${NC}"
        SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/heads/STABLE.zip"
        if stage_extension_from_source_zip "$SOURCE_ZIP_URL"; then
            promote_extension_stage
            echo -e "✅ Staged extension directory: $EXT_DIR"
        else
            echo -e "${RED}❌ Failed to download extension source archive.${NC}"
            exit 1
        fi
    fi
else
    # Fallback to source zip extraction (covers older releases and bad extension zip assets)
    SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/heads/STABLE.zip"
    if stage_extension_from_source_zip "$SOURCE_ZIP_URL"; then
        promote_extension_stage
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
