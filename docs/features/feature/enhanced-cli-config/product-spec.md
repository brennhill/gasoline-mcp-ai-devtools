---
feature: Enhanced CLI Configuration Management
status: proposed
tool: configure
mode: cli
version: v5.3
doc_type: product-spec
feature_id: feature-enhanced-cli-config
last_reviewed: 2026-02-16
---

# Product Spec: Enhanced CLI Configuration Management

## Problem Statement

Users installing Gasoline MCP (via NPM or PyPI) need to:
1. Configure it across multiple AI assistant clients (Claude Code, Claude Desktop, VS Code, Cursor, Windsurf)
2. Verify the configuration is correct before committing changes
3. Diagnose connection/installation issues when things go wrong
4. Manage environment variables for different deployment scenarios
5. Uninstall Gasoline cleanly if needed

The current implementation (`--config`, `--install`) is functional but incomplete in both wrappers:
- **No preview mode**: Users can't see what changes will be made before committing
- **No diagnostics**: When installation fails, users have no way to troubleshoot
- **Single-client support**: Only first matching config file gets updated; other clients left out
- **No verification**: No way to confirm the config is actually working
- **No environment variables**: Can't inject custom env vars (e.g., API keys, server URLs) into MCP config
- **Limited error recovery**: Confusing error messages when JSON is malformed or permissions denied

## Solution

Enhance the gasoline-mcp CLI with four new features:

### 1. **Dry-Run Mode** (`--dry-run`)
Preview exactly what changes will be made without actually writing files. Shows:
- Which config file would be modified/created
- Before/after JSON diff
- Estimated file permissions
- Safe for checking before real installation

### 2. **Doctor Command** (`--doctor`)
Diagnostic tool that verifies local configuration:
- Checks all 5 client configs and their JSON validity
- Verifies gasoline entry is present in each config
- Confirms gasoline binary path resolves and is executable
- Tests basic binary invocation (runs `gasoline --version`)
- Reports which clients are configured, which aren't
- Detects legacy config paths and warns about orphaned configs
- Provides actionable repair suggestions for fixable issues
- **Note**: Cannot test actual MCP connection without AI agent running (VS Code, Claude, etc.)

### 3. **Environment Variables** (`--install --env KEY=VALUE`)
Allow injecting environment variables into MCP config:
- `--env GASOLINE_SERVER=http://localhost:7890`
- `--env DEBUG=1`
- Multiple `--env` flags supported
- Saved in `env` object of MCP config
- Useful for custom server URLs, debug modes, offline deployments

### 4. **Uninstall Command** (`--uninstall`)
Remove Gasoline from AI assistant configs:
- Find gasoline entry in all detected client configs
- Remove it cleanly (preserve other MCP servers)
- Confirm removal before writing
- Report which clients were cleaned up

## Requirements

1. **Usability**: All commands must have helpful output and clear error messages
2. **Safety**: Dry-run must work correctly; no actual changes on `--dry-run`
3. **Idempotence**: Multiple runs with same args produce same result
4. **Backward Compatible**: Existing `--config`, `--install`, `--help` must still work
5. **Composable**: `--dry-run` works with `--install`, `--uninstall`, `--doctor`
6. **Verbose**: `--verbose` flag shows detailed operation logs
7. **Recovery**: Commands provide next-step suggestions on failure
8. **Multi-client**: `--install` targets all detected clients by default

## Out of Scope

- GUI configuration tool (CLI only)
- Automatic config migration between versions
- Remote config management (local files only)
- Automatic version upgrades
- Configuration encryption/secrets management (users responsible for env vars)

## Success Criteria

1. ✅ `--dry-run` shows exact changes without writing files
2. ✅ `--doctor` diagnoses 90%+ of common config issues
3. ✅ `--install` auto-detects and installs to all 5 supported clients
4. ✅ `--env KEY=VALUE` properly saves env vars to config
5. ✅ `--uninstall` cleanly removes gasoline from all configs
6. ✅ All commands provide actionable error messages with recovery steps
7. ✅ 100% backward compatibility with v5.2.0 CLI behavior
8. ✅ All commands covered by automated tests (unit + integration)
9. ✅ Help text and examples available for each command

## User Workflows

