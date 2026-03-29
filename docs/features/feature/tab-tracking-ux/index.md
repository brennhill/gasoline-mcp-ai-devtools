---
doc_type: feature_index
feature_id: feature-tab-tracking-ux
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-28
code_paths:
  - src/lib/brand.ts
  - src/lib/constants.ts
  - src/types/runtime-messages.ts
  - src/content.ts
  - src/content/tab-tracking.ts
  - src/content/ui/terminal-panel-bridge.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/popup.ts
  - src/popup/logo-motion.ts
  - src/popup/tab-tracking.ts
  - src/popup/tab-tracking-api.ts
  - src/background/message-handlers.ts
  - src/background/recording-listeners.ts
test_paths:
  - tests/extension/brand-metadata.test.js
  - tests/extension/popup-tab-tracking-branding.test.js
  - tests/extension/recording-listeners-target-tab.test.js
  - tests/extension/tracked-hover-launcher.test.js
  - tests/extension/logo-motion.test.js
  - tests/extension/content.test.js
  - tests/extension/runtime-log-branding.test.js
  - tests/extension/sidepanel-terminal.test.js
last_verified_version: 0.8.1
last_verified_date: 2026-03-28
---

# Tab Tracking Ux

## TL;DR

- Status: shipped
- Tool: null
- Mode/Action: null
- Location: `docs/features/feature/tab-tracking-ux`
- The hover launcher is shown on tracked workspace tabs and hides only while the Kaboom side panel is open.
- Terminal workspace ownership now targets one Chrome tab group, even though broader tracking flows still use `TRACKED_TAB_ID` during the rollout.
- The hover island now uses the restored Kaboom flame icon consistently; hover elevates the button chrome but does not swap the asset.
- Cloaked-domain disable messaging and popup-driven recording guidance now use Kaboom copy consistently.
- The hover launcher settings gear now points at `gokaboom.dev/docs` and the Kaboom repo, and tracked-tab-loss guidance tells users to reopen the Kaboom popup.
- Draw-mode recovery warnings from the hover launcher now use Kaboom copy when the extension was reloaded or the draw bundle is unavailable.
- Popup tab-tracking logs now use the shared Kaboom runtime prefix instead of hardcoded Kaboom labels.

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_TAB_TRACKING_UX_001
- FEATURE_TAB_TRACKING_UX_002
- FEATURE_TAB_TRACKING_UX_003

## Code and Tests

Concrete implementation and test paths are listed in frontmatter `code_paths` and `test_paths`.
