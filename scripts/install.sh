#!/bin/bash
# Gasoline - The Ultimate One-liner Installer
# https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp
#
# PURPOSE:
# This script provides a zero-dependency, platform-aware installation flow for Gasoline.
# It handles binary acquisition, extension staging, and native configuration in one go.
#
# USAGE:
#   curl -sSL https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.sh | bash

# Fail immediately if a command fails (-e), an unset variable is used (-u),
# or a command in a pipeline fails (-o pipefail). This is critical for installer safety.
set -euo pipefail

# Configuration: Define the single source of truth for paths and repository metadata.
REPO="brennhill/gasoline-agentic-browser-devtools-mcp"
INSTALL_DIR="$HOME/.gasoline"
BIN_DIR="$INSTALL_DIR/bin"
EXT_DIR="${GASOLINE_EXTENSION_DIR:-$HOME/GasolineAgenticDevtoolExtension}"
STAGE_EXT_DIR="$INSTALL_DIR/.extension-stage-$$"
BACKUP_EXT_DIR="$INSTALL_DIR/.extension-backup-$$"
# The VERSION file on the STABLE branch is the source of truth for the latest release.
VERSION_URL="https://raw.githubusercontent.com/$REPO/STABLE/VERSION"
STRICT_CHECKSUM="${GASOLINE_INSTALL_STRICT:-0}"
# Minimum plausible binary size (5 MB). Catches truncated downloads and HTML error pages.
MIN_BINARY_BYTES=5000000

# UI: Define colors for high-visibility terminal output.
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
ORANGE='\033[38;5;208m'
BOLD='\033[1m'
NC='\033[0m' # No Color (Reset)

# Anonymous install error beacon (disable: STRUM_TELEMETRY=off).
# Fire-and-forget, never blocks, never fails the install.
beacon_error() {
    local step="${1:-unknown}"
    if [ "${STRUM_TELEMETRY:-}" = "off" ]; then return; fi
    curl -s --max-time 2 -X POST "https://t.getstrum.dev/v1/event" \
        -H "Content-Type: application/json" \
        -d "{\"event\":\"install_error\",\"v\":\"${VERSION:-unknown}\",\"os\":\"$(uname -s)-$(uname -m)\",\"props\":{\"step\":\"${step}\",\"method\":\"curl\"}}" \
        > /dev/null 2>&1 || true
}

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
  ____ _____ ____  _   _ __  __ 
 / ___|_   _|  _ \| | | |  \/  |
 \___ \ | | | |_) | | | | |\/| |
  ___) || | |  _ <| |_| | |  | |
 |____/ |_| |_| \_\\___/|_|  |_|
EOF
echo -e "${NC}"
echo -e "${ORANGE}${BOLD}STRUM Installer${NC}"
echo -e "${BLUE}--------------------------------------------------${NC}"
if [ "$STRICT_CHECKSUM" = "1" ]; then
    echo -e "Strict checksum mode enabled (GASOLINE_INSTALL_STRICT=1)"
fi

# ─────────────────────────────────────────────────────────────
# Prerequisite Checks
# ─────────────────────────────────────────────────────────────

check_prerequisites() {
    local missing=""

    if ! command -v curl >/dev/null 2>&1; then
        missing="${missing}  - curl (required for downloads)\n"
    fi
    if ! command -v unzip >/dev/null 2>&1; then
        missing="${missing}  - unzip (required for extension extraction)\n"
    fi

    if [ -n "$missing" ]; then
        echo -e "${RED}Missing required tools:${NC}"
        echo -e "$missing"
        echo -e "Install them with your package manager and re-run."
        exit 1
    fi
}

