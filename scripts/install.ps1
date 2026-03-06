# Gasoline - Ultimate Windows Installer (PowerShell)
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
$EXT_DIR = if ($env:GASOLINE_EXTENSION_DIR) { $env:GASOLINE_EXTENSION_DIR } else { Join-Path $HOME "GasolineAgenticDevtoolExtension" }
$CANONICAL_GASOLINE_BIN = Join-Path $BIN_DIR "gasoline-agentic-devtools.exe"
$LEGACY_GASOLINE_BIN = Join-Path $BIN_DIR "gasoline.exe"
$LEGACY_GASOLINE_BROWSER_BIN = Join-Path $BIN_DIR "gasoline-agentic-browser.exe"
$GASOLINE_BIN = $CANONICAL_GASOLINE_BIN
# Release version source of truth.
$VERSION_URL = "https://raw.githubusercontent.com/$REPO/STABLE/VERSION"
$STRICT_CHECKSUM = $env:GASOLINE_INSTALL_STRICT -eq "1"
$TEMP_TOKEN = [Guid]::NewGuid().ToString("N")
$STAGE_EXT_DIR = Join-Path $INSTALL_DIR ".extension-stage-$TEMP_TOKEN"
$BACKUP_EXT_DIR = Join-Path $INSTALL_DIR ".extension-backup-$TEMP_TOKEN"
$INSTALL_WARNINGS = New-Object System.Collections.Generic.List[string]
$script:WARNINGS_PRINTED = $false
# Minimum plausible binary size (5 MB). Catches truncated downloads and HTML error pages.
$MIN_BINARY_BYTES = 5000000

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
    Write-Host "INSTALL WARNING: MANUAL ACTION REQUIRED" -ForegroundColor Red
    foreach ($warning in $INSTALL_WARNINGS) {
        Write-Host " - $warning" -ForegroundColor Yellow
    }
    Write-Host ""
    Write-Host "The old server may still be running. Kill it manually:" -ForegroundColor Red
    Write-Host "  Get-Process gasoline-agentic-devtools,gasoline-agentic-browser,gasoline -ErrorAction SilentlyContinue | Stop-Process -Force" -ForegroundColor Yellow
    Write-Host "  taskkill /F /IM gasoline-agentic-devtools.exe /IM gasoline-agentic-browser.exe /IM gasoline.exe /T" -ForegroundColor Yellow
    Write-Host "  Remove-Item `"$CANONICAL_GASOLINE_BIN`",`"$LEGACY_GASOLINE_BIN`",`"$LEGACY_GASOLINE_BROWSER_BIN`" -Force" -ForegroundColor Yellow
    Write-Host "Then re-run installer:" -ForegroundColor Red
    Write-Host "  irm https://raw.githubusercontent.com/$REPO/STABLE/scripts/install.ps1 | iex" -ForegroundColor Yellow
    Write-Host "============================================================" -ForegroundColor Red
}

# ─────────────────────────────────────────────────────────────
# Prerequisite Checks
# ─────────────────────────────────────────────────────────────

function Test-NetworkConnectivity {
    try {
        $null = Invoke-WebRequest -Uri "https://github.com" -UseBasicParsing -TimeoutSec 10 -ErrorAction Stop
    } catch {
        Write-Host "Cannot reach github.com - check your network connection or proxy settings." -ForegroundColor Red
        Write-Host "If you are behind a corporate proxy, configure your system proxy before running." -ForegroundColor Yellow
        throw "Network connectivity check failed."
    }
}

function Test-DiskSpace {
    $requiredMB = 50
    try {
        $drive = (Get-Item $HOME).PSDrive
        $freeMB = [math]::Floor($drive.Free / 1MB)
        if ($freeMB -lt $requiredMB) {
            throw "Insufficient disk space: ${freeMB} MB available, need ${requiredMB} MB. Free up space and re-run."
        }
    } catch [System.Management.Automation.PropertyNotFoundException] {
        # PSDrive.Free not available on some configurations; skip check.
    }
}

