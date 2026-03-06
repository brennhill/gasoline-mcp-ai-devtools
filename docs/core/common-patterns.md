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

## 7) CommandBuilder for Interact Handlers

New interact handlers that follow the standard guard/correlate/arm/enqueue/wait sequence
must use the `commandBuilder` pattern instead of repeating the boilerplate manually.

- **File:** `cmd/dev-console/tools_interact_command_builder.go`
- **Tests:** `cmd/dev-console/tools_interact_command_builder_test.go`

### When to Use

Use `commandBuilder` when a handler follows this sequence:
1. Run guard checks (pilot, extension, tab tracking)
2. Generate correlation ID
3. Arm evidence for the command
4. Enqueue a pending query
5. Optionally record an AI action
6. Wait for the command result

### Usage

```go
return h.newCommand("highlight").
    correlationPrefix("highlight").
    reason("highlight").
    queryType("highlight").
    queryParams(args).
    tabID(params.TabID).
    guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
    recordAction("highlight", "", map[string]any{"selector": params.Selector}).
    queuedMessage("Highlight queued").
    execute(req, args)
```

### Builder Methods

| Method | Purpose |
|--------|---------|
| `correlationPrefix(s)` | Prefix for generated correlation ID |
| `reason(s)` | Reason string for evidence arming |
| `queryType(s)` | PendingQuery.Type (e.g. "execute", "browser_action") |
| `queryParams(p)` | Pre-serialized query params |
| `buildParams(m)` | Build params from map (calls `buildQueryParams`) |
| `tabID(id)` | Tab ID for the pending query |
| `guards(fns...)` | Guard checks (run in order, first blocker short-circuits) |
| `guardsWithOpts(opts, fns...)` | Guards with StructuredError context options |
| `cspGuard(world)` | CSP check for world param |
| `preEnqueue(fn)` | Callback after correlation ID, before enqueue |
| `postEnqueue(fn)` | Callback after enqueue, before wait |
| `recordAction(action, url, extra)` | Record AI action after enqueue |
| `timeout(d)` | Override default enqueue timeout |
| `queuedMessage(msg)` | Message for async (queued) responses |
| `execute(req, args)` | Run the full sequence |
| `executeWithCorrelation(req, args)` | Like execute, also returns correlation ID |

### When NOT to Use

- Handlers with completely custom response logic (e.g. `handleDrawModeStart` returns a static response instead of `MaybeWaitForCommand`)
- Handlers on `*ToolHandler` (not `*interactActionHandler`) that create queries independently (e.g. `enrichNavigateResponse`)

## Review Checklist

- [ ] Storage access follows helper/module boundaries.
- [ ] Multi-entry-point behavior uses a shared helper path.
- [ ] Runtime message contract is typed and synchronized.
- [ ] UX labels/toasts/badges come from shared utilities.
- [ ] `jscpd` run completed and clones were resolved or documented.
- [ ] Unit + e2e/smoke tests reflect current behavior and pass.
- [ ] New interact handlers use `commandBuilder` when the standard guard/correlate/enqueue/wait pattern applies.