check_disk_space() {
    # Need ~50 MB for binary + extension + temp files.
    local required_mb=50
    local available_mb=0

    if command -v df >/dev/null 2>&1; then
        # df -Pm gives POSIX-portable megabyte output; grab the mount containing $HOME.
        available_mb=$(df -Pm "$HOME" 2>/dev/null | awk 'NR==2 {print $4}' || echo "0")
    fi

    if [ "${available_mb:-0}" -gt 0 ] && [ "$available_mb" -lt "$required_mb" ]; then
        echo -e "${RED}Insufficient disk space: ${available_mb} MB available, need ${required_mb} MB.${NC}"
        echo -e "Free up space in $HOME and re-run."
        exit 1
    fi
}

check_network_connectivity() {
    # Quick connectivity probe — catch proxy/firewall/offline errors early
    # with a clear message instead of a raw curl failure later.
    if ! curl -fsSL --max-time 10 -o /dev/null "https://github.com" 2>/dev/null; then
        echo -e "${YELLOW}Cannot reach github.com — check your network connection or proxy settings.${NC}"
        echo -e "If you are behind a corporate proxy, set https_proxy before running."
        exit 1
    fi
}

check_prerequisites
check_disk_space
check_network_connectivity

# ─────────────────────────────────────────────────────────────
# Retry-capable download helper
# ─────────────────────────────────────────────────────────────

# curl_retry wraps curl with automatic retry for transient network errors.
# Usage: curl_retry <output_file> <url> [extra_curl_flags...]
# Retries up to 3 times with exponential backoff (2s, 4s, 8s).
curl_retry() {
    local output="$1"
    local url="$2"
    shift 2
    local max_attempts=3
    local attempt=1
    local delay=2

    while [ $attempt -le $max_attempts ]; do
        if curl -fsSL --connect-timeout 15 --max-time 120 "$@" "$url" -o "$output" 2>/dev/null; then
            return 0
        fi
        if [ $attempt -lt $max_attempts ]; then
            echo -e "${YELLOW}  Download attempt $attempt/$max_attempts failed; retrying in ${delay}s...${NC}"
            sleep $delay
            delay=$((delay * 2))
        fi
        attempt=$((attempt + 1))
    done
    return 1
}

# ─────────────────────────────────────────────────────────────
# Extension staging helpers
# ─────────────────────────────────────────────────────────────

prepare_extension_stage() {
    rm -rf "$STAGE_EXT_DIR"
    mkdir -p "$STAGE_EXT_DIR"
}

validate_extension_stage() {
    local base_dir="${1:-$EXT_DIR}"
    [ -f "$base_dir/manifest.json" ] || return 1

    # Support both modern bundled extension layout and legacy modular layout.
    local has_background=0
    local has_content=0
    local has_inject=0
    local has_bootstrap=0

    [ -f "$base_dir/background.js" ] || [ -f "$base_dir/background/init.js" ] && has_background=1
    [ -f "$base_dir/content.bundled.js" ] || [ -f "$base_dir/content/script-injection.js" ] && has_content=1
    [ -f "$base_dir/inject.bundled.js" ] || [ -f "$base_dir/inject/index.js" ] && has_inject=1
    [ -f "$base_dir/early-patch.bundled.js" ] || [ -f "$base_dir/theme-bootstrap.js" ] && has_bootstrap=1

    [ "$has_background" -eq 1 ] &&
    [ "$has_content" -eq 1 ] &&
    [ "$has_inject" -eq 1 ] &&
    [ "$has_bootstrap" -eq 1 ]
}

