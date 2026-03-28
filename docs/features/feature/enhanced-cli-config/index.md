---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-28
code_paths:
  - Makefile
  - scripts/build-crx.js
  - cmd/browser-agent/native_install.go
  - scripts/install.sh
  - scripts/install.ps1
  - server/scripts/install.js
  - npm/kaboom-agentic-browser/lib/config.js
  - npm/kaboom-agentic-browser/lib/install.js
  - npm/kaboom-agentic-browser/lib/uninstall.js
  - npm/kaboom-agentic-browser/lib/cli.js
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/install.py
  - pypi/kaboom-agentic-browser/kaboom_agentic_browser/platform.py
  - docs/mcp-install-guide.md
test_paths:
  - cmd/browser-agent/native_install_test.go
  - npm/kaboom-agentic-browser/lib/config.test.js
  - npm/kaboom-agentic-browser/lib/install.test.js
  - npm/kaboom-agentic-browser/lib/uninstall.test.js
  - pypi/kaboom-agentic-browser/tests/test_install.py
  - pypi/kaboom-agentic-browser/tests/test_skills.py
  - tests/extension/install-script-extension-source.test.js
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
  - tests/cli/server-install-hardening.test.cjs
  - tests/cli/cli-integration.test.cjs
  - tests/cli/install.test.cjs
  - tests/cli/uninstall.test.cjs
last_verified_version: 0.8.0
last_verified_date: 2026-03-06
---

# Enhanced Cli Config

## TL;DR

- Status: proposed
- Tool: configure
- Mode/Action: cli
- Location: `docs/features/feature/enhanced-cli-config`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_ENHANCED_CLI_CONFIG_001
- FEATURE_ENHANCED_CLI_CONFIG_002
- FEATURE_ENHANCED_CLI_CONFIG_003

## Code and Tests

- npm wrapper installs now register `kaboom-browser-devtools` and remove legacy `gasoline-*` and `strum-*` MCP entries during install/update/uninstall.
- Platform npm packages now ship `kaboom-agentic-browser` and `kaboom-hooks` binaries while preserving legacy cleanup for customer machines.