function Test-WriteAccess {
    if (-not (Test-Path $INSTALL_DIR)) {
        New-Item -Path $INSTALL_DIR -ItemType Directory -Force | Out-Null
    }
    $testFile = Join-Path $INSTALL_DIR ".write-test"
    try {
        [System.IO.File]::WriteAllText($testFile, "test")
        Remove-Item -Path $testFile -Force -ErrorAction SilentlyContinue
    } catch {
        throw "Cannot write to $INSTALL_DIR - check directory permissions. If installed with elevated privileges previously, fix permissions and re-run."
    }
}

Test-NetworkConnectivity
Test-DiskSpace
Test-WriteAccess

# ─────────────────────────────────────────────────────────────
# Retry-capable download helper
# ─────────────────────────────────────────────────────────────

function Invoke-DownloadWithRetry {
    param(
        [string]$Uri,
        [string]$OutFile,
        [int]$MaxAttempts = 3,
        [int]$InitialDelaySec = 2
    )

    $delay = $InitialDelaySec
    for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++) {
        try {
            Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing -TimeoutSec 120 -ErrorAction Stop
            return
        } catch {
            if ($attempt -lt $MaxAttempts) {
                Write-Host "  Download attempt $attempt/$MaxAttempts failed; retrying in ${delay}s..." -ForegroundColor Yellow
                Start-Sleep -Seconds $delay
                $delay = $delay * 2
            } else {
                throw $_.Exception
            }
        }
    }
}

# ─────────────────────────────────────────────────────────────
# Process management helpers
# ─────────────────────────────────────────────────────────────

function Get-GasolineServerPids {
    $pids = @()
    $targetPaths = @(
        $CANONICAL_GASOLINE_BIN,
        $LEGACY_GASOLINE_BIN,
        $LEGACY_GASOLINE_BROWSER_BIN
    ) | ForEach-Object {
        try {
            [System.IO.Path]::GetFullPath($_).ToLowerInvariant()
        } catch {
            $null
        }
    } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }

    $processes = Get-CimInstance Win32_Process -Filter "Name = 'gasoline-agentic-devtools.exe' OR Name = 'gasoline-agentic-browser.exe' OR Name = 'gasoline.exe' OR Name = 'gasoline.new.exe'" -ErrorAction SilentlyContinue

    foreach ($proc in $processes) {
        if (-not $proc.ProcessId) { continue }

        if ([string]::IsNullOrWhiteSpace($proc.ExecutablePath)) {
            # No path info: still kill to avoid stale lock survivors.
            $pids += [int]$proc.ProcessId
            continue
        }

        try {
            $procPath = [System.IO.Path]::GetFullPath($proc.ExecutablePath).ToLowerInvariant()
            if ($targetPaths -contains $procPath) {
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

    Write-Host "  Stopping running Gasoline server: PID(s) $($targetPids -join ', ')"
    foreach ($procId in $targetPids) {
        Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }

    Start-Sleep -Milliseconds 350
    $remaining = @(Get-GasolineServerPids)
    if ($remaining.Count -eq 0) {
        return $true
    }

    Write-Host "  Escalating termination with taskkill..." -ForegroundColor Yellow
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
                Write-Host "  Binary replace attempt $attempt/$maxAttempts failed; retrying..." -ForegroundColor Yellow
                Start-Sleep -Milliseconds (400 * $attempt)
                continue
            }
            Add-InstallWarning "Could not replace $LivePath due to an active file/process lock."
            return $false
        }
    }

    return $false
}

