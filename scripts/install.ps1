# Gasoline Agentic Browser - Ultimate Windows Installer (PowerShell)
# https://github.com/brennhill/gasoline-agentic-browser-devtools-mcp
#
# PURPOSE:
# This PowerShell script provides a native, one-liner installation for Windows users.
# It avoids external dependencies like bash/curl by using built-in .NET/PowerShell features.
#
# USAGE:
#   irm https://raw.githubusercontent.com/brennhill/gasoline-agentic-browser-devtools-mcp/STABLE/scripts/install.ps1 | iex

# Stop the script if any command results in an error. Equivalent to 'set -e'.
$ErrorActionPreference = "Stop"

# Configuration: Single source of truth for repository and local paths.
$REPO = "brennhill/gasoline-agentic-browser-devtools-mcp"
$INSTALL_DIR = Join-Path $HOME ".gasoline"
$BIN_DIR = Join-Path $INSTALL_DIR "bin"
$EXT_DIR = Join-Path $INSTALL_DIR "extension"
# Release version source of truth.
$VERSION_URL = "https://raw.githubusercontent.com/$REPO/STABLE/VERSION"

$TOTAL_STEPS = 7
function Write-Panel([string]$Title, [string[]]$Lines) {
    Write-Host "+----------------------------------------------------------+" -ForegroundColor Cyan
    $titleLine = "{0,-56}" -f $Title
    Write-Host ("| " + $titleLine + " |") -ForegroundColor Cyan
    Write-Host "+----------------------------------------------------------+" -ForegroundColor Cyan
    foreach ($line in $Lines) {
        $content = "{0,-58}" -f $line
        Write-Host ("|" + " " + $content + " |") -ForegroundColor Cyan
    }
    Write-Host "+----------------------------------------------------------+" -ForegroundColor Cyan
}
function Write-Banner() {
    Write-Host ""
    Write-Host '   ____                 _ _            ' -ForegroundColor Cyan
    Write-Host '  / ___| __ _ ___  ___ | (_)_ __   ___ ' -ForegroundColor Cyan
    Write-Host " | |  _ / _` / __|/ _ \| | | '_ \ / _ \\" -ForegroundColor Cyan
    Write-Host ' | |_| | (_| \__ \ (_) | | | | | |  __/' -ForegroundColor Cyan
    Write-Host '  \____|\__,_|___/\___/|_|_|_| |_|\___|' -ForegroundColor Cyan
    Write-Host ""
    Write-Panel -Title "GASOLINE INSTALLER" -Lines @(
        "Polished one-shot setup for binary + extension + MCP config.",
        "",
        "Install flow:",
        "  1) Resolve latest stable release",
        "  2) Download + verify binary",
        "  3) Stage extension files",
        "  4) Configure MCP clients",
        "  5) Show manual browser checklist"
    )
}
function Write-Progress([int]$Number) {
    $width = 28
    $filled = [Math]::Floor(($Number / [double]$TOTAL_STEPS) * $width)
    $bar = ("#" * $filled).PadRight($width, "-")
    $pct = [Math]::Floor(($Number / [double]$TOTAL_STEPS) * 100)
    Write-Host "   [$bar] $pct%" -ForegroundColor Yellow
}
function Write-Step([int]$Number, [string]$Message) {
    Write-Host ""
    Write-Host "[$Number/$TOTAL_STEPS] $Message" -ForegroundColor Cyan
    Write-Progress $Number
}
function Write-StepOk([string]$Message) {
    Write-Host "   ✓ $Message" -ForegroundColor Green
}
function Write-StepNote([string]$Message) {
    Write-Host "   -> $Message" -ForegroundColor DarkGray
}

Write-Banner

# 1. Fetch Version: Get the latest stable version tag from GitHub.
Write-Step 1 "Resolving latest stable version"
$VERSION = (Invoke-RestMethod -Uri $VERSION_URL).Trim()
Write-StepOk "Version: v$VERSION (win32-x64)"

# 2. Directory Setup: Ensure the target installation folders exist on the filesystem.
Write-Step 2 "Preparing install directories"
if (-not (Test-Path $BIN_DIR)) { New-Item -Path $BIN_DIR -ItemType Directory -Force }
if (-not (Test-Path $EXT_DIR)) { New-Item -Path $EXT_DIR -ItemType Directory -Force }
Write-StepOk "Install root: $INSTALL_DIR"

# 3. Binary Installation: Download the Windows-native executable.
$GASOLINE_BIN = Join-Path $BIN_DIR "gasoline-agentic-browser.exe"
$BINARY_NAME = "gasoline-agentic-browser-win32-x64.exe"
$BINARY_URL = "https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
$CHECKSUM_URL = "https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"

