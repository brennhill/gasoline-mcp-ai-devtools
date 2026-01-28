---
feature: Enhanced CLI Configuration Management
status: proposed
tool: configure
mode: cli
version: v5.3
---

# Product Spec: Enhanced CLI Configuration Management

## Problem Statement

Users installing Gasoline MCP need to:
1. Configure it across multiple AI assistant tools (Claude Desktop, VSCode, Cursor, Codeium)
2. Verify the configuration is correct before committing changes
3. Diagnose connection/installation issues when things go wrong
4. Manage environment variables for different deployment scenarios
5. Uninstall Gasoline cleanly if needed

The current implementation (`--config`, `--install`) is functional but incomplete:
- **No preview mode**: Users can't see what changes will be made before committing
- **No diagnostics**: When installation fails, users have no way to troubleshoot
- **Single-tool support**: Only first matching config file gets updated; other tools left out
- **No verification**: No way to confirm the config is actually working
- **No environment variables**: Can't inject custom env vars (e.g., API keys, server URLs) into MCP config
- **Limited error recovery**: Confusing error messages when JSON is malformed or permissions denied

## Solution

Enhance the gasoline-mcp CLI with five new features:

### 1. **Dry-Run Mode** (`--dry-run`)
Preview exactly what changes will be made without actually writing files. Shows:
- Which config file would be modified/created
- Before/after JSON diff
- Estimated file permissions
- Safe for checking before real installation

### 2. **Doctor Command** (`--doctor`)
Diagnostic tool that verifies local configuration:
- Checks all 4 config file locations and their JSON validity
- Verifies gasoline entry is present in each config
- Confirms gasoline binary path resolves and is executable
- Tests basic binary invocation (runs `gasoline --version`)
- Reports which tools are configured, which aren't
- Provides actionable repair suggestions for fixable issues
- **Note**: Cannot test actual MCP connection without AI agent running (VSCode, Claude, etc.)

### 3. **Multi-Tool Install** (`--install --for-all`)
Instead of stopping at first config, update ALL detected config files:
- Find and update Claude Desktop config
- Find and update VSCode config
- Find and update Cursor config
- Find and update Codeium config
- Report which tools were updated

### 4. **Environment Variables** (`--install --env KEY=VALUE`)
Allow injecting environment variables into MCP config:
- `--env GASOLINE_SERVER=http://localhost:7890`
- `--env DEBUG=1`
- Multiple `--env` flags supported
- Saved in `env` object of MCP config
- Useful for custom server URLs, debug modes, offline deployments

### 5. **Uninstall Command** (`--uninstall`)
Remove Gasoline from AI assistant configs:
- Find gasoline entry in all config files
- Remove it cleanly (preserve other MCP servers)
- Confirm removal before writing
- Report which tools were cleaned up

## Requirements

1. **Usability**: All commands must have helpful output and clear error messages
2. **Safety**: Dry-run must work correctly; no actual changes on `--dry-run`
3. **Idempotence**: Multiple runs with same args produce same result
4. **Backward Compatible**: Existing `--config`, `--install`, `--help` must still work
5. **Composable**: `--dry-run` works with `--install`, `--uninstall`, `--doctor`
6. **Verbose**: `--verbose` flag shows detailed operation logs
7. **Recovery**: Commands provide next-step suggestions on failure

## Out of Scope

- GUI configuration tool (CLI only)
- Automatic config migration between versions
- Remote config management (local files only)
- Automatic version upgrades
- Configuration encryption/secrets management (users responsible for env vars)

## Success Criteria

1. ✅ `--dry-run` shows exact changes without writing files
2. ✅ `--doctor` diagnoses 90%+ of common config issues
3. ✅ `--for-all` successfully updates all 4 tool configs in one run
4. ✅ `--env KEY=VALUE` properly saves env vars to config
5. ✅ `--uninstall` cleanly removes gasoline from all configs
6. ✅ All commands provide actionable error messages with recovery steps
7. ✅ 100% backward compatibility with v5.2.0 CLI behavior
8. ✅ All commands covered by automated tests (unit + integration)
9. ✅ Help text and examples available for each command

## User Workflows

### Workflow 1: Safe Installation
```
User wants to try Gasoline on their VSCode but isn't sure what will change

1. User runs:   gasoline-mcp --config
2. Sees:        Config template + 4 tool locations
3. User runs:   gasoline-mcp --install --dry-run
4. Sees:        Exact JSON changes, which file would be modified
5. User runs:   gasoline-mcp --install
6. Sees:        ✅ Updated: ~/.claude/claude.mcp.json
7. User runs:   gasoline-mcp --doctor
8. Sees:        ✅ Gasoline configured in Claude Desktop
                ❌ Not configured in VSCode, Cursor, Codeium
```

### Workflow 2: Multi-Tool Install
```
User wants Gasoline available in all AI assistants

1. User runs:   gasoline-mcp --install --for-all
2. Sees:        ✅ Claude Desktop: Updated
                ✅ VSCode: Updated
                ✅ Cursor: Updated
                ✅ Codeium: Updated
3. User runs:   gasoline-mcp --doctor
4. Sees:        ✅ All 4 tools configured and connected
```

