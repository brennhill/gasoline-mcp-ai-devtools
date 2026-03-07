---
doc_type: qa_plan
feature_id: feature-terminal
status: shipped
last_reviewed: 2026-03-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# QA Plan

## Automated Gates

1. `go test ./cmd/browser-agent -run Terminal -count=1`
2. `go test ./internal/pty/...`
3. `node --test tests/extension/terminal-widget.test.js`
4. `npm run docs:check:strict`

## Manual Checks

1. Open terminal overlay, minimize, restore, and close.
2. Verify redraw (`↻`) resets widget geometry without killing session.
3. Type in terminal while annotation auto-write is triggered and confirm queued behavior.
4. Simulate WebSocket disconnect during queued submit and confirm submit resumes after reconnect.
5. Confirm terminal health reports `terminal_port` when running and `0` when unavailable.

## Regression Focus

- Focus theft while user is typing in xterm.
- Frame-write concurrency corruption under ping/output/control traffic.
- PTY session loss across page refresh.
- Main daemon stability when terminal bind/start fails.

## Linked Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- Feature Index: [index.md](./index.md)
- Flow Map Pointer: [flow-map.md](./flow-map.md)
- Canonical Flow Map: [terminal-server-isolation.md](../../../architecture/flow-maps/terminal-server-isolation.md)
