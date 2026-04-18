---
doc_type: flow_map
flow_id: tracked-tab-hover-quick-actions
status: active
last_reviewed: 2026-04-18
owners:
  - Brenn
entrypoints:
  - src/content.ts (initTabTracking callback)
  - src/content/ui/tracked-hover-launcher.ts (setTrackedHoverLauncherEnabled)
  - src/popup.ts (initPopup reshow signal)
code_paths:
  - src/lib/brand.ts
  - src/lib/request-audit.ts
  - src/lib/workspace-actions.ts
  - src/content.ts
  - src/content/tab-tracking.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/popup.ts
  - src/popup/logo-motion.ts
  - src/popup/tab-tracking.ts
  - src/popup/tab-tracking-api.ts
  - src/background/message-handlers.ts
  - src/background/recording-listeners.ts
test_paths:
  - tests/extension/brand-metadata.test.js
  - tests/extension/popup-audit-button.test.js
  - tests/extension/popup-tab-tracking-sync.test.js
  - tests/extension/workspace-actions.test.js
  - tests/extension/tracked-hover-launcher.test.js
  - tests/extension/content.test.js
  - tests/extension/logo-motion.test.js
  - tests/extension/popup-status.test.js
  - tests/extension/runtime-log-branding.test.js
last_verified_version: 0.8.2
last_verified_date: 2026-04-18
---

# Tracked Tab Hover Quick Actions

## Scope

Inject a floating quick-action launcher on tracked workspace tabs so users can start annotation draw mode, start or stop recording, take screenshots, launch an audit, and open the QA workspace sidebar without reopening the popup. The launcher also exposes a settings gear with docs/repo links and a hide control.

Related feature docs:

- `docs/features/feature/tab-tracking-ux/index.md`
- `docs/features/feature/tab-tracking-ux/flow-map.md`

## Entrypoints

- `initTabTracking` callback in `src/content.ts`.
- `setTrackedHoverLauncherEnabled` in `src/content/ui/tracked-hover-launcher.ts`.

## Primary Flow

1. `initTabTracking` computes whether the current content script tab matches `trackedTabId`.
2. `src/content.ts` callback mounts the launcher when tracked and unmounts when untracked.
3. Hovering the launcher expands the action pill; clicking the gear expands a settings menu with fluid transform+opacity transitions.
4. The settings menu points to `https://gokaboom.dev/docs` and `https://github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP`.
5. The hover island logo uses the shared Kaboom flame mark from `icons/icon.svg` and swaps to `icons/logo-animated.svg` on hover.
6. `Draw` action dynamically loads `content/draw-mode.js` and calls `activateDrawMode('user')`.
7. `Rec` or `Stop` action routes through shared workspace helpers and sends `screen_recording_start` or `screen_recording_stop`.
8. `Shot` action routes through the shared workspace screenshot helper and sends `capture_screenshot`.
9. `Audit` action calls the shared workspace audit helper, which opens the workspace and then sends `qa_scan_requested`.
10. `Workspace — open the QA workspace` sends `open_terminal_panel`; the background worker resolves the workspace host tab and opens the panel there.
11. `Hide KaBOOM! Devtool` sets `StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN=true` and unmounts the launcher.
12. On next popup open, `initPopup` sends `kaboom_show_tracked_hover_launcher` to the active tab and rehydrates popup logo state through `src/popup/logo-motion.ts`.
13. Content script clears persisted hidden state and remounts launcher if tracking is still enabled and the side panel is not open.
14. Record button state stays aligned with `chrome.storage.local[kaboom_recording]` via initial read plus `chrome.storage.onChanged`.
15. Popup tab-tracking API logs use `KABOOM_LOG_PREFIX` so tracking diagnostics stay aligned with the rebrand.

## Error and Recovery Paths

- Draw-mode dynamic import failures are best-effort and do not block page interactions.
- Draw-mode recovery warnings are Kaboom-branded for both invalidated extension context and draw-bundle load failures.
- Runtime messaging errors are ignored to prevent launcher UI lockups when background is unavailable.
- Audit launch treats `open_terminal_panel` as best-effort and still sends the audit bridge when panel open fails.
- Recording button falls back to storage re-sync if response status is unexpected.
- Popup reshow message is best-effort; if active tab has no content script, it is ignored.

## State and Contracts

- Launcher is tab-local content UI and only mounts for tracked tabs.
- `StorageKey.RECORDING` is the source of truth for active recording UI state.
- `StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN` persists hidden-state across page reloads.
- `hiddenUntilPopupOpen` mirrors persisted hidden-state in memory and suppresses remounts until popup sends reshow message.
- `StorageKey.TERMINAL_UI_STATE` hides the launcher only while the side panel is actually open.
- Action message contracts:
  - Draw: `KABOOM_DRAW_MODE_START` equivalent behavior via direct module activation.
  - Record: `screen_recording_start` / `screen_recording_stop`.
  - Screenshot: `capture_screenshot`.
  - Audit: `qa_scan_requested` via `requestAudit`.
  - Reshow: `kaboom_show_tracked_hover_launcher`.

## Code Paths

- `src/content.ts`
- `src/content/tab-tracking.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `src/lib/request-audit.ts`
- `src/popup.ts`
- `src/popup/logo-motion.ts`
- `src/popup/tab-tracking.ts`
- `src/popup/tab-tracking-api.ts`
- `src/background/message-handlers.ts`
- `src/background/recording-listeners.ts`

## Test Paths

- `tests/extension/popup-audit-button.test.js`
- `tests/extension/popup-tab-tracking-sync.test.js`
- `tests/extension/tracked-hover-launcher.test.js`
- `tests/extension/content.test.js`
- `tests/extension/logo-motion.test.js`
- `tests/extension/popup-status.test.js`

## Edit Guardrails

- Keep launcher mount strictly tied to tracked-tab state; never show it on arbitrary untracked tabs.
- Do not bypass storage-based recording state sync with ad hoc local toggles.
- Preserve non-blocking UI behavior for action failures; avoid throwing in content-script interaction handlers.
- Keep reshow trigger explicit from popup-open flow; do not auto-clear hidden state on page navigation alone.
- Keep hover audit/screenshot behavior on the shared workspace action helpers so it stays aligned with the popup CTA.