promote_extension_stage() {
    if ! validate_extension_stage "$STAGE_EXT_DIR"; then
        echo -e "${RED}Extension staging failed: required module files are missing from staging.${NC}"
        exit 1
    fi

    rm -rf "$BACKUP_EXT_DIR"
    if [ -d "$EXT_DIR" ]; then
        mv "$EXT_DIR" "$BACKUP_EXT_DIR"
    fi

    mkdir -p "$(dirname "$EXT_DIR")"

    if ! mv "$STAGE_EXT_DIR" "$EXT_DIR"; then
        echo -e "${RED}Failed to promote staged extension directory.${NC}"
        if [ -d "$BACKUP_EXT_DIR" ]; then
            mv "$BACKUP_EXT_DIR" "$EXT_DIR" || true
        fi
        exit 1
    fi

    if ! validate_extension_stage "$EXT_DIR"; then
        echo -e "${RED}Promoted extension failed validation; restoring previous extension.${NC}"
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

    if ! curl_retry "$TEMP_ZIP" "$source_zip_url"; then
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

sync_binary_compat_aliases() {
    local source_bin="$1"
    shift
    local target_bin=""
    local had_failure=0

    for target_bin in "$@"; do
        if [ -z "$target_bin" ] || [ "$target_bin" = "$source_bin" ]; then
            continue
        fi

        rm -f "$target_bin" || true
        if ln -s "$(basename "$source_bin")" "$target_bin" 2>/dev/null; then
            :
        elif cp "$source_bin" "$target_bin" 2>/dev/null; then
            :
        else
            had_failure=1
            echo -e "${YELLOW}  Could not create compatibility alias: $target_bin${NC}"
            continue
        fi
        chmod 755 "$target_bin" 2>/dev/null || true
    done

    return "$had_failure"
}

# ─────────────────────────────────────────────────────────────
# Stale process cleanup (pre-install)
# ─────────────────────────────────────────────────────────────

kill_stale_gasoline_processes() {
    # Kill any running gasoline daemons before replacing the binary.
    # This avoids "text file busy" on Linux and ensures a clean upgrade.
    local killed=0
    local pids=""

    if command -v pgrep >/dev/null 2>&1; then
        pids=$(pgrep -f 'gasoline-agentic-devtools|gasoline-agentic-browser|gasoline.*--daemon' 2>/dev/null || true)
    elif command -v pkill >/dev/null 2>&1; then
        # pgrep not available but pkill is — just send TERM directly.
        pkill -f 'gasoline-agentic-devtools|gasoline-agentic-browser' 2>/dev/null || true
        sleep 0.5
        pkill -9 -f 'gasoline-agentic-devtools|gasoline-agentic-browser' 2>/dev/null || true
        return 0
    fi

    if [ -n "$pids" ]; then
        echo -e "  Stopping running Gasoline processes..."
        for pid in $pids; do
            # Don't kill ourselves.
            if [ "$pid" != "$$" ]; then
                kill "$pid" 2>/dev/null && killed=$((killed + 1)) || true
            fi
        done

        if [ $killed -gt 0 ]; then
            sleep 1
            # Force-kill any survivors.
            for pid in $pids; do
                if [ "$pid" != "$$" ] && kill -0 "$pid" 2>/dev/null; then
                    kill -9 "$pid" 2>/dev/null || true
                fi
            done
        fi
    fi
}

# ─────────────────────────────────────────────────────────────
# 1. Platform Detection
# ─────────────────────────────────────────────────────────────

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  darwin)  PLATFORM="darwin" ;; # macOS
  linux)   PLATFORM="linux" ;;  # Linux
  mingw*|cygwin*) PLATFORM="win32" ;; # Windows (Git Bash/Cygwin)
  *) echo -e "${RED}Unsupported OS: $OS${NC}"; exit 1 ;;
esac

# Normalize architecture strings to match release asset naming conventions (x64 vs arm64).
case "$ARCH" in
  x86_64|amd64) E_ARCH="x64" ;;
  arm64|aarch64) E_ARCH="arm64" ;;
  *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

# Windows-specific binary suffix and architecture enforcement.
if [ "$PLATFORM" == "win32" ]; then
    E_ARCH="x64"
    BINARY_EXT=".exe"
else
    BINARY_EXT=""
fi

# ─────────────────────────────────────────────────────────────
# 2. Version Check
# ─────────────────────────────────────────────────────────────

