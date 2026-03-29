---
doc_type: flow_map
flow_id: installer-binary-path-and-manual-extension-handoff
status: active
last_reviewed: 2026-03-28
owners:
  - Brenn
entrypoints:
  - scripts/install.sh
  - scripts/install.ps1
  - scripts/build-crx.js
  - cmd/browser-agent/native_install.go:runNativeInstall
  - npm/kaboom-agentic-browser/lib/install.js:executeInstall
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py:run_install
code_paths:
  - Makefile
  - scripts/build-crx.js
  - scripts/install.sh
  - scripts/install.ps1
  - server/scripts/install.js
  - cmd/browser-agent/native_install.go
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py
  - docs/mcp-install-guide.md
test_paths:
  - cmd/browser-agent/native_install_test.go
  - npm/kaboom-agentic-browser/lib/config.test.js
  - npm/kaboom-agentic-browser/lib/install.test.js
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
  - pypi/kaboom-agentic-browser/tests/test_install.py
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
---

# Installer Binary Path and Manual Extension Handoff

## Scope

Covers installer behavior for shell, PowerShell, npm wrapper, and PyPI wrapper to ensure:

1. MCP configs use a direct binary path when available.
2. Installer output clearly states that extension loading is a manual browser action.
3. Extension staging always includes required MV3 module files for service-worker registration.
4. Installer output uses a consistent, polished step-and-checklist presentation across entrypoints.
5. CRX fallback packaging must include the full `extension/` tree (no allowlist packaging).
6. Extension refresh is atomic (stage + validate + promote) so failed upgrades do not destroy a previously working extension install.
7. Installers support strict checksum enforcement (`KABOOM_INSTALL_STRICT=1`) for fail-closed install workflows.

## Entrypoints

1. One-liner installers: `scripts/install.sh` and `scripts/install.ps1`.
2. Native CLI install flow: `runNativeInstall`.
3. Wrapper install commands: npm `executeInstall` and PyPI `run_install`.

## Primary Flow

1. Installer resolves platform and downloads/stages binary + extension artifacts.
2. Extension release packaging (`make extension-zip` and `scripts/build-crx.js` fallback zip) archives the entire `extension/` directory.
3. Binary installers verify SHA-256 against release `checksums.txt` (or fail immediately in strict mode).
4. Extension is extracted into a staging directory and validated for required module files (`manifest.json`, `background/init.js`, `content/script-injection.js`, `inject/index.js`, `theme-bootstrap.js`).
5. If the release extension zip is incomplete, installer falls back to source-zip extraction and validates again.
6. Only validated staging directories are promoted atomically to `~/KaboomAgenticDevtoolExtension`; prior extension state is restored on promotion failure.
7. Wrapper/native install writes MCP client configs.
8. Config entries prefer resolved binary paths over transient launchers.
9. Installer prints explicit manual extension checklist:
   - open extensions page
   - enable developer mode
   - load unpacked extension folder
   - pin extension
   - click Track This Tab
10. Installer surfaces a branded panel-style summary at completion with the resolved binary path.

## Error and Recovery Paths

1. If platform binary cannot be resolved, wrappers fall back to command name for compatibility.
2. If release extension zip is missing required module files, installer falls back to source zip and revalidates staged files.
3. If extension promotion fails, installer restores the pre-existing extension directory instead of leaving a partial install.
4. If strict checksum mode is enabled and checksums cannot be verified, installers fail closed.
5. npm postinstall validates existing `/health` identity/version when port is already in use and refuses false-positive success for non-Kaboom services.
6. If extension cannot be side-loaded automatically, installer still succeeds but instructs user on manual steps.
7. Missing client config directories are skipped without aborting install.

## State and Contracts

1. npm wrapper installs register MCP server key `kaboom-browser-devtools` and remove managed `kaboom-*`, `gasoline-*`, and `strum-*` entries during install/update/uninstall.
2. npm wrapper config and doctor helpers share the same legacy-key list so diagnostics flag stale aliases that install/update will purge.
3. File-based clients must receive deterministic command entries (`command` + `args`).
4. Release extension artifacts must include the full extension tree so MV3 module imports resolve at runtime.
5. Installer output must never imply that browser extension installation is fully automatic.
6. In strict mode, checksum verification is mandatory for release binary downloads.
7. Existing-daemon reuse on port checks requires service identity and version parity.
8. Extension unpacked path defaults to `~/KaboomAgenticDevtoolExtension` (overridable with `KABOOM_EXTENSION_DIR`).

## Code Paths

- `Makefile`
- `scripts/build-crx.js`
- `scripts/install.sh`
- `scripts/install.ps1`
- `server/scripts/install.js`
- `cmd/browser-agent/native_install.go`
- `npm/kaboom-agentic-browser/lib/config.js`
- `npm/kaboom-agentic-browser/lib/install.js`
- `npm/kaboom-agentic-browser/lib/uninstall.js`
- `pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py`
- `pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py`
- `docs/mcp-install-guide.md`

## Test Paths

- `cmd/browser-agent/native_install_test.go`
- `npm/kaboom-agentic-browser/lib/config.test.js`
- `npm/kaboom-agentic-browser/lib/install.test.js`
- `npm/kaboom-agentic-browser/lib/uninstall.test.js`
- `pypi/kaboom-agentic-browser/tests/test_install.py`
- `tests/extension/release-extension-zip.test.js`
- `tests/extension/release-extension-crx-fallback.test.js`
- `tests/extension/manifest-startup-integrity.test.js`
- `tests/extension/install-script-extension-source.test.js`
- `tests/cli/server-install-hardening.test.cjs`
- `tests/cli/install.test.cjs`
- `tests/cli/uninstall.test.cjs`

## Edit Guardrails

1. Keep wrapper install outputs aligned across npm and PyPI.
2. Do not regress to `npx`-only config entries for managed installs.
3. Do not reintroduce allowlist-based packaging in extension zip or CRX fallback flows.
4. Preserve manual-extension wording in installer output to avoid user confusion.
