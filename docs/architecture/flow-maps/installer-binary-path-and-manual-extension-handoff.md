---
doc_type: flow_map
flow_id: installer-binary-path-and-manual-extension-handoff
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - scripts/install.sh
  - scripts/install.ps1
  - cmd/dev-console/native_install.go:runNativeInstall
  - npm/gasoline-agentic-browser/lib/install.js:executeInstall
  - pypi/gasoline-agentic-browser/gasoline_agentic_browser/platform.py:run_install
code_paths:
  - scripts/install.sh
  - scripts/install.ps1
  - server/scripts/install.js
  - cmd/dev-console/native_install.go
  - npm/gasoline-agentic-browser/lib/config.js
  - npm/gasoline-agentic-browser/lib/install.js
  - pypi/gasoline-agentic-browser/gasoline_agentic_browser/install.py
  - pypi/gasoline-agentic-browser/gasoline_agentic_browser/platform.py
  - docs/mcp-install-guide.md
test_paths:
  - cmd/dev-console/native_install_test.go
  - npm/gasoline-agentic-browser/lib/install.test.js
  - pypi/gasoline-agentic-browser/tests/test_install.py
---

# Installer Binary Path and Manual Extension Handoff

## Scope

Covers installer behavior for shell, PowerShell, npm wrapper, and PyPI wrapper to ensure:

1. MCP configs use a direct binary path when available.
2. Installer output clearly states that extension loading is a manual browser action.
3. Installer output uses a consistent, polished step-and-checklist presentation across entrypoints.

## Entrypoints

1. One-liner installers: `scripts/install.sh` and `scripts/install.ps1`.
2. Native CLI install flow: `runNativeInstall`.
3. Wrapper install commands: npm `executeInstall` and PyPI `run_install`.

## Primary Flow

1. Installer resolves platform and downloads/stages binary + extension artifacts.
2. Wrapper/native install writes MCP client configs.
3. Config entries prefer resolved binary paths over transient launchers.
4. Installer prints explicit manual extension checklist:
   - open extensions page
   - enable developer mode
   - load unpacked extension folder
   - pin extension
   - click Track This Tab
5. Installer surfaces a branded panel-style summary at completion with the resolved binary path.

## Error and Recovery Paths

1. If platform binary cannot be resolved, wrappers fall back to command name for compatibility.
2. If extension cannot be side-loaded automatically, installer still succeeds but instructs user on manual steps.
3. Missing client config directories are skipped without aborting install.

## State and Contracts

1. MCP server key remains `gasoline-browser-devtools`.
2. File-based clients must receive deterministic command entries (`command` + `args`).
3. Installer output must never imply that browser extension installation is fully automatic.

## Code Paths

- `scripts/install.sh`
- `scripts/install.ps1`
- `server/scripts/install.js`
- `cmd/dev-console/native_install.go`
- `npm/gasoline-agentic-browser/lib/config.js`
- `npm/gasoline-agentic-browser/lib/install.js`
- `pypi/gasoline-agentic-browser/gasoline_agentic_browser/install.py`
- `pypi/gasoline-agentic-browser/gasoline_agentic_browser/platform.py`
- `docs/mcp-install-guide.md`

## Test Paths

- `cmd/dev-console/native_install_test.go`
- `npm/gasoline-agentic-browser/lib/install.test.js`
- `pypi/gasoline-agentic-browser/tests/test_install.py`

## Edit Guardrails

1. Keep wrapper install outputs aligned across npm and PyPI.
2. Do not regress to `npx`-only config entries for managed installs.
3. Preserve manual-extension wording in installer output to avoid user confusion.