function Sync-BinaryCompatAliases {
    param(
        [string]$CanonicalPath,
        [string[]]$AliasPaths
    )

    if (-not (Test-Path $CanonicalPath)) {
        Add-InstallWarning "Compatibility alias sync skipped because canonical binary is missing: $CanonicalPath"
        return $false
    }

    $allGood = $true
    foreach ($aliasPath in $AliasPaths) {
        if ([string]::IsNullOrWhiteSpace($aliasPath)) {
            continue
        }
        if ([System.IO.Path]::GetFullPath($aliasPath).ToLowerInvariant() -eq [System.IO.Path]::GetFullPath($CanonicalPath).ToLowerInvariant()) {
            continue
        }
        try {
            Copy-Item -Path $CanonicalPath -Destination $aliasPath -Force -ErrorAction Stop
        } catch {
            $allGood = $false
            Add-InstallWarning "Could not create compatibility alias: $aliasPath"
        }
    }
    return $allGood
}

# ─────────────────────────────────────────────────────────────
# Extension staging helpers
# ─────────────────────────────────────────────────────────────

function New-ExtensionStage {
    if (Test-Path $STAGE_EXT_DIR) {
        Remove-Item -Path $STAGE_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }
    New-Item -Path $STAGE_EXT_DIR -ItemType Directory -Force | Out-Null
}

function Test-ExtensionStage {
    param(
        [string]$BaseDir = $EXT_DIR
    )

    $hasManifest = Test-Path (Join-Path $BaseDir "manifest.json")
    $hasBackground = (Test-Path (Join-Path $BaseDir "background.js")) -or (Test-Path (Join-Path $BaseDir "background\init.js"))
    $hasContent = (Test-Path (Join-Path $BaseDir "content.bundled.js")) -or (Test-Path (Join-Path $BaseDir "content\script-injection.js"))
    $hasInject = (Test-Path (Join-Path $BaseDir "inject.bundled.js")) -or (Test-Path (Join-Path $BaseDir "inject\index.js"))
    $hasBootstrap = (Test-Path (Join-Path $BaseDir "early-patch.bundled.js")) -or (Test-Path (Join-Path $BaseDir "theme-bootstrap.js"))

    return ($hasManifest -and $hasBackground -and $hasContent -and $hasInject -and $hasBootstrap)
}

