---
feature: Enhanced CLI Configuration Management
---

# QA Plan: Enhanced CLI Configuration Management

> How to test the enhanced CLI features. Includes code-level testing + human UAT walkthrough.

## Testing Strategy

### Code Testing (Automated)

#### Unit Tests: Config File Operations

- [ ] `readConfigFile()` returns parsed JSON for valid file
- [ ] `readConfigFile()` returns error for non-existent file
- [ ] `readConfigFile()` returns error for invalid JSON with line number
- [ ] `readConfigFile()` handles empty mcpServers object
- [ ] `readConfigFile()` preserves other MCP server entries
- [ ] `writeConfigFile()` with `dryRun=true` doesn't actually write file
- [ ] `writeConfigFile()` with `dryRun=false` creates file with correct permissions
- [ ] `writeConfigFile()` overwrites existing gasoline entry only
- [ ] `writeConfigFile()` preserves formatting of other entries
- [ ] `validateMCPConfig()` rejects config without mcpServers
- [ ] `validateMCPConfig()` accepts valid MCP config structure

#### Unit Tests: Install Flow

- [ ] `executeInstall()` with `dryRun=true` returns diff without writing
- [ ] `executeInstall()` with `forAll=true` processes all candidates
- [ ] `executeInstall()` with `forAll=false` stops at first success (v5.2 behavior)
- [ ] `executeInstall()` merges env vars into config
- [ ] `executeInstall()` with empty env vars doesn't add env object
- [ ] `executeInstall()` with multiple env vars merges all
- [ ] `executeInstall()` rejects invalid env var format (no = sign)
- [ ] `executeInstall()` reports success for each updated tool
- [ ] `executeInstall()` reports errors for permission/write failures
- [ ] `executeInstall()` creates ~/.claude/claude.mcp.json if no configs exist
- [ ] `executeInstall()` idempotent: running twice produces same result

#### Unit Tests: Doctor Flow

- [ ] `runDiagnostics()` checks all 4 config locations
- [ ] `runDiagnostics()` reports "ok" for valid config with gasoline entry
- [ ] `runDiagnostics()` reports "error" for invalid JSON
- [ ] `runDiagnostics()` reports "error" for missing gasoline entry
- [ ] `runDiagnostics()` reports "error" for binary not found
- [ ] `runDiagnostics()` verifies binary is executable
- [ ] `runDiagnostics()` tests binary invocation with --version
- [ ] `runDiagnostics()` skips non-existent config files gracefully
- [ ] `runDiagnostics()` provides recovery suggestions for each issue
- [ ] Doctor output is human-readable with ✅/❌/⚠️ symbols

#### Unit Tests: Uninstall Flow

- [ ] `executeUninstall()` with `dryRun=true` shows what would be removed
- [ ] `executeUninstall()` removes gasoline entry cleanly
- [ ] `executeUninstall()` preserves other MCP servers
- [ ] `executeUninstall()` handles gasoline not being in config gracefully
- [ ] `executeUninstall()` reports count of tools from which gasoline was removed
- [ ] Uninstall produces valid JSON in remaining config

#### Unit Tests: CLI Argument Parsing

- [ ] `--dry-run` flag recognized for install/uninstall
- [ ] `--for-all` flag recognized for install
- [ ] `--env KEY=VALUE` parsed correctly; multiple `--env` supported
- [ ] `--verbose` flag enables debug logging
- [ ] Invalid flag combinations rejected with helpful error
- [ ] Help text updated with all new commands

#### Unit Tests: Error Handling

- [ ] Permission denied on file write shows recovery suggestion
- [ ] Invalid JSON in existing config shows line number and suggestion
- [ ] Binary not found shows installation instructions
- [ ] Directory doesn't exist creates parent directories
- [ ] File locked/in-use shows helpful error message
- [ ] All errors include next-step suggestions

---

### Integration Tests

#### Install + Doctor Integration

- [ ] Run `--install` then `--doctor` shows gasoline as configured
- [ ] Run `--install --for-all` then `--doctor` shows all tools as configured
- [ ] Run `--install --env VAR=val` then read config file shows env object

#### Uninstall + Doctor Integration

