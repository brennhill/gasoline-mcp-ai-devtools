---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - Makefile
  - scripts/build-crx.js
  - cmd/dev-console/native_install.go
  - scripts/install.sh
  - scripts/install.ps1
  - server/scripts/install.js
  - npm/gasoline-agentic-browser/lib/config.js
  - npm/gasoline-agentic-browser/lib/install.js
  - npm/gasoline-agentic-browser/lib/cli.js
  - pypi/gasoline-agentic-browser/gasoline_agentic_browser/install.py
  - pypi/gasoline-agentic-browser/gasoline_agentic_browser/platform.py
  - docs/mcp-install-guide.md
test_paths:
  - cmd/dev-console/native_install_test.go
  - npm/gasoline-agentic-browser/lib/install.test.js
  - pypi/gasoline-agentic-browser/tests/test_install.py
  - pypi/gasoline-mcp/tests/test_install.py
  - pypi/gasoline-mcp/tests/test_skills.py
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
  - tests/cli/cli-integration.test.cjs
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
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

Add concrete implementation and test links here as this feature evolves.