### Workflow 1: Safe Installation
```
User wants to try Gasoline on their VS Code but isn't sure what will change

1. User runs:   gasoline-mcp --config
2. Sees:        Detected clients + config status for each
3. User runs:   gasoline-mcp --install --dry-run
4. Sees:        Exact JSON changes for each detected client
5. User runs:   gasoline-mcp --install
6. Sees:        ✅ Installed to all detected clients
7. User runs:   gasoline-mcp --doctor
8. Sees:        ✅ Gasoline configured in Claude Code, Cursor
                ⚪ Claude Desktop not detected
                ⚪ Windsurf not detected
```

### Workflow 2: Multi-Client Install
```
User wants Gasoline available in all AI assistants

1. User runs:   gasoline-mcp --install
2. Sees:        ✅ Claude Code: Installed (via CLI)
                ✅ Claude Desktop: Updated
                ✅ Cursor: Updated
                ✅ Windsurf: Updated
                ✅ VS Code: Updated
3. User runs:   gasoline-mcp --doctor
4. Sees:        ✅ All 5 clients configured
```

### Workflow 3: Environment Variables
```
User needs to point Gasoline at custom server URL

1. User runs:   gasoline-mcp --install --env GASOLINE_SERVER=http://192.168.1.100:7890
2. Sees:        ✅ Installed to all detected clients
3. Config has:  "env": { "GASOLINE_SERVER": "http://192.168.1.100:7890" }
```

### Workflow 4: Troubleshooting
```
User says "It's not working, can you check?"

1. User runs:   gasoline-mcp --doctor
2. Sees:
                Gasoline MCP Diagnostic Report

                ✅ Claude Code
                   Configured via CLI

                ✅ Cursor
                   ~/.cursor/mcp.json - Configured and ready

                ⚠️  Windsurf
                   ~/.codeium/windsurf/mcp_config.json - Invalid JSON at line 5
                   Fix: Manually edit or restore from backup

                ⚪ Claude Desktop
                   Not detected

                ⚪ VS Code
                   Not detected

                ✅ Binary check
                   Gasoline binary found at /usr/local/bin/gasoline

                Summary: 2 clients ready, 1 needs repair, 2 not detected

3. User gets:   Actionable next steps for each issue
```

### Workflow 5: Cleanup
```
User wants to remove Gasoline from all configs

1. User runs:   gasoline-mcp --uninstall --dry-run
2. Sees:        Would remove gasoline from:
                - Claude Code (via CLI)
                - ~/.cursor/mcp.json
3. User runs:   gasoline-mcp --uninstall
4. Sees:        ✅ Removed from 2 clients
```

## Examples

### Basic Commands
```bash
# Show detected clients and config status
gasoline-mcp --config

# Install to all detected clients
gasoline-mcp --install
# Output:
# ✅ 5/5 clients installed:
#    ✅ Claude Code (via CLI)
#    ✅ Claude Desktop (at ~/Library/Application Support/Claude/claude_desktop_config.json)
#    ✅ Cursor (at ~/.cursor/mcp.json)
#    ✅ Windsurf (at ~/.codeium/windsurf/mcp_config.json)
#    ✅ VS Code (at ~/Library/Application Support/Code/User/mcp.json)

# Preview what --install would do (no files written)
gasoline-mcp --install --dry-run
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
# ❌ Error: Permission denied writing ~/.cursor/mcp.json
#    Try: sudo gasoline-mcp --install
#    Or: Check permissions with: ls -la ~/.cursor/

# Error: Invalid JSON in config
gasoline-mcp --install
# ❌ Error: Invalid JSON in ~/.cursor/mcp.json at line 15
#    Unexpected token }
#
#    Fix options:
#    1. Manually edit: code ~/.cursor/mcp.json
#    2. Restore from backup and try --install again
#    3. Run: gasoline-mcp --doctor (for more info)
```

## Notes

- **Distribution**: Both NPM (`npm/gasoline-mcp/`) and PyPI (`pypi/gasoline-mcp/`) wrappers must be updated with feature parity
- **NPM Entry Point**: `npm/gasoline-mcp/bin/gasoline-mcp` (Node.js)
- **PyPI Entry Point**: `pypi/gasoline-mcp/gasoline_mcp/__main__.py` (Python)
- **Related**: `.claude/docs/architecture.md` (5-tool MCP constraint)
- **Building on**: v5.2.0 CLI foundation (`--config`, `--install`)
- **Test reference**: `tests/extension/` (use as model for new tests)
