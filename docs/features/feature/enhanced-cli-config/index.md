---
doc_type: feature_index
feature_id: feature-enhanced-cli-config
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-02-23
code_paths:
  - Makefile
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

## Requirement IDs

- FEATURE_ENHANCED_CLI_CONFIG_001
- FEATURE_ENHANCED_CLI_CONFIG_002
- FEATURE_ENHANCED_CLI_CONFIG_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
