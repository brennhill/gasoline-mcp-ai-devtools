# Terminal Side Panel Design

## Goal

Move the terminal out of the page overlay and into the extension side panel while keeping the existing page hover launcher for screenshot and recording actions. The terminal button in the hover launcher should open the side panel, hide the launcher while the panel is open, and restore the launcher when the panel closes.

## Problem Statement

The current terminal UI is mounted inside page context through the content-script overlay. That works for many pages, but it inherits page/CSP constraints and keeps terminal host logic tied to the page DOM. The desired model is:

- page hover launcher stays available for quick actions
- terminal is hosted only in the side panel
- the side panel can later grow an action palette above the terminal
- some palette items auto-send, some ask questions first, and some may emit extension actions instead of terminal text

## Constraints

- No in-page xterm loader for terminal rendering.
- Hover launcher remains the page overlay for non-terminal actions.
- Opening the terminal should hide the hover launcher.
- Closing the side panel should restore the hover launcher.
- Terminal session persistence and reconnect behavior should remain local-first and singleton-based.
- Existing terminal server and PTY lifecycle should stay intact.
- The design should support both terminal command generation and higher-level extension actions from the same palette pipeline.

## Proposed Architecture

### 1. Hover Launcher

The tracked hover launcher continues to render in page context, but its terminal button becomes a launcher action instead of a terminal host action.

Responsibilities:

- render quick actions like screenshot and recording controls
- open the side panel when the terminal button is clicked
- hide itself while the side panel is open
- restore itself when the panel closes

### 2. Side Panel Shell

The side panel becomes the only terminal host.

Layout:

- upper region: action palette / command builder / form-driven flows
- lower region: terminal viewport and session controls

Responsibilities:

- restore the persisted terminal session
- own the terminal iframe and terminal UI state
- host future palette entries that can build commands or extension actions
- coordinate auto-send vs review-before-send behavior

### 3. Shared Terminal Core

Terminal session management stays split from the UI shell so the panel can reuse the existing behavior without page DOM dependencies.

Responsibilities:

- session start/validate/restore/stop
- storage/session persistence
- server URL derivation
- write queue / typing guard behavior
- terminal scrollback and reconnect handling

## Flow

1. User clicks the terminal button in the hover launcher.
2. The launcher asks the background worker to open the side panel.
3. The background worker opens the panel for the active tab.
4. The panel loads and restores the singleton terminal session from storage.
5. The panel connects to the dedicated terminal server on `main_port + 1`.
6. The hover launcher hides while the panel is open.
7. When the panel closes, the hover launcher becomes visible again.

## Palette Behavior

The palette is intentionally flexible and should not be over-specified in v1.

Supported output types:

- direct terminal command
- extension action / tool call
- mixed flow where the palette builds a prompt or command first and then routes it to one or both targets

Send modes:

- auto-send for simple actions
- question-gated send for ambiguous or parameterized actions
- optional review step before dispatch for more complex actions

The first version can ship with the palette shell and execution hooks even if the set of palette templates is small.

## Error Handling

- If side panel opening fails, the launcher should report a clear local error.
- If the terminal server is unavailable, the panel should show the existing terminal-unavailable state.
- If the persisted session is stale, the panel should clear it and start a fresh session.
- If the panel closes while commands are queued, pending writes should be dropped or safely reset rather than replayed into a closed host.

## Testing Strategy

- Update launcher tests so the terminal button opens the side panel rather than mounting an overlay.
- Add panel-host tests for session restore, open/close behavior, and terminal rendering.
- Add bridge tests for launcher-to-panel coordination and for terminal write forwarding from launcher-triggered actions.
- Remove obsolete tests that assert page-mounted terminal behavior.

## Documentation Contract

The implementation must update:

- the canonical flow map in `docs/architecture/flow-maps/`
- the terminal feature pointer in `docs/features/feature/terminal/flow-map.md`
- the terminal feature index and specs in `docs/features/feature/terminal/`
- the tab-tracking UX docs if the launcher behavior changes there
- the flow-maps README when a new canonical map is added

