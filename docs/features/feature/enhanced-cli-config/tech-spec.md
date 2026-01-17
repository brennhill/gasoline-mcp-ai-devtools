---
feature: Enhanced CLI Configuration Management
status: proposed
---

# Tech Spec: Enhanced CLI Configuration Management

> Plain language only. Implementation guide for developers.

## Architecture Overview

The enhancement extends **both wrappers** with the same CLI commands:

### NPM Wrapper (`npm/gasoline-mcp/bin/gasoline-mcp`)
Node.js CLI entry point. Current structure:
- **Lines 32-68**: `findBinary()` - Locates gasoline binary on disk
- **Lines 70-81**: `generateMCPConfig()` - Builds MCP config object
- **Lines 83-102**: `showConfigCommand()` - Displays config template + locations
- **Lines 104-151**: `installCommand()` - Writes config to first matching location
- **Lines 153-169**: Command routing (--config, --install, --help)

### PyPI Wrapper (`pypi/gasoline-mcp/gasoline_mcp/__main__.py`)
Python CLI entry point. Current structure:
- **Lines 1-12**: Entry point that calls `run()` function
- Related: `pypi/gasoline-mcp/gasoline_mcp/platform.py` has `get_binary_path()` and `run()` functions

### New additions (both wrappers)
- Shared utility functions for config file operations (JSON read/write/validate)
- Command handlers for: `--dry-run`, `--doctor`, `--uninstall`, `--for-all`, `--env`
- Refactored install logic to support `--dry-run` and `--for-all`
- New error handling with recovery suggestions
- Verbose logging support
- **Feature parity**: Both wrappers must behave identically

## Implementation: NPM vs. Python

### NPM (Node.js)
- **Language**: JavaScript (Node.js stdlib)
- **Location**: `npm/gasoline-mcp/bin/gasoline-mcp`
- **Modules**: `lib/config.js`, `lib/doctor.js`, `lib/install.js`, `lib/uninstall.js`, `lib/output.js`, `lib/errors.js`
- **File I/O**: `fs` module
- **JSON**: Built-in `JSON.parse()` and `JSON.stringify()`

### Python
- **Language**: Python 3 (stdlib only)
- **Location**: `pypi/gasoline-mcp/gasoline_mcp/__main__.py` (entry point) + new module files
- **Modules**:
  - `pypi/gasoline-mcp/gasoline_mcp/config.py` (config utilities)
  - `pypi/gasoline-mcp/gasoline_mcp/doctor.py` (diagnostics)
  - `pypi/gasoline-mcp/gasoline_mcp/install.py` (install logic)
  - `pypi/gasoline-mcp/gasoline_mcp/uninstall.py` (uninstall logic)
  - `pypi/gasoline-mcp/gasoline_mcp/output.py` (formatters)
  - `pypi/gasoline-mcp/gasoline_mcp/errors.py` (error classes)
- **File I/O**: `pathlib.Path` or `os`/`open()`
- **JSON**: Built-in `json` module
- **Entry Point**: Updated `__main__.py` to parse CLI args and route commands

### Feature Parity Requirement
Both wrappers must:
- Accept same command flags (`--config`, `--install`, `--doctor`, `--uninstall`, `--dry-run`, `--for-all`, `--env`, `--verbose`, `--help`)
- Produce identical output (same messages, same emojis, same error text)
- Behave identically (same logic, same edge case handling)
- Use same config file locations (hardcoded paths, not user input)

---

## Key Components

### 1. **Config File Utilities**
- `getConfigCandidates()`: Returns array of candidate config file paths (Claude, VSCode, Cursor, Codeium)
- `readConfigFile(path)`: Safely reads and parses JSON, returns `{valid: bool, data: obj, error: string}`
- `writeConfigFile(path, data, dryRun)`: Writes JSON to file; if `dryRun=true`, returns what would be written without actually writing
- `validateMCPConfig(data)`: Ensures config has `mcpServers` object
- `mergeGassolineConfig(existing, gassolineEntry, envVars)`: Merges new gasoline entry into existing mcpServers

### 2. **Diagnostic Engine**
- `runDiagnostics()`: Iterates through all config candidates, tests each one:
  - File exists and readable
  - JSON parses without error
  - gasoline entry present in mcpServers
  - gasoline command path resolves
  - Binary is executable
  - Returns report object with health status for each tool

### 3. **Installation Engine (Refactored)**
- `executeInstall(options)`: Main install logic supporting:
  - `{dryRun: bool}` - Preview mode
  - `{forAll: bool}` - Install to all detected tools
  - `{envVars: {KEY: VALUE}}` - Environment variables to inject
  - Returns `{success: bool, updated: [paths], errors: [details], output: [messages]}`