- [ ] Run `--install`, then `--uninstall`, then `--doctor` shows unconfigured
- [ ] Uninstall preserves other MCP servers

#### Dry-Run Verification

- [ ] `--install --dry-run` followed by `--install` without dry-run produces same result
- [ ] `--uninstall --dry-run` shows same entries that actual `--uninstall` removes

#### File System Scenarios

- [ ] Works when config files don't exist (creates them)
- [ ] Works when config files exist but are empty
- [ ] Works when config files have other MCP servers
- [ ] Works with both absolute and relative home paths
- [ ] Handles symlinks in config directories

---

### Security/Compliance Testing

#### Data Leak Tests

- [ ] Config files contain only gasoline entry and env vars
- [ ] Auth headers/tokens not captured in config
- [ ] User home path not exposed in error messages

#### Permission Tests

- [ ] Cannot write to read-only config directory (error message)
- [ ] Config file has appropriate read permissions for user
- [ ] Respects umask when creating new files
- [ ] Works with chmod 700 and chmod 755 homes

#### JSON Injection Tests

- [ ] Env var keys validated (no null bytes, control chars)
- [ ] Env var values safely escaped in JSON
- [ ] Config cannot be corrupted by malformed --env input
- [ ] Multiple writes don't cause data loss

#### Path Traversal Tests

- [ ] Cannot use `--env` to inject shell commands
- [ ] Cannot use paths to escape home directory
- [ ] symlink attacks mitigated with path.resolve()

---

## Human UAT Walkthrough

### Estimated Total Time: ~1 hour for all scenarios (46 minutes execution + 14 minutes setup/cleanup)

### Scenario 1: New User Safe Installation
#### Time: ~7 minutes

**Goal**: Verify a user can safely preview before installing.

**Setup**:
1. Backup any existing ~/.claude/claude.mcp.json
2. Delete ~/.claude, ~/.vscode, ~/.cursor, ~/.codeium directories (reset state)
3. Fresh gasoline-mcp install (npm install -g gasoline-mcp@latest)

**Steps**:
- [ ] Run `gasoline-mcp --config`
  - **Expected**: Shows MCP config template and 4 tool locations
  - **Verification**: JSON structure valid, all 4 paths shown

- [ ] Run `gasoline-mcp --install --dry-run`
  - **Expected**: Shows "Would create: ~/.claude/claude.mcp.json" + JSON diff
  - **Verification**: No actual file created; output shows exact changes

- [ ] Check file doesn't exist: `ls ~/.claude/claude.mcp.json`
  - **Expected**: File not found (command returns error)
  - **Verification**: Dry-run truly didn't write

- [ ] Run `gasoline-mcp --install`
  - **Expected**: "✅ Created: ~/.claude/claude.mcp.json"
  - **Verification**: File now exists

- [ ] Read created file: `cat ~/.claude/claude.mcp.json`
  - **Expected**: Valid JSON with gasoline entry
  - **Verification**: Structure matches --dry-run output

**Result**: ✅ PASS (user can preview before committing)

---

### Scenario 2: Multi-Tool Installation
#### Time: ~8 minutes

**Goal**: Verify user can install to all 4 tools in one command.