function Promote-ExtensionStage {
    if (-not (Test-ExtensionStage -BaseDir $STAGE_EXT_DIR)) {
        throw "Extension staging failed: required module files are missing from staging."
    }

    if (Test-Path $BACKUP_EXT_DIR) {
        Remove-Item -Path $BACKUP_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }

    if (Test-Path $EXT_DIR) {
        Move-Item -Path $EXT_DIR -Destination $BACKUP_EXT_DIR -Force
    }

    $extensionParentDir = Split-Path -Path $EXT_DIR -Parent
    if (-not [string]::IsNullOrWhiteSpace($extensionParentDir) -and -not (Test-Path $extensionParentDir)) {
        New-Item -Path $extensionParentDir -ItemType Directory -Force | Out-Null
    }

    try {
        Move-Item -Path $STAGE_EXT_DIR -Destination $EXT_DIR -Force
    } catch {
        if (Test-Path $BACKUP_EXT_DIR) {
            Move-Item -Path $BACKUP_EXT_DIR -Destination $EXT_DIR -Force -ErrorAction SilentlyContinue
        }
        throw
    }

    if (-not (Test-ExtensionStage -BaseDir $EXT_DIR)) {
        Remove-Item -Path $EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
        if (Test-Path $BACKUP_EXT_DIR) {
            Move-Item -Path $BACKUP_EXT_DIR -Destination $EXT_DIR -Force -ErrorAction SilentlyContinue
        }
        throw "Promoted extension failed validation; previous extension restored."
    }

    if (Test-Path $BACKUP_EXT_DIR) {
        Remove-Item -Path $BACKUP_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# ─────────────────────────────────────────────────────────────
# Banner
# ─────────────────────────────────────────────────────────────

Write-Host ""
Write-Host '   ____                 _ _            ' -ForegroundColor DarkYellow
Write-Host '  / ___| __ _ ___  ___ | (_)_ __   ___ ' -ForegroundColor DarkYellow
Write-Host " | |  _ / _` / __|/ _ \| | | '_ \ / _ \\" -ForegroundColor DarkYellow
Write-Host ' | |_| | (_| \__ \ (_) | | | | | |  __/' -ForegroundColor DarkYellow
Write-Host '  \____|\__,_|___/\___/|_|_|_| |_|\___|' -ForegroundColor DarkYellow
Write-Host ""
Write-Host "Gasoline Installer" -ForegroundColor DarkYellow
Write-Host "--------------------------------------------------" -ForegroundColor DarkYellow
if ($STRICT_CHECKSUM) {
    Write-Host "Strict checksum mode enabled (GASOLINE_INSTALL_STRICT=1)" -ForegroundColor Yellow
}

# ─────────────────────────────────────────────────────────────
# 1. Fetch Version
# ─────────────────────────────────────────────────────────────

Write-Host "Checking for updates..."
try {
    $VERSION = (Invoke-RestMethod -Uri $VERSION_URL -TimeoutSec 15).Trim()
} catch {
    Write-Host "Failed to fetch latest version from $VERSION_URL" -ForegroundColor Red
    Write-Host "Check your network connection and try again." -ForegroundColor Yellow
    throw "Version fetch failed: $($_.Exception.Message)"
}

# ─────────────────────────────────────────────────────────────
# 2. Detect install vs upgrade
# ─────────────────────────────────────────────────────────────

$IS_UPGRADE = $false
$PREVIOUS_VERSION = ""

if (Test-Path $CANONICAL_GASOLINE_BIN) {
    try {
        $versionOutput = & $CANONICAL_GASOLINE_BIN --version 2>&1
        if ($versionOutput -match '(\d+\.\d+\.\d+)') {
            $PREVIOUS_VERSION = $Matches[1]
            $IS_UPGRADE = $true
        }
    } catch {
        # Old binary may be corrupted or incompatible; treat as fresh install.
        $IS_UPGRADE = $true
    }
}

if ($IS_UPGRADE -and $PREVIOUS_VERSION) {
    Write-Host "Upgrading: v$PREVIOUS_VERSION -> v$VERSION (win32-x64)"
} else {
    Write-Host "Installing: v$VERSION (win32-x64)"
}

# ─────────────────────────────────────────────────────────────
# 3. Directory Setup
# ─────────────────────────────────────────────────────────────

if (-not (Test-Path $BIN_DIR)) { New-Item -Path $BIN_DIR -ItemType Directory -Force | Out-Null }
if (-not (Test-Path $INSTALL_DIR)) { New-Item -Path $INSTALL_DIR -ItemType Directory -Force | Out-Null }
Write-Host "Install root: $INSTALL_DIR"

# ─────────────────────────────────────────────────────────────
# 4. Binary Installation
# ─────────────────────────────────────────────────────────────

$INSTALL_BIN = $GASOLINE_BIN
$BINARY_NAME = "gasoline-agentic-devtools-win32-x64.exe"
$BINARY_URL = "https://github.com/$REPO/releases/download/v$VERSION/$BINARY_NAME"
$CHECKSUM_URL = "https://github.com/$REPO/releases/download/v$VERSION/checksums.txt"
$STAGED_BIN = "$GASOLINE_BIN.tmp.$TEMP_TOKEN"

Write-Host "Downloading binary..."
if (Test-Path $STAGED_BIN) {
    Remove-Item -Path $STAGED_BIN -Force -ErrorAction SilentlyContinue
}
try {
    Invoke-DownloadWithRetry -Uri $BINARY_URL -OutFile $STAGED_BIN
} catch {
    Write-Host "Download failed after 3 attempts." -ForegroundColor Red
    Write-Host "URL: $BINARY_URL" -ForegroundColor Yellow
    Write-Host "Check your network connection, proxy settings, or try again later." -ForegroundColor Yellow
    throw "Binary download failed: $($_.Exception.Message)"
}

# Validate binary size — catch truncated downloads and HTML error pages.
$downloadedSize = (Get-Item $STAGED_BIN).Length
if ($downloadedSize -lt $MIN_BINARY_BYTES) {
    Remove-Item -Path $STAGED_BIN -Force -ErrorAction SilentlyContinue
    throw "Downloaded file is too small ($downloadedSize bytes, expected >$MIN_BINARY_BYTES). The download may have been truncated or intercepted by a proxy."
}

# ─────────────────────────────────────────────────────────────
# 5. Integrity Verification (SHA-256)
# ─────────────────────────────────────────────────────────────

$checksumVerified = $false
try {
    $checksums = Invoke-RestMethod -Uri $CHECKSUM_URL -TimeoutSec 15
    $expectedLine = ($checksums -split "`n") | Where-Object { $_ -match $BINARY_NAME }
    if (-not $expectedLine) {
        throw "checksums.txt did not include $BINARY_NAME"
    }

    $expectedHash = ($expectedLine -split "\s+")[0].ToLower()
    $actualHash = (Get-FileHash $STAGED_BIN -Algorithm SHA256).Hash.ToLower()
    if ($expectedHash -ne $actualHash) {
        Remove-Item -Path $STAGED_BIN -Force -ErrorAction SilentlyContinue
        throw "Checksum mismatch for $BINARY_NAME`nExpected: $expectedHash`nActual:   $actualHash"
    }

    $checksumVerified = $true
    Write-Host "Checksum verified." -ForegroundColor Green
} catch {
    $msg = $_.Exception.Message
    if ($msg -like "*mismatch*") {
        throw "Checksum verification failed! The download may be corrupted or tampered with.`n$msg"
    }
    if ($STRICT_CHECKSUM) {
        throw "Strict checksum mode: $msg"
    }
    Write-Host "  Checksum verification skipped: $msg" -ForegroundColor Yellow
}

if ($STRICT_CHECKSUM -and -not $checksumVerified) {
    throw "Strict checksum mode: verification did not complete successfully."
}

# Force-stop old server, then replace binary with retries for lock contention.
if (-not (Replace-GasolineBinary -StagePath $STAGED_BIN -LivePath $GASOLINE_BIN)) {
    $FALLBACK_BIN = Join-Path $BIN_DIR "gasoline.new.exe"
    try {
        Move-Item -Path $STAGED_BIN -Destination $FALLBACK_BIN -Force -ErrorAction Stop
        $INSTALL_BIN = $FALLBACK_BIN
        Add-InstallWarning "Using fallback binary $FALLBACK_BIN because $(Split-Path -Path $GASOLINE_BIN -Leaf) could not be replaced."
    } catch {
        Add-InstallWarning "Downloaded update could not be installed. $(Split-Path -Path $GASOLINE_BIN -Leaf) is likely still locked by a running process."
    }
} else {
    Write-Host "Binary replaced: $GASOLINE_BIN" -ForegroundColor Green
}

# Quick smoke test — verify the binary actually runs.
try {
    $versionCheck = & $INSTALL_BIN --version 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "non-zero exit"
    }
} catch {
    Write-Host "Binary smoke test failed - the downloaded binary cannot execute." -ForegroundColor Red
    Write-Host "This may indicate a corrupted download. Try running the installer again." -ForegroundColor Yellow
}

$aliasTargets = @(
    $CANONICAL_GASOLINE_BIN,
    $LEGACY_GASOLINE_BIN,
    $LEGACY_GASOLINE_BROWSER_BIN
)
if (Sync-BinaryCompatAliases -CanonicalPath $INSTALL_BIN -AliasPaths $aliasTargets) {
    Write-Host "Binary installed with command aliases." -ForegroundColor Green
} else {
    Write-Host "  Core binary installed, but one or more compatibility aliases could not be created." -ForegroundColor Yellow
}

# ─────────────────────────────────────────────────────────────
# 6. Extension Staging
# ─────────────────────────────────────────────────────────────

Write-Host "Refreshing browser extension..."
$EXT_ZIP_NAME = "gasoline-extension-v$VERSION.zip"
$EXT_ZIP_URL = "https://github.com/$REPO/releases/download/v$VERSION/$EXT_ZIP_NAME"
$TEMP_ZIP = Join-Path $env:TEMP "gasoline-ext-$TEMP_TOKEN.zip"
$TEMP_EXTRACT = Join-Path $env:TEMP "gasoline-ext-src-$TEMP_TOKEN"

try {
    Invoke-DownloadWithRetry -Uri $EXT_ZIP_URL -OutFile $TEMP_ZIP
    New-ExtensionStage
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $STAGE_EXT_DIR -Force
    if (-not (Test-ExtensionStage -BaseDir $STAGE_EXT_DIR)) {
        throw "Release extension zip missing required module files"
    }
} catch {
    # Fallback logic for older releases or bad extension zip assets.
    Write-Host "  Falling back to source zip..." -ForegroundColor Yellow
    $SOURCE_ZIP_URL = "https://github.com/$REPO/archive/refs/heads/STABLE.zip"
    try {
        Invoke-DownloadWithRetry -Uri $SOURCE_ZIP_URL -OutFile $TEMP_ZIP
    } catch {
        throw "Failed to download extension after multiple attempts. Check your network connection."
    }
    if (Test-Path $TEMP_EXTRACT) { Remove-Item -Path $TEMP_EXTRACT -Recurse -Force }
    Expand-Archive -Path $TEMP_ZIP -DestinationPath $TEMP_EXTRACT -Force
    $extractRoot = Get-ChildItem -Path $TEMP_EXTRACT | Where-Object { $_.PSIsContainer } | Select-Object -First 1
    if (-not $extractRoot) {
        throw "Source zip extraction failed: missing root directory."
    }

    $sourceExtensionDir = Join-Path $extractRoot.FullName "extension"
    if (-not (Test-Path $sourceExtensionDir)) {
        throw "Source zip extraction failed: missing extension directory."
    }

    New-ExtensionStage
    Copy-Item -Path (Join-Path $sourceExtensionDir "*") -Destination $STAGE_EXT_DIR -Recurse -Force
    if (-not (Test-ExtensionStage -BaseDir $STAGE_EXT_DIR)) {
        throw "Extension staging failed: required module files are missing."
    }
}

Promote-ExtensionStage
Write-Host "Extension staged: $EXT_DIR" -ForegroundColor Green

# Clean up staging temp directories.
if (Test-Path $TEMP_EXTRACT) {
    Remove-Item -Path $TEMP_EXTRACT -Recurse -Force -ErrorAction SilentlyContinue
}
if (Test-Path $STAGE_EXT_DIR) {
    Remove-Item -Path $STAGE_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
}
if (Test-Path $BACKUP_EXT_DIR) {
    Remove-Item -Path $BACKUP_EXT_DIR -Recurse -Force -ErrorAction SilentlyContinue
}
Remove-Item -Path $TEMP_ZIP -ErrorAction SilentlyContinue

# ─────────────────────────────────────────────────────────────
# 7. Native Configuration (Go binary --install)
# ─────────────────────────────────────────────────────────────

Write-Host "Finalizing configuration..."
if (-not (Test-Path $INSTALL_BIN)) {
    Add-InstallWarning "Installer could not locate an executable to run for --install."
    Show-InstallWarnings
    throw "Gasoline binary install failed. See warning panel for manual recovery steps."
}

& $INSTALL_BIN --install
if ($LASTEXITCODE -ne 0) {
    Write-Host "  Native configuration returned an error." -ForegroundColor Yellow
    Write-Host "  The binary and extension were installed successfully." -ForegroundColor Yellow
    Write-Host "  You may need to manually configure your MCP clients." -ForegroundColor Yellow
    Write-Host "  Run: $INSTALL_BIN --install" -ForegroundColor Yellow
    # Don't throw here - core install succeeded, only config auto-detection had issues.
}

# ─────────────────────────────────────────────────────────────
# 8. Post-install health verification
# ─────────────────────────────────────────────────────────────

[void](Stop-GasolineServerProcesses)
Start-Sleep -Seconds 1

$healthOk = $false
try {
    $healthResponse = Invoke-RestMethod -Uri "http://127.0.0.1:7890/health" -TimeoutSec 5 -ErrorAction Stop
    if ($healthResponse.status -or $healthResponse) {
        $healthOk = $true
    }
} catch {
    # Server may still be starting; non-fatal.
}

if ($healthOk) {
    Write-Host "Server health check passed (port 7890)." -ForegroundColor Green
} else {
    Write-Host "  Server not yet responding on port 7890 - may still be starting." -ForegroundColor Yellow
    Write-Host "  Verify: Invoke-RestMethod http://127.0.0.1:7890/health" -ForegroundColor Yellow
}

# ─────────────────────────────────────────────────────────────
# 9. Register start-on-login
# ─────────────────────────────────────────────────────────────

$regPath = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$regName = "GasolineDaemon"
$regValue = "`"$CANONICAL_GASOLINE_BIN`" --daemon --port 7890"

try {
    Set-ItemProperty -Path $regPath -Name $regName -Value $regValue -ErrorAction Stop
    Write-Host "Registered to start on login (Windows Registry)." -ForegroundColor Green
} catch {
    Write-Host "  Could not register start-on-login automatically." -ForegroundColor Yellow
    Write-Host "  To register manually, run:" -ForegroundColor Yellow
    Write-Host "  Set-ItemProperty -Path '$regPath' -Name '$regName' -Value '$regValue'" -ForegroundColor Green
}

# ─────────────────────────────────────────────────────────────
# 10. PATH registration
# ─────────────────────────────────────────────────────────────

$binDirNorm = [System.IO.Path]::GetFullPath($BIN_DIR).TrimEnd("\")
$currentUserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$inPath = $false

if ($currentUserPath) {
    foreach ($entry in ($currentUserPath -split ";")) {
        if (-not [string]::IsNullOrWhiteSpace($entry)) {
            try {
                $entryNorm = [System.IO.Path]::GetFullPath($entry).TrimEnd("\")
                if ($entryNorm -eq $binDirNorm) {
                    $inPath = $true
                    break
                }
            } catch {
                # Skip invalid path entries.
            }
        }
    }
}

if (-not $inPath) {
    try {
        $newUserPath = if ($currentUserPath) { "$BIN_DIR;$currentUserPath" } else { $BIN_DIR }
        [Environment]::SetEnvironmentVariable("PATH", $newUserPath, "User")
        $env:PATH = "$BIN_DIR;$env:PATH"
        Write-Host "Added $BIN_DIR to user PATH." -ForegroundColor Green
        Write-Host "Restart your terminal for the change to take effect in new sessions." -ForegroundColor Yellow
    } catch {
        Write-Host "  Could not add $BIN_DIR to PATH automatically." -ForegroundColor Yellow
        Write-Host "  Add it manually:" -ForegroundColor Yellow
        Write-Host "  [Environment]::SetEnvironmentVariable('PATH', `"$BIN_DIR;`$env:PATH`", 'User')" -ForegroundColor Green
    }
}

# ─────────────────────────────────────────────────────────────
# 11. Final summary
# ─────────────────────────────────────────────────────────────

Show-InstallWarnings

Write-Host ""
if ($IS_UPGRADE -and $PREVIOUS_VERSION) {
    Write-Host "Gasoline upgraded: v$PREVIOUS_VERSION -> v$VERSION" -ForegroundColor Green
} else {
    Write-Host "Gasoline v$VERSION installed successfully." -ForegroundColor Green
}
Write-Host "Note: In-page xterm terminal support is currently disabled on Windows." -ForegroundColor Yellow
