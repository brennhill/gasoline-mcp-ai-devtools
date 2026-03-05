# Gasoline - Ultimate Windows Installer (PowerShell)
# https://github.com/brennhill/gasoline-mcp-ai-devtools
#
# PURPOSE:
# This PowerShell script provides a native, one-liner installation for Windows users.
# It avoids external dependencies like bash/curl by using built-in .NET/PowerShell features.
#
# USAGE:
#   irm https://raw.githubusercontent.com/brennhill/gasoline-mcp-ai-devtools/STABLE/scripts/install.ps1 | iex

# Stop the script if any command results in an error. Equivalent to 'set -e'.
$ErrorActionPreference = "Stop"

# Configuration: Single source of truth for repository and local paths.
$REPO = "brennhill/gasoline-mcp-ai-devtools"
$INSTALL_DIR = Join-Path $HOME ".gasoline"
$BIN_DIR = Join-Path $INSTALL_DIR "bin"
$EXT_DIR = Join-Path $INSTALL_DIR "extension"
$GASOLINE_BIN = Join-Path $BIN_DIR "gasoline.exe"
# Release version source of truth.
$VERSION_URL = "https://raw.githubusercontent.com/$REPO/STABLE/VERSION"
$INSTALL_WARNINGS = New-Object System.Collections.Generic.List[string]
$script:WARNINGS_PRINTED = $false

function Add-InstallWarning {
    param([string]$Message)
    if (-not [string]::IsNullOrWhiteSpace($Message)) {
        [void]$INSTALL_WARNINGS.Add($Message)
    }
}

