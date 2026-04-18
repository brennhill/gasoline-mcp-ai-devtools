---
doc_type: feature_flow_map_pointer
feature_id: feature-tab-tracking-ux
status: active
last_reviewed: 2026-04-18
canonical_flow_map: docs/architecture/flow-maps/tracked-tab-hover-quick-actions.md
last_verified_version: 0.8.2
last_verified_date: 2026-04-18
---

# Flow Map Pointer

Canonical flow map:

- [Tracked Tab Hover Quick Actions](../../../architecture/flow-maps/tracked-tab-hover-quick-actions.md)
- [Workspace Sidebar QA Shell](../../../architecture/flow-maps/workspace-sidebar-qa-shell.md)
- The tracked hover launcher now uses `src/content/ui/terminal-panel-bridge.ts` to open the terminal side panel and observe panel visibility.
- The launcher hides only while the workspace side panel is open and reappears for both minimized and fully closed panel states.
- The hover island now keeps the restored Kaboom flame icon on hover and routes audit/screenshot workspace actions through shared helpers.
