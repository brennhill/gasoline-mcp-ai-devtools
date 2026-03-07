---
doc_type: flow_map
flow_id: push-inbox-screenshot-throttle
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - internal/push/inbox.go:Enqueue
  - cmd/browser-agent/tools_observe_inbox.go:appendPushPiggyback
  - cmd/browser-agent/tools_configure_state_impl.go:clearConfiguredBuffer
code_paths:
  - internal/push/inbox.go
  - internal/push/types.go
  - cmd/browser-agent/tools_observe_inbox.go
  - cmd/browser-agent/tools_configure_state_impl.go
test_paths:
  - internal/push/inbox_test.go
  - cmd/browser-agent/tools_observe_inbox_test.go
  - cmd/browser-agent/tools_configure_handler_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Push Inbox Screenshot Throttle

## Scope

Covers screenshot deduplication at the inbox level and screenshot capping at the piggyback delivery level. Also covers `configure(clear, buffer=all|inbox)` draining the push inbox.

## Entrypoints

1. `PushInbox.Enqueue` — Deduplicates consecutive screenshots from the same tab+URL on enqueue.
2. `appendPushPiggyback` — Caps piggybacked screenshots to the most recent one per tool response.
3. `clearConfiguredBuffer` — Drains the push inbox on `buffer=all` or `buffer=inbox`.

## Primary Flow

1. Extension pushes a screenshot via `POST /push/screenshot`.
2. Daemon creates a `PushEvent` and calls `PushInbox.Enqueue`.
3. **Dedup check**: If the last event in the queue is also a screenshot with the same `TabID` and `PageURL`, the new event replaces it instead of appending. This prevents rapid-fire identical screenshots from flooding the inbox.
4. On the next MCP tool response, `appendPushPiggyback` drains all inbox events.
5. **Cap check**: Only the most recent screenshot is included in the response. If earlier screenshots were skipped, a summary text block notes the count.
6. Non-screenshot events (chat, annotations) always pass through fully.

## Error and Recovery Paths

1. If inbox is nil, both piggyback and clear operations are no-ops.
2. If all events are non-screenshot, no capping occurs — all events pass through.
3. `configure(clear, buffer=all)` always drains the inbox regardless of content type.

## State and Contracts

1. Dedup is positional (last-event only), not content-hash based. If a non-screenshot event intervenes, no dedup occurs.
2. Screenshot cap is per-response, not per-session. Each tool call gets at most 1 screenshot.
3. The `inbox` buffer option is standalone — it only clears push events, not capture buffers.
4. DrainAll is destructive and atomic under mutex.

## Code Paths

- `internal/push/inbox.go` — Enqueue dedup logic
- `cmd/browser-agent/tools_observe_inbox.go` — Piggyback cap logic
- `cmd/browser-agent/tools_configure_state_impl.go` — Clear buffer inbox case

## Test Paths

- `internal/push/inbox_test.go` — TestInbox_ScreenshotDedup_* (6 tests)
- `cmd/browser-agent/tools_observe_inbox_test.go` — TestAppendPushPiggyback_Caps*, _NonScreenshot*, _Skipped*, _SingleScreenshotNoSkip* (4 tests)
- `cmd/browser-agent/tools_configure_handler_test.go` — TestToolsConfigureClear_AllDrainsInbox, _InboxBuffer, _SpecificBuffers/inbox (3 tests)

## Edit Guardrails

1. Dedup must only apply to consecutive screenshots with matching tab+URL. Never dedup across event types.
2. Cap applies to piggyback only — explicit `observe({what: "inbox"})` returns all events unfiltered.
3. Keep DrainAll as the single clear mechanism; do not add selective removal to the inbox.
