---
doc_type: flow_map_pointer
status: active
last_reviewed: 2026-03-05
canonical_flow_map: ../../../architecture/flow-maps/installer-binary-path-and-manual-extension-handoff.md
---

# Enhanced CLI Config Flow Map Pointer

Canonical flow map:

- [Installer Binary Path and Manual Extension Handoff](../../../architecture/flow-maps/installer-binary-path-and-manual-extension-handoff.md)

Notable coverage:

- Extension staging integrity checks and source-zip fallback for incomplete release extension artifacts.
- Installer extension refresh now stages + validates + promotes atomically, with rollback to previous extension state on promotion failure.
- Strict checksum mode (`GASOLINE_INSTALL_STRICT=1`) enforces fail-closed binary verification.
- CRX fallback packaging in `scripts/build-crx.js` archives the full `extension/` directory to prevent missing MV3 module imports.
- Startup integrity regression checks assert manifest file paths and service worker import graph resolve before release.