### Workflow 3: Environment Variables
```
User needs to point Gasoline at custom server URL

1. User runs:   gasoline-mcp --install --env GASOLINE_SERVER=http://192.168.1.100:7890
2. Sees:        ✅ Updated: ~/.claude/claude.mcp.json
3. Config has:  "env": { "GASOLINE_SERVER": "http://192.168.1.100:7890" }
```

### Workflow 4: Troubleshooting
```
User says "It's not working, can you check?"

1. User runs:   gasoline-mcp --doctor
2. Sees:
                Gasoline MCP Diagnostic Report

                ✅ Claude Desktop
                   ~/.claude/claude.mcp.json - Configured and ready

                ❌ VSCode
                   ~/.vscode/claude.mcp.json - gasoline entry missing
                   Fix: gasoline-mcp --install --for-all

                ✅ Cursor
                   ~/.cursor/mcp.json - Configured and ready

                ⚠️  Codeium
                   ~/.codeium/mcp.json - Invalid JSON at line 5
                   Fix: Manually edit or restore from backup
                       code ~/.codeium/mcp.json

                ✅ Binary check
                   Gasoline binary found at /usr/local/bin/gasoline
                   Version: v5.3.0

                Summary: 2 tools ready, 1 needs repair, 1 not configured

3. User gets:   Actionable next steps for each issue
```

### Workflow 5: Cleanup
```
User wants to remove Gasoline from all configs

1. User runs:   gasoline-mcp --uninstall --dry-run
2. Sees:        Would remove gasoline from:
                - ~/.claude/claude.mcp.json
                - ~/.vscode/claude.mcp.json
3. User runs:   gasoline-mcp --uninstall
4. Sees:        ✅ Removed from 2 tools
                ℹ️  Not configured in Cursor, Codeium
```

## Examples

### Basic Commands
```bash
# Show current config template
gasoline-mcp --config

# Install to first matching config
gasoline-mcp --install
# Output: ✅ Installed Gasoline to Claude Desktop at ~/.claude/claude.mcp.json

# Preview what --install would do (no files written)
gasoline-mcp --install --dry-run

# Install to ALL 4 detected tools
gasoline-mcp --install --for-all
# Output:
# ✅ 4/4 tools updated:
#    ✅ Claude Desktop (at ~/.claude/claude.mcp.json)
#    ✅ VSCode (at ~/.vscode/claude.mcp.json)
#    ✅ Cursor (at ~/.cursor/mcp.json)
#    ✅ Codeium (at ~/.codeium/mcp.json)
```

### Environment Variables
```bash
# Install with custom server URL
gasoline-mcp --install --env GASOLINE_SERVER=http://custom:7890

# Install with multiple env vars
gasoline-mcp --install --env DEBUG=1 --env LOG_LEVEL=info

# ⚠️  SECURITY: Never store API keys in --env!
# WRONG: gasoline-mcp --install --env API_KEY='sk-...'
# RIGHT: Store key in ~/.gasoline/secrets (mode 600) then:
#        gasoline-mcp --install --env SECRETS_FILE=/home/user/.gasoline/secrets
```

### Diagnostics & Uninstall
```bash
# Run diagnostics (check all configs are valid)
gasoline-mcp --doctor

# Uninstall from all configs (preserves other MCP servers)
gasoline-mcp --uninstall

# Preview uninstall (no files written)
gasoline-mcp --uninstall --dry-run

# Verbose output for all commands
gasoline-mcp --install --verbose

# Help
gasoline-mcp --help
```

### Error Scenarios
```bash
# Error: --env without --install
gasoline-mcp --env DEBUG=1
# ❌ Error: --env only works with --install
#    Usage: gasoline-mcp --install --env KEY=VALUE

# Error: Invalid env format
gasoline-mcp --install --env BADFORMAT
# ❌ Error: Invalid env format "BADFORMAT". Expected: KEY=VALUE
#    Example: gasoline-mcp --install --env GASOLINE_SERVER=http://localhost:7890

# Error: Permission denied
gasoline-mcp --install
# ❌ Error: Permission denied writing ~/.claude/claude.mcp.json
#    Try: sudo gasoline-mcp --install
#    Or: Check permissions with: ls -la ~/.claude/

# Error: Invalid JSON in config
gasoline-mcp --install
# ❌ Error: Invalid JSON in ~/.vscode/claude.mcp.json at line 15
#    Unexpected token }
#
#    Fix options:
#    1. Manually edit: code ~/.vscode/claude.mcp.json
#    2. Restore from backup and try --install again
#    3. Run: gasoline-mcp --doctor (for more info)
```

## Notes

- Related: `.claude/docs/architecture.md` (4-tool MCP constraint)
- Related: `npm/gasoline-mcp/package.json` (distribution)
- Building on: v5.2.0 CLI foundation (`--config`, `--install`)
- Test reference: `tests/extension/` (use as model for new tests)
