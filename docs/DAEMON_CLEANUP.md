---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Daemon Cleanup & Version Management

## Overview

Gasoline is a persistent daemon server. To ensure clean upgrades and prevent version conflicts, we've implemented comprehensive daemon cleanup and version validation.

## Issues Fixed (Feb 10, 2026)

### 🔴 Critical: v5.8.2 Pinned in optionalDependencies

**Problem:** The npm package's `optionalDependencies` were hard-coded to v5.8.2 instead of v6.0.0, causing users to receive the old binary even after upgrading.

**File:** `npm/gasoline-mcp/package.json`

**Fix:** Updated all optionalDependencies to `6.0.0`:
```json
"optionalDependencies": {
  "@brennhill/gasoline-darwin-arm64": "6.0.0",
  "@brennhill/gasoline-darwin-x64": "6.0.0",
  "@brennhill/gasoline-linux-arm64": "6.0.0",
  "@brennhill/gasoline-linux-x64": "6.0.0",
  "@brennhill/gasoline-win32-x64": "6.0.0"
}
```

### 🟡 Missing Daemon Cleanup on Install

**Problem:** The `preinstall` hook only tried to uninstall the old npm package but didn't kill the running daemon, leaving old processes alive.

**Fix:** Enhanced `preinstall` hook to kill all running gasoline processes:
```json
"preinstall": "node -e \"const cp = require('child_process'); try { cp.execSync('npm uninstall -g gasoline-mcp', {stdio:'ignore'}); } catch(e) {} try { const cmd = process.platform === 'win32' ? 'taskkill /F /IM gasoline.exe 2>nul || true' : 'pkill -9 gasoline 2>/dev/null || true'; cp.execSync(cmd, {stdio:'ignore', shell:true}); } catch(e) {}\""
```

Also updated `preuninstall` to use the same cross-platform cleanup.

### 🟡 No Force Cleanup for Users

**Problem:** Users had no way to force-kill old daemons if installation failed or if they needed to manually recover.

**Fixes:**
1. Added `--force` flag to Go binary
2. Created user-accessible cleanup script
3. Version validation in CI/CD

## Usage

### Option 1: Automatic Cleanup During Install (Recommended)

When you run `npm install -g gasoline-mcp`, the `preinstall` hook automatically:
- Uninstalls old global version
- Kills all running daemons
- Cleans up stale PID files

```bash
npm install -g gasoline-mcp@latest
```

No additional action needed!

### Option 2: Manual Cleanup Before Install

If you want to ensure daemons are cleaned up before installing:

```bash
# Using the Go binary
gasoline --force

# Or using the shell script
./scripts/clean-old-daemons.sh
```

### Option 3: Stop Specific Server

To stop only the server on a specific port:

```bash
gasoline --stop --port 7890
```

## Version Synchronization

Since npm's `package.json` doesn't support variable interpolution, we use:

1. **Manual sync via Makefile:** The `sync-version` target updates all version references
   ```bash
   make sync-version
   ```

2. **Validation:** The `validate-deps-versions` check ensures optionalDependencies match the main version
   ```bash
   make validate-deps-versions
   ```

3. **CI Integration:** The quality gate includes version validation
   ```bash
   make quality-gate
   ```

### How It Works

**File:** `npm/gasoline-mcp/lib/validate-versions.js`

This script checks that all optionalDependencies match the main package version. If they don't match:

```
❌ Version mismatch in optionalDependencies:

  @brennhill/gasoline-darwin-arm64: "5.8.2" (expected "6.0.0")
  @brennhill/gasoline-darwin-x64: "5.8.2" (expected "6.0.0")
  ...

Fix by running: make sync-version
```

## Implementation Details

### Go Binary: `--force` Flag

**File:** `cmd/dev-console/main.go` and `cmd/dev-console/main_connection.go`

The `--force` flag invokes `runForceCleanup()` which:

1. **Finds all gasoline processes** using:
   - `lsof -c gasoline` (Unix-like systems)
   - `taskkill` (Windows)

2. **Kills them gracefully:**
   - Sends SIGTERM first (graceful shutdown)
   - Waits 100ms for process to exit
   - Sends SIGKILL if needed

3. **Cleans up PID files** for ports 7890-7910

4. **Logs to lifecycle events** for debugging

### Package Hooks

**Files:**
- `npm/gasoline-mcp/package.json` - preinstall/preuninstall scripts

**preinstall:**
- Runs before npm install
- Uninstalls old global version
- Kills all running daemons
- Cross-platform (macOS/Linux/Windows)

**preuninstall:**
- Runs before npm uninstall
- Kills all running daemons

### Shell Script: `clean-old-daemons.sh`

**File:** `scripts/clean-old-daemons.sh`

User-accessible cleanup script for manual daemon cleanup.

Platform detection:
- **macOS:** Uses `lsof` + `pkill`
- **Linux:** Uses `pgrep` + `pkill`
- **Windows:** Uses `taskkill`

## Testing

### Verify No Version Drift

```bash
make validate-deps-versions
```

Should output:
```
✓ All optionalDependencies match version 6.0.0
```

### Verify Daemon Cleanup Works

```bash
# Start a daemon
gasoline

# In another terminal, force cleanup
gasoline --force

# Verify it's gone
lsof -ti :7890  # Should return nothing
```

### Verify Install Process

```bash
# Simulate old version
npm install -g gasoline-mcp@5.8.2

# Verify 5.8.2 is running
gasoline --version  # Should show 5.8.2

# Upgrade to 6.0.0
npm install -g gasoline-mcp@latest

# Verify 6.0.0 is running and daemon was cleaned up
gasoline --version  # Should show 6.0.0
```

## Troubleshooting

### "Still have v5.8 after upgrade"

This shouldn't happen anymore, but if it does:

1. **Force cleanup:**
   ```bash
   gasoline --force
   ```

2. **Uninstall completely:**
   ```bash
   npm uninstall -g gasoline-mcp
   ```

3. **Reinstall:**
   ```bash
   npm install -g gasoline-mcp@latest
   ```

### "Multiple gasoline processes still running"

Check for zombie processes:

```bash
# Find all gasoline processes
ps aux | grep gasoline

# Kill everything
gasoline --force

# Or manually
pkill -9 gasoline
```

### "PID file stale, can't start new daemon"

```bash
# Clean up stale PID files
rm -f ~/.gasoline-*.pid

# Then start fresh
gasoline
```

## Release Checklist

Before releasing a new version:

1. **Update VERSION file** to new semantic version
2. **Run sync-version:**
   ```bash
   make sync-version
   ```
3. **Validate versions match:**
   ```bash
   make validate-deps-versions
   ```
4. **Run full quality gate:**
   ```bash
   make quality-gate
   ```
5. **Commit and tag:**
   ```bash
   git commit -am "v{VERSION}"
   git tag v{VERSION}
   ```
6. **Push and release**

The sync-version Makefile target will automatically update:
- `npm/gasoline-mcp/package.json` - optionalDependencies
- `npm/*/package.json` - all platform packages
- `pypi/*/pyproject.toml` - all PyPI packages
- And more (see Makefile sync-version target)

## Related Files

- `npm/gasoline-mcp/package.json` - Main package with daemon cleanup hooks
- `npm/gasoline-mcp/lib/validate-versions.js` - Version validation script
- `cmd/dev-console/main.go` - `--force` flag definition
- `cmd/dev-console/main_connection.go` - `runForceCleanup()` implementation
- `scripts/clean-old-daemons.sh` - User-friendly cleanup script
- `Makefile` - sync-version and validate-deps-versions targets
