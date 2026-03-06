# Common Patterns (Required)

This file defines the default implementation patterns for extension and MCP changes.
Use this as a hard checklist during design, coding, and review.

## 1) Shared State Access

- Use feature helpers/modules for shared keys instead of new inline `chrome.storage.local` logic.
- For tab tracking, route through tab-state helpers and keep key usage centralized.
- For recording/pending-intent state, keep reads/writes in recording modules and avoid copy/paste storage flows in unrelated files.

## 2) Multi-Entry-Point Actions

- If behavior is reachable from keyboard, context menu, popup, and MCP, implement one shared toggle/start-stop helper.
- Entry points should only do minimal input mapping and call the shared helper.
- Do not duplicate stop/start branching logic per entry point.

## 3) Cross-Context Message Contracts

- Define message contracts in `src/types/runtime-messages.ts` first.
- Keep names, payload shape, and response semantics consistent across popup/background/content/offscreen.
- If a message crosses Go/TS boundary, update wire/schema definitions in the same change.

## 4) User-Facing Recording UX

- Use shared label/toast/badge helpers so wording and truncation stay consistent.
- Do not hardcode new recording status text in multiple modules.
- When replacing UX mechanisms (example: watermark -> badge), remove old behavior and align tests immediately.

## 5) Duplicate Code Policy

- Run:
  - `npx jscpd src/background src/popup --min-lines 8 --min-tokens 60`
- For each non-trivial clone:
  - Extract to a helper, or
  - Keep intentionally and add a short comment explaining why extraction is worse (performance, isolation, sandbox constraints, etc.).

## 6) Tests for End-to-End Data Passing

- Any cross-context flow change must include:
  - producer-side unit coverage,
  - consumer-side unit coverage,
  - one end-to-end/smoke assertion of payload shape and behavior.
- If behavior changes, update/remove stale tests in the same PR; do not leave failing legacy assertions.

## Review Checklist

- [ ] Storage access follows helper/module boundaries.
- [ ] Multi-entry-point behavior uses a shared helper path.
- [ ] Runtime message contract is typed and synchronized.
- [ ] UX labels/toasts/badges come from shared utilities.
- [ ] `jscpd` run completed and clones were resolved or documented.
- [ ] Unit + e2e/smoke tests reflect current behavior and pass.