### 4. **Uninstall Engine**
- `executeUninstall(options)`: Removes gasoline from configs:
  - `{dryRun: bool}` - Preview mode
  - Iterates all config files
  - Removes gasoline entry from each mcpServers
  - Preserves other MCP servers
  - Returns success report

### 5. **Output Formatters**
- `formatConfigDiff(before, after)`: Shows JSON before/after for dry-run
- `formatDiagnosticsReport(report)`: Pretty-prints doctor output with ✅/❌/⚠️  symbols
- `formatInstallResult(result)`: Shows which tools updated, any errors
- `formatErrorWithRecovery(error, context)`: Error message + next steps

## Data Flows

### Install Flow (with --dry-run support)
```
Parse CLI args (--install, --dry-run, --for-all, --env)
  ↓
Build install options object {dryRun, forAll, envVars}
  ↓
Execute install:
  - Get config candidates
  - For each candidate:
    - Read existing config
    - Merge gasoline entry + env vars
    - If !dryRun: write file, track success
    - If dryRun: show what would be written
  ↓
Report results (files updated, errors, diffs)
```

### Doctor Flow
```
Parse CLI args (--doctor, --verbose)
  ↓
For each config candidate:
  - Check file exists
  - Validate JSON
  - Verify gasoline entry
  - Check binary path resolves
  - Test binary executable
  ↓
Compile diagnostic report:
  - Tool name → health status
  - Issues found → recovery suggestions
  ↓
Format and display report
```

### Uninstall Flow
```
Parse CLI args (--uninstall, --dry-run)
  ↓
If dryRun: show preview, exit
  ↓
For each config candidate:
  - Read config
  - Remove gasoline from mcpServers
  - Write modified config (if changed)
  - Track removed count
  ↓
Report results
```

## Implementation Strategy

### Phase 1: Refactor Current Code
1. Extract `generateMCPConfig()` to include `envVars` parameter
2. Extract config file I/O into `readConfigFile()` and `writeConfigFile()` utilities
3. Update `installCommand()` to call new utilities (no behavior change, just refactor)
4. Add `--verbose` flag support to existing commands

### Phase 2: Implement --dry-run
1. Add `--dry-run` flag parsing to CLI args
2. Pass `dryRun: true` through install flow
3. Implement `writeConfigFile(..., dryRun=true)` to return diff without writing
4. Display JSON diff using simple before/after format

### Phase 3: Implement --doctor
1. Create `runDiagnostics()` function checking each candidate
2. Create diagnostic report object: `{tool: string, status: 'ok'|'error'|'warn', issues: [], suggestions: []}`
3. For each candidate, run checks:
   - `fs.existsSync(path)` for file existence
   - `JSON.parse()` for JSON validity
   - Check `data.mcpServers.gasoline` exists
   - Check binary path exists and is executable
4. Format output with emojis (✅/❌/⚠️) and recovery suggestions

### Phase 4: Implement --for-all
1. Refactor `installCommand()` to support `forAll` option
2. Instead of breaking at first success, continue through all candidates
3. Collect results from all updated files
4. Report all successful updates and any errors

### Phase 5: Implement --env
1. Parse `--env KEY=VALUE` arguments into array
2. Validate format (contains =, non-empty key and value)
3. Merge into config's `env` object
4. Support multiple `--env` flags

### Phase 6: Implement --uninstall
1. Create `executeUninstall()` function
2. For each candidate:
   - Read config
   - Delete `data.mcpServers.gasoline`
   - Write back (if changed)
3. Report removed count and tool names

### Phase 7: Improve Error Messages
1. Wrap all file operations in try-catch
2. For each error type, provide context and next steps:
   - **Permission denied** → "Run with `sudo` or check file permissions"
   - **Invalid JSON** → "Config has syntax error at line X, use `--doctor` to see issues"
   - **Binary not found** → "Reinstall: `npm install -g gasoline-mcp`"
3. Test error paths with deliberately broken configs

## Edge Cases & Assumptions

