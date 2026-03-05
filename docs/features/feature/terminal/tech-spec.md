---
doc_type: tech_spec
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-03-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Tech Spec

## Primary Components

- `cmd/dev-console/terminal_server.go`
- `cmd/dev-console/terminal_handlers.go`
- `cmd/dev-console/terminal_assets/terminal.html`
- `src/content/ui/terminal-widget.ts`
- `extension/content/ui/terminal-widget.js`
- `internal/pty/manager.go`
- `internal/pty/session.go`

## Core Contracts

- Terminal server runs on `main_port + 1` to isolate WebSocket behavior from MCP request timeouts.
- Terminal session is singleton and restored via `chrome.storage.session`.
- Auto-write queue must defer sends while user typing/focus is active.
- Queued submit must wait for reconnect if WebSocket is disconnected.
- Redraw recovery must not terminate PTY session.

## Failure Modes

- Port bind conflict on terminal server startup.
- WebSocket disconnect during queued auto-submit.
- Corrupted overlay geometry or terminal canvas requiring redraw.
- Stale persisted session token after daemon restart.

## Linked Specs

- Product Spec: [product-spec.md](./product-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Feature Index: [index.md](./index.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Flow Map: [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)
