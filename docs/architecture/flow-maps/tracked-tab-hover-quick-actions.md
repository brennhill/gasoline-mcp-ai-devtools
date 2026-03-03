---
doc_type: flow_map
flow_id: tracked-tab-hover-quick-actions
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
entrypoints:
  - src/content.ts (initTabTracking callback)
  - src/content/ui/tracked-hover-launcher.ts (setTrackedHoverLauncherEnabled)
  - src/popup.ts (initPopup reshow signal)
code_paths:
  - src/content.ts
  - src/content/tab-tracking.ts
  - src/content/ui/tracked-hover-launcher.ts
  - src/popup.ts
  - src/background/message-handlers.ts
  - src/background/recording-listeners.ts
test_paths:
  - tests/extension/tracked-hover-launcher.test.js
  - tests/extension/content.test.js
  - tests/extension/popup-status.test.js
---

# Tracked Tab Hover Quick Actions

## Scope

Inject a floating quick-action launcher on tracked tabs so users can start annotation draw mode, start or stop recording, and take screenshots without reopening the popup. The launcher also exposes a settings gear with docs/repo links and a hide control.

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
4. `Draw` action dynamically loads `content/draw-mode.js` and calls `activateDrawMode('user')`.
5. `Rec` or `Stop` action sends `record_start` or `record_stop` to background recording listeners.
6. `Shot` action sends `captureScreenshot` to background message handlers.
7. `Hide Gasoline Devtool` sets `StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN=true` and unmounts the launcher.
8. On next popup open, `initPopup` sends `GASOLINE_SHOW_TRACKED_HOVER_LAUNCHER` to active tab.
9. Content script clears persisted hidden state and remounts launcher if tracking is still enabled.
10. Record button state stays aligned with `chrome.storage.local[gasoline_recording]` via initial read plus `chrome.storage.onChanged`.

## Error and Recovery Paths

- Draw-mode dynamic import failures are best-effort and do not block page interactions.
- Runtime messaging errors are ignored to prevent launcher UI lockups when background is unavailable.
- Recording button falls back to storage re-sync if response status is unexpected.
- Popup reshow message is best-effort; if active tab has no content script, it is ignored.

## State and Contracts

- Launcher is tab-local content UI and only mounts for tracked tabs.
- `StorageKey.RECORDING` is the source of truth for active recording UI state.
- `StorageKey.TRACKED_HOVER_LAUNCHER_HIDDEN` persists hidden-state across page reloads.
- `hiddenUntilPopupOpen` mirrors persisted hidden-state in memory and suppresses remounts until popup sends reshow message.
- Action message contracts:
  - Draw: `GASOLINE_DRAW_MODE_START` equivalent behavior via direct module activation.
  - Record: `record_start` / `record_stop`.
  - Screenshot: `captureScreenshot`.
  - Reshow: `GASOLINE_SHOW_TRACKED_HOVER_LAUNCHER`.

## Code Paths

- `src/content.ts`
- `src/content/tab-tracking.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `src/popup.ts`
- `src/background/message-handlers.ts`
- `src/background/recording-listeners.ts`

## Test Paths

- `tests/extension/tracked-hover-launcher.test.js`
- `tests/extension/content.test.js`
- `tests/extension/popup-status.test.js`

## Edit Guardrails

- Keep launcher mount strictly tied to tracked-tab state; never show it on untracked tabs.
- Do not bypass storage-based recording state sync with ad hoc local toggles.
- Preserve non-blocking UI behavior for action failures; avoid throwing in content-script interaction handlers.
- Keep reshow trigger explicit from popup-open flow; do not auto-clear hidden state on page navigation alone.