echo -e "Checking for updates..."
VERSION=$(curl -sSL --fail --max-time 15 "$VERSION_URL" | tr -d '[:space:]' || true)
if [ -z "$VERSION" ]; then
    echo -e "${RED}Failed to fetch latest version info from $VERSION_URL${NC}"
    echo -e "Check your network connection and try again."
    exit 1
fi

# ─────────────────────────────────────────────────────────────
# 3. Detect install vs upgrade
# ─────────────────────────────────────────────────────────────

CANONICAL_GASOLINE_BIN="$BIN_DIR/gasoline-agentic-devtools$BINARY_EXT"
LEGACY_GASOLINE_BIN="$BIN_DIR/gasoline$BINARY_EXT"
LEGACY_GASOLINE_BROWSER_BIN="$BIN_DIR/gasoline-agentic-browser$BINARY_EXT"
IS_UPGRADE=0
PREVIOUS_VERSION=""

if [ -x "$CANONICAL_GASOLINE_BIN" ]; then
    PREVIOUS_VERSION=$("$CANONICAL_GASOLINE_BIN" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 || true)
    IS_UPGRADE=1
fi

if [ "$IS_UPGRADE" = "1" ] && [ -n "$PREVIOUS_VERSION" ]; then
    echo -e "Upgrading: v$PREVIOUS_VERSION -> v$VERSION ($PLATFORM-$E_ARCH)"
else
    echo -e "Installing: v$VERSION ($PLATFORM-$E_ARCH)"
fi

# ─────────────────────────────────────────────────────────────
# 4. Directory Setup
# ─────────────────────────────────────────────────────────────

mkdir -p "$BIN_DIR"
mkdir -p "$INSTALL_DIR"

# Verify the directory is writable (catches permission issues early).
if ! touch "$INSTALL_DIR/.write-test" 2>/dev/null; then
    echo -e "${RED}Cannot write to $INSTALL_DIR — check directory permissions.${NC}"
    echo -e "If this was installed with sudo previously, run: sudo chown -R \$USER $INSTALL_DIR"
    exit 1
fi
rm -f "$INSTALL_DIR/.write-test"

echo -e "Install root: $INSTALL_DIR"

# ─────────────────────────────────────────────────────────────
# 5. Stop stale processes before binary replacement
# ─────────────────────────────────────────────────────────────

kill_stale_gasoline_processes

# ─────────────────────────────────────────────────────────────
# 6. Binary Installation
# ─────────────────────────────────────────────────────────────

BINARY_NAME="gasoline-agentic-devtools-$PLATFORM-$E_ARCH$BINARY_EXT"
BINARY_URL="https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
CHECKSUM_URL="https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"

echo -e "Downloading binary..."
if ! curl_retry "$TEMP_ROOT/gasoline_dl" "$BINARY_URL"; then
    echo -e "${RED}Download failed after 3 attempts.${NC}"
    echo -e "URL: $BINARY_URL"
    echo -e "Check your network connection, proxy settings, or try again later."
    beacon_error "download_failed"
    exit 1
fi

# Validate binary size — catch truncated downloads and HTML error pages.
DOWNLOADED_SIZE=$(wc -c < "$TEMP_ROOT/gasoline_dl" | tr -d ' ')
if [ "$DOWNLOADED_SIZE" -lt "$MIN_BINARY_BYTES" ]; then
    echo -e "${RED}Downloaded file is too small (${DOWNLOADED_SIZE} bytes, expected >${MIN_BINARY_BYTES}).${NC}"
    echo -e "The download may have been truncated or intercepted by a proxy."
    beacon_error "binary_too_small"
    exit 1
fi

# ─────────────────────────────────────────────────────────────
# 7. Integrity Verification (SHA-256)
# ─────────────────────────────────────────────────────────────

