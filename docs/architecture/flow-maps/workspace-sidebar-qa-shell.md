---
doc_type: flow_map
status: active
last_reviewed: 2026-04-18
owners:
  - Brenn
last_verified_version: 0.8.2
last_verified_date: 2026-04-18
---

# Workspace Sidebar QA Shell

## Scope

This flow covers the refactored sidepanel workspace shell that wraps the existing PTY-backed terminal with a QA summary strip, shared action row, lightweight page-status area, and the background/content status snapshot bridge.

## Entrypoints

- `src/content/ui/tracked-hover-launcher.ts`
- `src/lib/workspace-actions.ts`
- `src/background/message-handlers.ts`
- `src/background/workspace-status.ts`
- `src/content/runtime-message-listener.ts`
- `src/content/workspace-status.ts`
- `src/sidepanel.ts`
- `src/sidepanel/workspace-context.ts`
- `src/sidepanel/workspace-shell.ts`
- `src/sidepanel/workspace-terminal-pane.ts`
- `src/sidepanel/workspace-status.ts`

## Primary Flow

1. The user clicks `Workspace — open the QA workspace` from the tracked hover launcher.
2. The content script sends `open_terminal_panel`; background resolves the workspace host tab and opens `sidepanel.html`.
3. The sidepanel restores or boots the singleton terminal session and mounts the workspace shell with four regions: summary strip, action row, terminal region, and status area.
4. During boot, the sidepanel requests `get_workspace_status` from background.
5. Background resolves the host tab, reads recording/session state, and asks the content script for `kaboom_get_workspace_status`.
6. Content computes deterministic heuristics for SEO, accessibility, performance, and page summary, then returns a typed payload.
7. Background assembles a `WorkspaceStatusSnapshot` and returns it to the sidepanel.
8. `src/sidepanel/workspace-context.ts` formats page context for the terminal, auto-injects it on workspace open, and queues route-refresh context while the user is typing.
9. The sidepanel renders live heuristic values immediately and can later replace them with `workspace_status_updated` audit snapshots without disturbing the terminal session.
10. Audit-backed snapshots also flow through `workspace-context.ts`, which injects a short audit summary into the terminal without bypassing the existing write guard.
11. Shared helpers in `src/lib/workspace-actions.ts` keep popup and hover audit/screenshot/note/recording triggers aligned with the workspace action row.
12. `TERMINAL_UI_STATE` remains the launcher visibility contract, so the hover launcher hides only while the sidepanel is open and remounts when it closes.

## Error and Recovery Paths

- If the terminal daemon is unavailable, the terminal region renders an inline unavailable state while the rest of the workspace shell stays mounted.
- If content status collection fails, background returns `unavailable` SEO/accessibility states and `not_measured` performance rather than fabricating values.
- If the sidepanel cannot fetch a status snapshot, terminal boot still succeeds and the shell stays usable.
- If queued terminal writes arrive while the user is typing, the write guard still defers them until idle.
- If a route change or audit result arrives while the user is typing, workspace context injection stays queued and never interrupts the active terminal input.

## State and Contracts

- `StorageKey.TERMINAL_UI_STATE` remains the source of truth for workspace visibility.
- `open_terminal_panel` and `terminal_panel_write` stay unchanged as the internal open/write runtime contracts.
- `get_workspace_status` requests a sidepanel snapshot from background.
- `kaboom_get_workspace_status` requests lightweight heuristics from the content script.
- `workspace_status_updated` lets the sidepanel replace live heuristics with explicit audit results.

## Code Paths

- `src/lib/request-audit.ts`
- `src/lib/workspace-actions.ts`
- `src/content/ui/tracked-hover-launcher.ts`
- `src/content/ui/terminal-panel-bridge.ts`
- `src/content/runtime-message-listener.ts`
- `src/content/workspace-status.ts`
- `src/background/message-handlers.ts`
- `src/background/workspace-status.ts`
- `src/sidepanel.ts`
- `src/sidepanel/workspace-context.ts`
- `src/sidepanel/workspace-shell.ts`
- `src/sidepanel/workspace-status.ts`
- `src/sidepanel/workspace-terminal-pane.ts`
- `src/types/runtime-messages.ts`
- `src/types/workspace-status.ts`

## Test Paths

- `tests/extension/sidepanel-terminal.test.js`
- `tests/extension/workspace-sidebar.test.js`
- `tests/extension/workspace-status.test.js`
- `tests/extension/tracked-hover-launcher.test.js`
- `tests/extension/workspace-actions.test.js`
- `tests/extension/popup-audit-button.test.js`
- `tests/extension/request-audit.test.js`

## Edit Guardrails

- Keep the PTY server, iframe host, and `terminal_panel_write` path intact while evolving the shell.
- Keep live status calculators deterministic and local-only; missing signals must degrade to `unavailable`/`not_measured`.
- Do not let popup and hover action paths drift away from `src/lib/workspace-actions.ts`.
- Preserve launcher visibility behavior keyed off `TERMINAL_UI_STATE === 'open'`.