function Show-InstallWarnings {
    if ($script:WARNINGS_PRINTED -or $INSTALL_WARNINGS.Count -eq 0) {
        return
    }
    $script:WARNINGS_PRINTED = $true

    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Red
    Write-Host "🚨 INSTALL WARNING: MANUAL ACTION REQUIRED" -ForegroundColor Red
    foreach ($warning in $INSTALL_WARNINGS) {
        Write-Host " - $warning" -ForegroundColor Yellow
    }
    Write-Host ""
    Write-Host "The old server may still be running. Kill it manually:" -ForegroundColor Red
    Write-Host "  Get-Process gasoline -ErrorAction SilentlyContinue | Stop-Process -Force" -ForegroundColor Yellow
    Write-Host "  taskkill /F /IM gasoline.exe /T" -ForegroundColor Yellow
    Write-Host "  Remove-Item `"$GASOLINE_BIN`" -Force" -ForegroundColor Yellow
    Write-Host "Then re-run installer:" -ForegroundColor Red
    Write-Host "  irm https://raw.githubusercontent.com/$REPO/STABLE/scripts/install.ps1 | iex" -ForegroundColor Yellow
    Write-Host "============================================================" -ForegroundColor Red
}

function Get-GasolineServerPids {
    $pids = @()
    $targetPath = [System.IO.Path]::GetFullPath($GASOLINE_BIN).ToLowerInvariant()
    $processes = Get-CimInstance Win32_Process -Filter "Name = 'gasoline.exe'" -ErrorAction SilentlyContinue

    foreach ($proc in $processes) {
        if (-not $proc.ProcessId) { continue }

        if ([string]::IsNullOrWhiteSpace($proc.ExecutablePath)) {
            # No path info: still kill to avoid stale lock survivors.
            $pids += [int]$proc.ProcessId
            continue
        }

        try {
            $procPath = [System.IO.Path]::GetFullPath($proc.ExecutablePath).ToLowerInvariant()
            if ($procPath -eq $targetPath) {
                $pids += [int]$proc.ProcessId
            }
        } catch {
            $pids += [int]$proc.ProcessId
        }
    }

    return @($pids | Sort-Object -Unique)
}

function Stop-GasolineServerProcesses {
    $targetPids = @(Get-GasolineServerPids)
    if ($targetPids.Count -eq 0) {
        return $true
    }

    Write-Host "🛑 Stopping running Gasoline server: PID(s) $($targetPids -join ', ')"
    foreach ($procId in $targetPids) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }

    Start-Sleep -Milliseconds 350
    $remaining = @(Get-GasolineServerPids)
    if ($remaining.Count -eq 0) {
        return $true
    }

    Write-Host "⚠️  Escalating termination with taskkill..." -ForegroundColor Yellow
    foreach ($procId in $remaining) {
        & taskkill /F /PID $procId /T *> $null
    }

    Start-Sleep -Milliseconds 500
    $remaining = @(Get-GasolineServerPids)
    if ($remaining.Count -eq 0) {
        return $true
    }

    Add-InstallWarning "Old server is still running after forced stop attempt (PID(s): $($remaining -join ', '))."
    return $false
}

function Replace-GasolineBinary {
    param(
        [string]$StagePath,
        [string]$LivePath
    )

    $maxAttempts = 4
    for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
        [void](Stop-GasolineServerProcesses)
        try {
            if (Test-Path $LivePath) {
                Remove-Item -Path $LivePath -Force -ErrorAction Stop
            }
            Move-Item -Path $StagePath -Destination $LivePath -Force -ErrorAction Stop
            return $true
        } catch {
            if ($attempt -lt $maxAttempts) {
                Write-Host "⚠️  Binary replace attempt $attempt/$maxAttempts failed; retrying..." -ForegroundColor Yellow
                Start-Sleep -Milliseconds (400 * $attempt)
                continue
            }
            Add-InstallWarning "Could not replace $LivePath due to an active file/process lock."
            return $false
        }
    }

    return $false
}

Write-Host ""
Write-Host '   ____                 _ _            ' -ForegroundColor DarkYellow
Write-Host '  / ___| __ _ ___  ___ | (_)_ __   ___ ' -ForegroundColor DarkYellow
Write-Host " | |  _ / _` / __|/ _ \| | | '_ \ / _ \\" -ForegroundColor DarkYellow
Write-Host ' | |_| | (_| \__ \ (_) | | | | | |  __/' -ForegroundColor DarkYellow
Write-Host '  \____|\__,_|___/\___/|_|_|_| |_|\___|' -ForegroundColor DarkYellow
Write-Host ""
Write-Host "🔥 Gasoline Installer" -ForegroundColor DarkYellow
Write-Host "--------------------------------------------------" -ForegroundColor DarkYellow
function Reset-ExtensionDir {
    if (Test-Path $EXT_DIR) {
        Remove-Item -Path (Join-Path $EXT_DIR '*') -Recurse -Force -ErrorAction SilentlyContinue
    } else {
        New-Item -Path $EXT_DIR -ItemType Directory -Force | Out-Null
    }
}

function Test-ExtensionStage {
    $required = @(
        (Join-Path $EXT_DIR "manifest.json"),
        (Join-Path $EXT_DIR "background\init.js"),
        (Join-Path $EXT_DIR "content\script-injection.js"),
        (Join-Path $EXT_DIR "inject\index.js"),
        (Join-Path $EXT_DIR "theme-bootstrap.js")
    )
    foreach ($path in $required) {
        if (-not (Test-Path $path)) {
            return $false
        }
    }
    return $true
}

# 1. Fetch Version: Get the latest stable version tag from GitHub.
Write-Host "🔍 Checking for updates..."
$VERSION = (Invoke-RestMethod -Uri $VERSION_URL).Trim()
Write-Host "✨ Version: v$VERSION (win32-x64)"

# 2. Directory Setup: Ensure the target installation folders exist on the filesystem.
if (-not (Test-Path $BIN_DIR)) { New-Item -Path $BIN_DIR -ItemType Directory -Force }
Reset-ExtensionDir
Write-Host "📁 Install root: $INSTALL_DIR"

# 3. Binary Installation: Download the Windows-native executable.
$INSTALL_BIN = $GASOLINE_BIN
$BINARY_NAME = "gasoline-win32-x64.exe"
$BINARY_URL = "https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
$CHECKSUM_URL = "https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"
$STAGED_BIN = "$GASOLINE_BIN.tmp"

Write-Host "⬇️  Downloading latest binary..."
# Download to a temporary '.tmp' file to ensure an atomic replacement later.
if (Test-Path $STAGED_BIN) {
    Remove-Item -Path $STAGED_BIN -Force -ErrorAction SilentlyContinue
}
Invoke-WebRequest -Uri $BINARY_URL -OutFile $STAGED_BIN

# 4. Integrity Verification: Verify the SHA-256 hash against the official release manifest.
try {
    # Fetch the checksum manifest and parse the hash for the windows binary.
    $checksums = Invoke-RestMethod -Uri $CHECKSUM_URL
    $expectedLine = ($checksums -split "`n") | Where-Object { $_ -match $BINARY_NAME }
    if ($expectedLine) {
        $expectedHash = ($expectedLine -split "\s+")[0]
        # Calculate the hash of the downloaded file using built-in Windows security tools.
        $actualHash = (Get-FileHash $STAGED_BIN -Algorithm SHA256).Hash.ToLower()
        if ($expectedHash -ne $actualHash) {
            Write-Error "❌ Checksum verification failed! The download may be corrupted."
        }
        Write-Host "✅ Checksum verified."
    }
} catch {
    # Non-fatal warning if checksums cannot be verified (e.g., firewall issues).
    Write-Host "⚠️  Checksum verification skipped (could not fetch manifest)." -ForegroundColor Yellow
}

# Force-stop old server, then replace binary with retries for lock contention.
if (-not (Replace-GasolineBinary -StagePath $STAGED_BIN -LivePath $GASOLINE_BIN)) {
    $FALLBACK_BIN = Join-Path $BIN_DIR "gasoline.new.exe"
    try {
        Move-Item -Path $STAGED_BIN -Destination $FALLBACK_BIN -Force -ErrorAction Stop
        $INSTALL_BIN = $FALLBACK_BIN
        Add-InstallWarning "Using fallback binary $FALLBACK_BIN because gasoline.exe could not be replaced."
    } catch {
        Add-InstallWarning "Downloaded update could not be installed. gasoline.exe is likely still locked by a running process."
    }
} else {
    Write-Host "✅ Binary replaced: $GASOLINE_BIN"
}

# 5. Extension Staging: Refresh the browser extension files.
# Tries the optimized release asset first, falling back to the full source zip if missing.
Write-Host "⬇️  Refreshing browser extension..."
$EXT_ZIP_NAME = "gasoline-extension-v$VERSION.zip"
$EXT_ZIP_URL = "https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
$TEMP_ZIP = Join-Path $env:TEMP "gasoline-ext.zip"

try {
    Invoke-WebRequest -Uri $EXT_ZIP_URL -OutFile $TEMP_ZIP
    Reset-ExtensionDir
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $EXT_DIR -Force
    if (-not (Test-ExtensionStage)) {
        throw "Release extension zip missing required module files"
    }
    Write-Host "✅ Staged extension directory: $EXT_DIR"
} catch {
    # Fallback logic for older releases or bad extension zip assets.
    Write-Host "⚠️  Falling back to source zip due to missing/incomplete extension zip" -ForegroundColor Yellow
    $SOURCE_ZIP_URL = "https://github.com/$REPO/archive/refs/heads/STABLE.zip"
    Invoke-WebRequest -Uri $SOURCE_ZIP_URL -OutFile $TEMP_ZIP
    $TEMP_EXTRACT = Join-Path $env:TEMP "gasoline-ext-src"
    if (Test-Path $TEMP_EXTRACT) { Remove-Item -Path $TEMP_EXTRACT -Recurse -Force }
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $TEMP_EXTRACT -Force
    # Find the extracted folder (named repo-version) and copy the extension subdirectory.
    $extractRoot = Get-ChildItem -Path $TEMP_EXTRACT | Select-Object -First 1
    Reset-ExtensionDir
    Copy-Item -Path (Join-Path $extractRoot.FullName "extension\*") -Destination $EXT_DIR -Recurse -Force
    if (-not (Test-ExtensionStage)) {
        throw "Extension staging failed: required module files are missing."
    }
    Write-Host "✅ Staged extension directory: $EXT_DIR"
    # Clean up the deep source extraction.
    Remove-Item -Path $TEMP_EXTRACT -Recurse -Force
}
# Cleanup the temporary zip file.
Remove-Item -Path $TEMP_ZIP -ErrorAction SilentlyContinue

# 6. Native Configuration: Execute the Go binary to handle complex client configuration.
# The binary's --install flag will:
#   - Detect all installed MCP clients (Claude, Cursor, VS Code, etc.).
#   - Safely update JSON configuration files with Windows-aware paths.
#   - Reset any running Gasoline processes.
#   - Display final success message and extension instructions.
Write-Host "🚀 Finalizing configuration..."
if (-not (Test-Path $INSTALL_BIN)) {
    Add-InstallWarning "Installer could not locate an executable to run for --install."
    Show-InstallWarnings
    throw "Gasoline binary install failed. See warning panel for manual recovery steps."
}

& $INSTALL_BIN --install
[void](Stop-GasolineServerProcesses)
Show-InstallWarnings