CHECKSUM_VERIFIED=0
if curl -fsSL --max-time 15 "$CHECKSUM_URL" -o "$TEMP_ROOT/checksums.txt" 2>/dev/null; then
    EXPECTED_HASH=$(grep "$BINARY_NAME" "$TEMP_ROOT/checksums.txt" | awk '{print $1}' || true)
    ACTUAL_HASH=""

    if [ -z "$EXPECTED_HASH" ]; then
        if [ "$STRICT_CHECKSUM" = "1" ]; then
            echo -e "${RED}Strict checksum mode: checksums.txt missing entry for $BINARY_NAME.${NC}"
            exit 1
        fi
        echo -e "${YELLOW}  checksums.txt did not contain $BINARY_NAME; continuing without checksum verification.${NC}"
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL_HASH=$(shasum -a 256 "$TEMP_ROOT/gasoline_dl" | awk '{print $1}')
    elif command -v sha256sum >/dev/null 2>&1; then
        ACTUAL_HASH=$(sha256sum "$TEMP_ROOT/gasoline_dl" | awk '{print $1}')
    else
        if [ "$STRICT_CHECKSUM" = "1" ]; then
            echo -e "${RED}Strict checksum mode: no SHA-256 tool found (need shasum or sha256sum).${NC}"
            exit 1
        fi
        echo -e "${YELLOW}  No SHA-256 tool found (shasum/sha256sum); continuing without checksum verification.${NC}"
    fi

    if [ -n "${ACTUAL_HASH:-}" ]; then
        if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
            echo -e "${RED}Checksum verification failed! The binary may be corrupted or tampered with.${NC}"
            echo -e "Expected: $EXPECTED_HASH"
            echo -e "Actual:   $ACTUAL_HASH"
            beacon_error "checksum_mismatch"
            exit 1
        fi
        CHECKSUM_VERIFIED=1
        echo -e "${GREEN}Checksum verified.${NC}"
    fi
else
    if [ "$STRICT_CHECKSUM" = "1" ]; then
        echo -e "${RED}Strict checksum mode: failed to download checksum manifest.${NC}"
        exit 1
    fi
    echo -e "${YELLOW}  Checksum verification skipped (could not fetch manifest).${NC}"
fi

if [ "$STRICT_CHECKSUM" = "1" ] && [ "$CHECKSUM_VERIFIED" -ne 1 ]; then
    echo -e "${RED}Strict checksum mode: verification did not complete successfully.${NC}"
    exit 1
fi

# Move the verified binary to its final path and set executable permissions.
mv "$TEMP_ROOT/gasoline_dl" "$CANONICAL_GASOLINE_BIN"
chmod 755 "$CANONICAL_GASOLINE_BIN"

# Quick smoke test — verify the binary actually runs.
if ! "$CANONICAL_GASOLINE_BIN" --version >/dev/null 2>&1; then
    echo -e "${RED}Binary smoke test failed — the downloaded binary cannot execute.${NC}"
    echo -e "This may indicate an architecture mismatch or a corrupted download."
    echo -e "Platform: $PLATFORM-$E_ARCH, Binary: $BINARY_NAME"
    beacon_error "smoke_test_failed"
    exit 1
fi

if sync_binary_compat_aliases "$CANONICAL_GASOLINE_BIN" "$LEGACY_GASOLINE_BIN" "$LEGACY_GASOLINE_BROWSER_BIN"; then
    echo -e "${GREEN}Binary installed with command aliases.${NC}"
else
    echo -e "${YELLOW}  Core binary installed, but one or more compatibility aliases could not be created.${NC}"
fi

# ─────────────────────────────────────────────────────────────
# 8. Extension Staging
# ─────────────────────────────────────────────────────────────

echo -e "Refreshing browser extension..."
EXT_ZIP_NAME="gasoline-extension-v$VERSION.zip"
EXT_ZIP_URL="https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
TEMP_ZIP="$TEMP_ROOT/extension.zip"