Write-Step 3 "Downloading and verifying binary"
Write-StepNote "Downloading release artifact and validating integrity"
# Download to a temporary '.tmp' file to ensure an atomic replacement later.
Invoke-WebRequest -Uri $BINARY_URL -OutFile "$GASOLINE_BIN.tmp"

# 4. Integrity Verification: Verify the SHA-256 hash against the official release manifest.
try {
    # Fetch the checksum manifest and parse the hash for the windows binary.
    $checksums = Invoke-RestMethod -Uri $CHECKSUM_URL
    $expectedLine = ($checksums -split "`n") | Where-Object { $_ -match $BINARY_NAME }
    if ($expectedLine) {
        $expectedHash = ($expectedLine -split "\s+")[0]
        # Calculate the hash of the downloaded file using built-in Windows security tools.
        $actualHash = (Get-FileHash "$GASOLINE_BIN.tmp" -Algorithm SHA256).Hash.ToLower()
        if ($expectedHash -ne $actualHash) {
            Write-Error "❌ Checksum verification failed! The download may be corrupted."
        }
        Write-Host "✅ Checksum verified."
    }
} catch {
    # Non-fatal warning if checksums cannot be verified (e.g., firewall issues).
    Write-Host "⚠️  Checksum verification skipped (could not fetch manifest)." -ForegroundColor Yellow
}

# Atomically replace the old binary with the newly downloaded and verified version.
Move-Item -Path "$GASOLINE_BIN.tmp" -Destination $GASOLINE_BIN -Force
Write-StepOk "Installed binary: $GASOLINE_BIN"

# 5. Extension Staging: Refresh the browser extension files.
# Tries the optimized release asset first, falling back to the full source zip if missing.
Write-Step 4 "Staging browser extension files (manual browser load required)"
Write-StepNote "Using extension zip when available; source zip fallback for older releases"
$EXT_ZIP_NAME = "gasoline-extension-v$VERSION.zip"
$EXT_ZIP_URL = "https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
$TEMP_ZIP = Join-Path $env:TEMP "gasoline-ext.zip"

try {
    Invoke-WebRequest -Uri $EXT_ZIP_URL -OutFile $TEMP_ZIP
    # Native extraction to the local staging directory.
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $EXT_DIR -Force
} catch {
    # Fallback logic for older releases that only have source zips.
    Write-Host "📦 Falling back to source zip (this may take a moment)..." -ForegroundColor Yellow
    $SOURCE_ZIP_URL = "https://github.com/$REPO/archive/refs/tags/v$VERSION.zip"
    Invoke-WebRequest -Uri $SOURCE_ZIP_URL -OutFile $TEMP_ZIP
    $TEMP_EXTRACT = Join-Path $env:TEMP "gasoline-ext-src"
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $TEMP_EXTRACT -Force
    # Find the extracted folder (named repo-version) and copy the extension subdirectory.
    $extractRoot = Get-ChildItem -Path $TEMP_EXTRACT | Select-Object -First 1
    Copy-Item -Path (Join-Path $extractRoot.FullName "extension\*") -Destination $EXT_DIR -Recurse -Force
    # Clean up the deep source extraction.
    Remove-Item -Path $TEMP_EXTRACT -Recurse -Force
}
# Cleanup the temporary zip file.
Remove-Item -Path $TEMP_ZIP -ErrorAction SilentlyContinue
Write-StepOk "Staged extension directory: $EXT_DIR"

# 6. Native Configuration: Execute the Go binary to handle complex client configuration.
# The binary's --install flag will:
#   - Detect all installed MCP clients (Claude, Cursor, VS Code, etc.).
#   - Safely update JSON configuration files with Windows-aware paths.
#   - Reset any running Gasoline processes.
#   - Display final success message and extension instructions.
Write-Step 5 "Configuring MCP clients with direct binary path (no npx)"
Write-StepNote "Handing off to native installer for client-specific config merges"
& $GASOLINE_BIN --install

Write-Step 6 "Reminder: browser extension load is manual"
Write-Panel -Title "MANUAL BROWSER CHECKLIST" -Lines @(
    "The installer cannot click browser UI controls for you.",
    "",
    "1) Open chrome://extensions (or brave://extensions)",
    "2) Enable Developer mode",
    "3) Click Load unpacked and select: $EXT_DIR",
    "4) Pin Gasoline in the toolbar (recommended)",
    "5) Open popup and click Track This Tab"
)

Write-Step 7 "Install workflow complete"
Write-StepOk "If you do not see data, click the extension popup and track the active tab"
