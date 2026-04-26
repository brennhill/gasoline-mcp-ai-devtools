---
doc_type: flow_map_pointer
status: active
last_reviewed: 2026-04-20
canonical_flow_map: ../../../architecture/flow-maps/installer-binary-path-and-manual-extension-handoff.md
---

# Enhanced CLI Config Flow Map Pointer

Canonical flow map:

- [Installer Binary Path and Manual Extension Handoff](../../../architecture/flow-maps/installer-binary-path-and-manual-extension-handoff.md)

Notable coverage:

- Extension staging integrity checks and source-zip fallback for incomplete release extension artifacts.
- Installer extension refresh now stages + validates + promotes atomically, with rollback to previous extension state on promotion failure.
- npm wrapper install/update/uninstall now converge on `kaboom-browser-devtools` and aggressively remove managed `kaboom-*`, `gasoline-*`, and `strum-*` entries.
- npm wrapper config/doctor helpers now share the same legacy-key list so old `kaboom-*`, `gasoline-*`, and `strum-*` entries are removed during writes and surfaced as non-OK during diagnostics.
- Strict checksum mode (`KABOOM_INSTALL_STRICT=1`) enforces fail-closed binary verification.
- Server postinstall validates `/health` against `kaboom-browser-devtools` before reusing an occupied port.
- Installer defaults unpacked extension output to `~/KaboomAgenticDevtoolExtension` (overridable via `KABOOM_EXTENSION_DIR`) so users can select it in Chrome without enabling hidden files.
- CRX fallback packaging in `scripts/build-crx.js` archives the full `extension/` directory to prevent missing MV3 module imports.
- Startup integrity regression checks assert manifest file paths and service worker import graph resolve before release.
- Shell installer rejects `sudo`/root execution so home-scoped install state and daemon install identity stay stable.
- Installer entrypoints no longer emit direct analytics beacons; onboarding is counted from daemon-owned canonical events only.
- Native `--install` also refuses privileged execution, npm postinstall skips elevated daemon auto-start, and install analytics state is file-locked so concurrent startups cannot mint duplicate installs or duplicate `first_tool_call`.
- Daemon startup and extension reconnect lifecycle signals remain local diagnostics and are not sent as raw analytics rows.