if curl_retry "$TEMP_ZIP" "$EXT_ZIP_URL"; then
    # Dedicated extension zip exists (faster); validate required module files after extract.
    prepare_extension_stage
    if unzip -q -o "$TEMP_ZIP" -d "$STAGE_EXT_DIR" && validate_extension_stage "$STAGE_EXT_DIR"; then
        promote_extension_stage
        echo -e "${GREEN}Extension staged: $EXT_DIR${NC}"
    else
        echo -e "${YELLOW}  Release extension zip missing required modules; falling back to source zip...${NC}"
        SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/heads/STABLE.zip"
        if stage_extension_from_source_zip "$SOURCE_ZIP_URL"; then
            promote_extension_stage
            echo -e "${GREEN}Extension staged: $EXT_DIR${NC}"
        else
            echo -e "${RED}Failed to download extension source archive.${NC}"
            exit 1
        fi
    fi
else
    # Fallback to source zip extraction (covers older releases and bad extension zip assets)
    SOURCE_ZIP_URL="https://github.com/$REPO/archive/refs/heads/STABLE.zip"
    if stage_extension_from_source_zip "$SOURCE_ZIP_URL"; then
        promote_extension_stage
        echo -e "${GREEN}Extension staged: $EXT_DIR${NC}"
    else
        echo -e "${RED}Failed to download extension after multiple attempts.${NC}"
        echo -e "URL: $EXT_ZIP_URL"
        exit 1
    fi
fi

# ─────────────────────────────────────────────────────────────
# 9. Native Configuration (Go binary --install)
# ─────────────────────────────────────────────────────────────

echo -e "Finalizing configuration..."
if ! "$CANONICAL_GASOLINE_BIN" --install; then
    echo -e "${YELLOW}Native configuration returned an error.${NC}"
    echo -e "The binary and extension were installed successfully."
    echo -e "You may need to manually configure your MCP clients."
    echo -e "Run: $CANONICAL_GASOLINE_BIN --install"
    # Don't exit 1 here — the core install succeeded, only config auto-detection had issues.
fi

# ─────────────────────────────────────────────────────────────
# 10. Post-install health verification
# ─────────────────────────────────────────────────────────────

# Give the daemon a moment to start.
sleep 1
HEALTH_OK=0
HEALTH_RESPONSE=$(curl -sS --max-time 5 "http://127.0.0.1:7890/health" 2>/dev/null || true)
if echo "$HEALTH_RESPONSE" | grep -q '"status"' 2>/dev/null; then
    HEALTH_OK=1
fi

if [ "$HEALTH_OK" = "1" ]; then
    echo -e "${GREEN}Server health check passed (port 7890).${NC}"
else
    echo -e "${YELLOW}  Server not yet responding on port 7890 — may still be starting.${NC}"
    echo -e "  Verify: curl http://127.0.0.1:7890/health"
fi

# ─────────────────────────────────────────────────────────────
# 11. Register start-on-login
# ─────────────────────────────────────────────────────────────

