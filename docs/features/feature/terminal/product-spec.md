---
doc_type: product_spec
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-03-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Product Spec

## Objective

Provide a reliable in-browser terminal overlay for Gasoline users that stays usable during debugging, annotation, and automation workflows without disrupting the primary MCP daemon.

## Scope

- Terminal overlay UI with open/minimized/closed states.
- Dedicated terminal HTTP server on `main_port + 1`.
- PTY-backed singleton terminal session across tabs.
- Typing-aware queue guard for auto-writes from annotation workflows.
- Reconnect-safe submit behavior and lightweight redraw recovery.

## User Outcomes

- Users can keep terminal context visible while interacting with the page.
- Auto-generated terminal commands do not interrupt active typing.
- Terminal sessions survive page refreshes and reconnect cleanly.
- Terminal transport failures do not bring down the main MCP daemon.

## Non-Goals

- Multi-tenant terminal sessions per tab (current model is singleton).
- Cloud terminal hosting (all terminal behavior is local-first).
- Replacing full IDE terminal features.

## Linked Specs

- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Feature Index: [index.md](./index.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Flow Map: [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)