| Edge Case | Handling |
|-----------|----------|
| **No config files exist** | `--install` creates ~/.claude/claude.mcp.json; `--doctor` reports "not configured" |
| **Config file has invalid JSON** | `--doctor` reports syntax error; `--install` refuses to write (don't corrupt) |
| **User lacks file permissions** | Error message with `sudo` suggestion; `--doctor` reports permission issue |
| **gasoline binary not found** | All commands report binary path issue; suggest reinstall |
| **File partially written (crash)** | `--dry-run` prevents this; next `--install` attempt merges cleanly |
| **Multiple gasoline entries** | Merge/update first found; warn if duplicates detected |
| **Other MCP servers in config** | Preserve them; only touch gasoline entry |
| **Config file deleted during operation** | No-op, file doesn't exist; report in results |
| **User runs --env without --install** | Show error: "--env only works with --install" |
| **--for-all with --dry-run** | Show diffs for all files, don't write any |

**Assumptions**:
- Config files are valid JSON when user didn't edit them
- User's home directory is writable (or at least one config location is)
- gasoline binary is installed and on PATH or findable via node_modules
- AI tools (Claude, VSCode) may not be running when --doctor executes

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| **Corrupt config on write failure** | Always `--dry-run` first; use atomic writes (write temp file, rename) |
| **Merge conflicts between gasoline + user edits** | Read existing config, merge gasoline entry into mcpServers, preserve other keys |
| **User loses other MCP server configs** | Test with multi-server configs; never overwrite entire mcpServers |
| **--doctor false negatives** | Can only verify local state; cannot test actual MCP connection. Document limitation. |
| **--uninstall removes wrong data** | Test with configs having multiple MCP servers; verify only gasoline removed |
| **Breaking change from v5.2 CLI** | Keep v5.2 commands working: --config, --install, --help all unchanged |

## Dependencies

- **Existing**: Node.js `fs`, `path`, `os` modules (already used)
- **Existing**: `execFileSync` for binary lookup (already used)
- **New**: `JSON.stringify(..., null, 2)` for formatting (already used)
- **New**: No new npm dependencies; stdlib only

## Platform-Specific Behavior

### Windows Path Handling
- All config paths use `os.homedir()` which returns correct path on Windows
- Example: `os.homedir()` returns `C:\Users\YourName` on Windows automatically
- `path.join()` handles platform-specific separators (\ on Windows, / on Unix)
- **No special %APPDATA% logic needed** — `os.homedir()` already handles this correctly
- Config location on Windows: `C:\Users\{username}\.claude\claude.mcp.json`

### Symlink Behavior
- **Policy**: Follow symlinks and update the target file (standard Unix behavior)
- Implementation: Use `path.resolve()` to get canonical path and prevent traversal attacks
- Documented for users: "Config files will be followed if symlinked"
- Test: Verify behavior with symlinked config directories

## Performance Considerations

- `readConfigFile()`: O(n) where n = config file size (typically < 10KB); acceptable
- `runDiagnostics()`: O(m) where m = number of config candidates (4 files); < 100ms
- `--install --for-all`: O(m) for all files; < 500ms total
- **Critical**: `--dry-run` must not write any files (test with file system mock)
- **File I/O**: Single I/O per file; no loops or polling

## Security Considerations

1. **JSON Injection**: Validate env var keys/values before merging into config
   - Reject keys with null bytes or control characters
   - Reject values if they contain shell metacharacters (document that users are responsible)

2. **File Permissions**:
   - Check file readable before reading
   - Check directory writable before writing
   - Don't change ownership or permissions
   - Respect umask when creating new files

3. **Path Traversal**:
   - All paths come from hardcoded candidate list (no user input for paths)
   - Use `path.resolve()` and `path.normalize()` to avoid symlink attacks

4. **Env Var Injection**:
   - Document that env vars are passed as-is to gasoline binary
   - Users responsible for safe values (no shell injection from our side)
   - Config stored in JSON; not vulnerable to YAML or shell parsing
   - **Caution**: Never store API keys or secrets in --env
   - Example: Store API key in `~/.gasoline/secrets` (mode 600), pass only path with `--env SECRETS_FILE=...`

5. **File Size Limits**:
   - Limit config file size to 1MB (prevents DoS from crafted configs)
   - Typical MCP config is < 1KB; 1MB is generous safety margin
   - Implementation: Check `if (stats.size > 1024 * 1024) throw new Error(...)`

6. **Private Keys**:
   - Config files stored in user's home directory (standard)
   - Warn in help text: don't store secrets in env vars; use config file permissions instead
   - Example: Store API key in ~/.gasoline/config instead of in --env

7. **Audit**:
   - Log all file modifications with timestamps when `--verbose`
   - Include diffs for transparency

## Data Model

```
Install Options:
{
  dryRun: boolean,
  forAll: boolean,
  envVars: {KEY: VALUE, ...},
  verbose: boolean
}

Diagnostic Report:
{
  tools: [
    {
      name: "Claude Desktop",
      path: "~/.claude/claude.mcp.json",
      status: "ok" | "error" | "warning",
      issues: ["gasoline entry missing", ...],
      suggestions: ["Run: gasoline-mcp --install", ...]
    },
    ...
  ],
  summary: "2 tools configured, 2 not configured"
}

Config Structure:
{
  mcpServers: {
    gasoline: {
      command: "gasoline-mcp",
      args: [],
      env: {
        KEY: "VALUE",
        ...
      }
    },
    otherTool: { ... }  // preserve
  }
}
```
