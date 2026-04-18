---
doc_type: tech-spec
feature_id: feature-tab-tracking-ux
status: proposed
owners: []
last_reviewed: 2026-04-18
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
last_verified_version: 0.8.2
last_verified_date: 2026-04-18
---

# Tab Tracking Ux Tech Spec

## TL;DR

- Status: shipped for hover/popup workspace launcher coordination
- Tool: null
- Mode/Action: null
- This document is a generated placeholder and should be completed.

## Linked Specs

- Product: [product-spec.md](./product-spec.md)
- Tech: [tech-spec.md](./tech-spec.md)
- QA: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_TAB_TRACKING_UX_001
- FEATURE_TAB_TRACKING_UX_002
- FEATURE_TAB_TRACKING_UX_003

## Notes

- Hover launcher visibility is still keyed off `StorageKey.TERMINAL_UI_STATE`.
- Popup and hover audit paths stay aligned through `src/lib/request-audit.ts` and `src/lib/workspace-actions.ts`.
- The hover `Workspace` CTA still uses `open_terminal_panel` internally, while screenshot, audit, note, and recording paths stay aligned with the workspace action row through shared helpers.
- The hover launcher now exposes `Workspace — open the QA workspace` while preserving the internal `open_terminal_panel` contract.
- Code references: `src/content/ui/tracked-hover-launcher.ts`, `src/content/ui/terminal-panel-bridge.ts`, `src/lib/workspace-actions.ts`, `src/popup/tab-tracking.ts`, `src/popup/tab-tracking-api.ts`
- Test references: `tests/extension/tracked-hover-launcher.test.js`, `tests/extension/popup-audit-button.test.js`, `tests/extension/workspace-actions.test.js`
