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
  - extension/popup.html
  - extension/options.html
  - extension/theme-bootstrap.js
  - scripts/install.sh
  - scripts/install.ps1
  - npm/gasoline-mcp/lib/cli.js
  - npm/gasoline-mcp/lib/skills.js
  - npm/gasoline-mcp/lib/postinstall-skills.js
  - pypi/gasoline-mcp/gasoline_mcp/platform.py
  - pypi/gasoline-mcp/gasoline_mcp/skills.py
  - scripts/install-bundled-skills.sh
test_paths:
  - pypi/gasoline-mcp/tests/test_install.py
  - pypi/gasoline-mcp/tests/test_skills.py
  - tests/extension/release-extension-zip.test.js
  - tests/extension/release-extension-crx-fallback.test.js
  - tests/extension/manifest-startup-integrity.test.js
  - tests/extension/install-script-extension-source.test.js
  - tests/cli/cli-integration.test.cjs
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