**Setup**:
1. Create dummy config files (or use real ones if available):
   - `~/.claude/claude.mcp.json` with other MCP servers
   - `~/.vscode/claude.mcp.json` with other MCP servers
   - `~/.cursor/mcp.json` empty
   - `~/.codeium/mcp.json` missing (doesn't exist)

**Steps**:
- [ ] Run `gasoline-mcp --install --for-all`
  - **Expected**:
    ```
    ✅ Claude Desktop: Updated ~/.claude/claude.mcp.json
    ✅ VSCode: Updated ~/.vscode/claude.mcp.json
    ✅ Cursor: Created ~/.cursor/mcp.json
    ℹ️  Codeium: No config found (run install separately if needed)
    ```
  - **Verification**: Each tool's config modified/created as expected

- [ ] Read each config file and verify:
  - gasoline entry present
  - Other MCP servers preserved
  - Valid JSON syntax

**Result**: ✅ PASS (multi-tool install works)

---

### Scenario 3: Environment Variables
#### Time: ~6 minutes

**Goal**: Verify env vars properly injected into config.

**Setup**:
1. Fresh config (delete previous)

**Steps**:
- [ ] Run:
  ```bash
  gasoline-mcp --install --env GASOLINE_SERVER=http://custom:7890 --env DEBUG=1
  ```
  - **Expected**: "✅ Created: ~/.claude/claude.mcp.json"
  - **Verification**: File created

- [ ] Read file: `cat ~/.claude/claude.mcp.json | grep -A 3 '"env"'`
  - **Expected**:
    ```json
    "env": {
      "GASOLINE_SERVER": "http://custom:7890",
      "DEBUG": "1"
    }
    ```
  - **Verification**: Both env vars present and properly formatted

- [ ] Run again with different env var:
  ```bash
  gasoline-mcp --install --env LOG_LEVEL=debug
  ```
  - **Expected**: Merged with existing env vars (not replaced)
  - **Verification**: All 3 env vars still present (GASOLINE_SERVER, DEBUG, LOG_LEVEL)

**Result**: ✅ PASS (env vars work correctly)

---

### Scenario 4: Doctor Diagnostics
#### Time: ~10 minutes

**Goal**: Verify --doctor provides useful diagnostics.

**Setup**:
1. Create test scenario: some tools configured, some not
   - `~/.claude/claude.mcp.json` - valid with gasoline
   - `~/.vscode/claude.mcp.json` - invalid JSON (missing quote)
   - `~/.cursor/mcp.json` - doesn't exist
   - `~/.codeium/mcp.json` - valid but no gasoline entry

**Steps**:
- [ ] Run `gasoline-mcp --doctor`
  - **Expected**:
    ```
    Gasoline MCP Diagnostic Report

    ✅ Claude Desktop
       ~/.claude/claude.mcp.json - configured and healthy

    ❌ VSCode
       ~/.vscode/claude.mcp.json - invalid JSON at line 5
       Suggestion: Fix syntax error or run: gasoline-mcp --install

    ℹ️  Cursor
       ~/.cursor/mcp.json - not configured
       Suggestion: Run: gasoline-mcp --install --for-all

    ⚠️  Codeium
       ~/.codeium/mcp.json - gasoline entry missing
       Suggestion: Run: gasoline-mcp --install --for-all

    Summary: 1 tool healthy, 1 needs repair, 2 not configured
    ```
  - **Verification**: Correct status for each tool; suggestions are actionable

- [ ] Follow one suggestion: fix VSCode JSON and re-run doctor
  - **Expected**: VSCode status changes to ✅
  - **Verification**: Doctor properly detects repair

**Result**: ✅ PASS (doctor provides useful diagnostics)

---

### Scenario 5: Uninstall
#### Time: ~5 minutes

**Goal**: Verify clean uninstall without losing other MCP servers.

**Setup**:
1. Create config with multiple MCP servers:
   ```json
   {
     "mcpServers": {
       "gasoline": { "command": "gasoline-mcp", "args": [], "env": {} },
       "other-tool": { "command": "other-tool", "args": [] }
     }
   }
   ```

**Steps**:
- [ ] Run `gasoline-mcp --uninstall --dry-run`
  - **Expected**: Shows "Would remove: gasoline from ~/.claude/claude.mcp.json"
  - **Verification**: other-tool entry shown as preserved

- [ ] Run `gasoline-mcp --uninstall`
  - **Expected**: "✅ Removed from 1 tool"
  - **Verification**: gasoline entry removed

- [ ] Read config file:
  - **Expected**: other-tool entry still present; gasoline gone
  - **Verification**: Config valid JSON with only other-tool

- [ ] Run `gasoline-mcp --doctor`
  - **Expected**: Shows unconfigured
  - **Verification**: Doctor confirms removal

**Result**: ✅ PASS (uninstall works cleanly)

---

### Scenario 6: Error Recovery - Invalid JSON
#### Time: ~7 minutes

**Goal**: Verify helpful error when config has invalid JSON.

**Setup**:
1. Create broken config: `~/.claude/claude.mcp.json` with missing quote:
   ```json
   {
     "mcpServers": {
       "test": "value  // missing quote
     }
   }
   ```

**Steps**:
- [ ] Run `gasoline-mcp --install`
  - **Expected**: Error message:
    ```
    ❌ Error: Invalid JSON in ~/.claude/claude.mcp.json
    Line 4: Unexpected token '/'

    Next steps:
    1. Fix the JSON syntax error
    2. Run: gasoline-mcp --doctor
    3. Or: gasoline-mcp --install (will overwrite)
    ```
  - **Verification**: Error message clear and actionable

- [ ] Fix the JSON file
- [ ] Run `gasoline-mcp --install` again
  - **Expected**: Now succeeds
  - **Verification**: Error recovery works

**Result**: ✅ PASS (error messages are helpful)

---

### Scenario 7: Backward Compatibility
#### Time: ~5 minutes

**Goal**: Verify v5.2 commands still work unchanged.

**Setup**:
1. Clean state

**Steps**:
- [ ] Run `gasoline-mcp --config`
  - **Expected**: Same output as v5.2.0
  - **Verification**: No changes to existing command

- [ ] Run `gasoline-mcp --install`
  - **Expected**: Same behavior as v5.2.0 (install to first matching config)
  - **Verification**: Installs to ~/.claude only (not --for-all behavior)

- [ ] Run `gasoline-mcp --help`
  - **Expected**: Help shows all commands (old + new)
  - **Verification**: New commands documented

**Result**: ✅ PASS (backward compatible)

---

## Regression Testing

### Backward Compatibility (v5.2 → v5.3)

#### Critical: Verify all v5.2 CLI commands still work unchanged

- [ ] `gasoline-mcp --config`
  - Expected: Shows config template and 4 tool locations (unchanged from v5.2)
  - Verification: Compare output with v5.2.0 release

- [ ] `gasoline-mcp --install` (without flags)
  - Expected: Installs to FIRST matching config only (not --for-all behavior)
  - Verification: Confirm only one tool updated (e.g., Claude Desktop only)
  - Regression: Don't accidentally update all tools when user expects just first

- [ ] `gasoline-mcp --help`
  - Expected: Lists all commands (old + new)
  - Verification: Shows --config, --install, --help, --doctor, --uninstall, --for-all, --env, --dry-run, --verbose

- [ ] Binary path resolution
  - Expected: findBinary() still works (unchanged)
  - Verification: gasoline-mcp finds binary correctly on macOS/Linux/Windows

- [ ] Unsupported platform error
  - Expected: Shows clear error for unsupported OS (unchanged)
  - Verification: Error message format and text unchanged from v5.2

- [ ] Other npm packages install correctly
  - Expected: NPM ecosystem not affected by gasoline-mcp changes
  - Verification: `npm install` and `npm ci` work normally

### Concurrent Operations

- [ ] Two simultaneous `gasoline-mcp --install` processes
  - Setup: Start two install processes to same config file in parallel
  - Expected: Both complete without corruption
  - Verification: Config file has valid JSON and gasoline entry present
  - Implementation: Use atomic writes (temp file + rename) to prevent race conditions

### Integration with gasoline Binary

- [ ] `gasoline-mcp` entry in config correctly points to binary
- [ ] Binary path resolution works across macOS, Linux, Windows paths
- [ ] Extension still loads MCP correctly with new config format
- [ ] gasoline binary can be invoked from config (test with --version)

---

## Performance/Load Testing

- [ ] Doctor completes in < 1 second (4 files)
- [ ] Install with --for-all completes in < 1 second (4 writes)
- [ ] Doctor with --verbose doesn't add significant time
- [ ] Uninstall completes in < 1 second (4 files)

---

## Test Files to Create

In `tests/extension/`:

1. **config-file-utils.test.js**
   - Test readConfigFile, writeConfigFile, validateMCPConfig
   - Test dryRun mode

2. **install-flow.test.js**
   - Test executeInstall with all options
   - Test --env parsing
   - Test --for-all behavior
   - Test idempotence

3. **doctor-flow.test.js**
   - Test runDiagnostics
   - Test all status types (ok, error, warn)
   - Test binary invocation check

4. **uninstall-flow.test.js**
   - Test executeUninstall
   - Test preservation of other MCP servers
   - Test dryRun mode

5. **cli-integration.test.js**
   - Test command parsing
   - Test argument combinations
   - Test error scenarios