register_autostart() {
    if [ "$PLATFORM" = "darwin" ]; then
        local plist_dir="$HOME/Library/LaunchAgents"
        local plist_path="$plist_dir/com.gasoline.daemon.plist"
        mkdir -p "$plist_dir"

        cat > "$plist_path" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.gasoline.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>$CANONICAL_GASOLINE_BIN</string>
        <string>--daemon</string>
        <string>--port</string>
        <string>7890</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/dev/null</string>
    <key>StandardErrorPath</key>
    <string>/dev/null</string>
</dict>
</plist>
PLIST

        # Unload previous registration (if any), then register fresh.
        launchctl bootout "gui/$(id -u)/com.gasoline.daemon" 2>/dev/null || true
        if launchctl bootstrap "gui/$(id -u)" "$plist_path" 2>/dev/null || \
           launchctl load "$plist_path" 2>/dev/null; then
            echo -e "${GREEN}Registered to start on login (LaunchAgent).${NC}"
        else
            echo -e "${YELLOW}  Could not register LaunchAgent automatically.${NC}"
            echo -e "  To start on login manually: launchctl load $plist_path"
        fi

    elif [ "$PLATFORM" = "linux" ]; then
        if command -v systemctl >/dev/null 2>&1 && systemctl --user status >/dev/null 2>&1; then
            local service_dir="$HOME/.config/systemd/user"
            local service_path="$service_dir/gasoline.service"
            mkdir -p "$service_dir"

            cat > "$service_path" <<SERVICE
[Unit]
Description=Gasoline Agentic Devtools Daemon
After=network.target

[Service]
Type=simple
ExecStart=$CANONICAL_GASOLINE_BIN --daemon --port 7890
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
SERVICE

            systemctl --user daemon-reload 2>/dev/null || true
            if systemctl --user enable gasoline.service 2>/dev/null; then
                echo -e "${GREEN}Registered to start on login (systemd user service).${NC}"
            else
                echo -e "${YELLOW}  Could not enable systemd service.${NC}"
                echo -e "  To start on login manually: systemctl --user enable gasoline.service"
            fi
        else
            # Fallback: XDG autostart for non-systemd desktops.
            local autostart_dir="$HOME/.config/autostart"
            local desktop_path="$autostart_dir/gasoline.desktop"
            mkdir -p "$autostart_dir"

            cat > "$desktop_path" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Gasoline Daemon
Exec=$CANONICAL_GASOLINE_BIN --daemon --port 7890
Hidden=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
DESKTOP

            echo -e "${GREEN}Registered to start on login (XDG autostart).${NC}"
        fi
    fi
}

register_autostart

# ─────────────────────────────────────────────────────────────
# 12. PATH registration
# ─────────────────────────────────────────────────────────────

register_path() {
    case ":${PATH}:" in
        *":$BIN_DIR:"*) return 0 ;;
    esac

    local shell_name=""
    shell_name=$(basename "${SHELL:-/bin/bash}")
    local rc_file=""
    case "$shell_name" in
        zsh)  rc_file="$HOME/.zshrc" ;;
        bash) rc_file="$HOME/.bashrc" ;;
        fish) rc_file="$HOME/.config/fish/config.fish" ;;
        *)    rc_file="$HOME/.profile" ;;
    esac

    # Check if already present in rc file (idempotent).
    if [ -f "$rc_file" ] && grep -qF "$BIN_DIR" "$rc_file" 2>/dev/null; then
        return 0
    fi

    local path_line=""
    if [ "$shell_name" = "fish" ]; then
        path_line="fish_add_path $BIN_DIR # gasoline"
    else
        path_line="export PATH=\"$BIN_DIR:\$PATH\" # gasoline"
    fi

    echo "" >> "$rc_file"
    echo "$path_line" >> "$rc_file"
    echo -e "${GREEN}Added $BIN_DIR to PATH in $rc_file${NC}"
    echo -e "Restart your terminal or run: ${BOLD}source $rc_file${NC}"
}

register_path

# ─────────────────────────────────────────────────────────────
# 13. Anonymous telemetry (disable: STRUM_TELEMETRY=off)
# ─────────────────────────────────────────────────────────────

if [ "${STRUM_TELEMETRY:-}" != "off" ]; then
    curl -s --max-time 2 -X POST "https://t.getstrum.dev/v1/event" \
        -H "Content-Type: application/json" \
        -d "{\"event\":\"install_complete\",\"v\":\"${VERSION}\",\"os\":\"$(uname -s)-$(uname -m)\",\"props\":{\"method\":\"curl\"}}" \
        > /dev/null 2>&1 &
fi

# ─────────────────────────────────────────────────────────────
# 14. Final summary
# ─────────────────────────────────────────────────────────────

echo ""
if [ "$IS_UPGRADE" = "1" ] && [ -n "$PREVIOUS_VERSION" ]; then
    echo -e "${GREEN}${BOLD}Gasoline upgraded: v$PREVIOUS_VERSION -> v$VERSION${NC}"
else
    echo -e "${GREEN}${BOLD}Gasoline v$VERSION installed successfully.${NC}"
fi
