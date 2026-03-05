---
doc_type: feature_index
feature_id: feature-tab-recording
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - cmd/dev-console/tools_recording_video.go
  - cmd/dev-console/tools_recording_video_handlers.go
  - src/background/event-listeners.ts
  - src/background/init.ts
  - src/background/recording.ts
  - src/popup/recording.ts
  - extension/manifest.json
  - extension/popup.html
test_paths:
  - cmd/dev-console/tools_recording_video_test.go
  - tests/extension/recording-shortcut-command.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tab Recording

## TL;DR

- Status: proposed
- Tool: interact, observe
- Mode/Action: record_start, record_stop, saved_videos, toggle_action_sequence_recording
- Location: `docs/features/feature/tab-recording`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_TAB_RECORDING_001
- FEATURE_TAB_RECORDING_002
- FEATURE_TAB_RECORDING_003

## Code and Tests

The implementation and tests for popup/manual recording and shortcut-toggle recording are listed in frontmatter `code_paths` and `test_paths`.
