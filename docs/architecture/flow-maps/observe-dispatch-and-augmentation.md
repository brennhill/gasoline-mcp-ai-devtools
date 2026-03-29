---
doc_type: flow_map
flow_id: observe-dispatch-and-augmentation
status: active
last_reviewed: 2026-03-28
owners:
  - Brenn
entrypoints:
  - cmd/browser-agent/tools_observe.go:toolObserve
  - cmd/browser-agent/tool_dispatch_helpers.go:resolveToolMode
code_paths:
  - cmd/browser-agent/tools_observe.go
  - cmd/browser-agent/tool_dispatch_helpers.go
  - cmd/browser-agent/tools_observe_registry.go
  - cmd/browser-agent/tools_observe_response.go
  - cmd/browser-agent/tools_observe_analysis.go
  - cmd/browser-agent/tools_shared_queries.go
  - internal/a11ysummary/summary.go
  - cmd/browser-agent/tools_observe_bundling.go
  - internal/tools/observe/
  - src/lib/brand.ts
  - src/lib/context.ts
  - src/content/message-forwarding.ts
  - src/content/runtime-message-listener.ts
  - src/content/window-message-listener.ts
test_paths:
  - cmd/browser-agent/tools_observe_handler_test.go
  - cmd/browser-agent/tools_observe_blackbox_test.go
  - cmd/browser-agent/tools_observe_audit_test.go
  - cmd/browser-agent/tools_observe_analysis_test.go
  - internal/a11ysummary/summary_test.go
  - cmd/browser-agent/tools_observe_unit_test.go
  - cmd/browser-agent/tools_schema_parity_test.go
  - tests/extension/content.test.js
  - tests/extension/runtime-log-branding.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Observe Dispatch and Augmentation

## Scope

Covers the `observe` tool entrypoint, mode selection, handler dispatch, post-dispatch response augmentation, and the content-script forwarding bridge that keeps page-context capture flowing into extension-side observe buffers.

## Entrypoints

- `toolObserve` delegates to `dispatchTool` with `observeRegistry`.
- `resolveToolMode` (shared) normalizes `what` plus deprecated aliases (`mode`, `action`).

## Primary Flow

1. MCP client sends `tools/call` with `name: "observe"`.
2. `toolObserve` parses request args and validates selector params.
3. `resolveToolMode` canonicalizes mode and applies alias mapping.
4. `dispatchTool` looks up canonical mode in `observeHandlers`.
5. Handler executes:
6. Most read modes delegate to `internal/tools/observe`.
7. Async/recording-related modes stay in local handler methods.
8. Response is post-processed:
9. Adds disconnect warning for extension-dependent modes.
10. Appends pending alerts as a second content block.
11. Alias usage warning is appended when deprecated params were used.
12. Page-context capture reaches the observe buffers through `window-message-listener.ts` and `message-forwarding.ts`, which map inject-side events to background runtime messages.

## Error and Recovery Paths

- Invalid JSON args return `ErrInvalidJSON`.
- Missing mode returns `ErrMissingParam` with valid mode hint.
- Unknown mode returns `ErrUnknownMode` with canonical mode list.
- Conflicting `what` vs alias values return alias conflict response.
- For `network_bodies`, empty-result hints incorporate active filters (`url`, `method`, `status_*`, `body_path`) so recovery guidance matches the exact query.
- If the extension reloads while an old content script remains on the page, `safeSendMessage` emits a Kaboom-branded refresh warning once and stops retrying stale bridge sends until the page is refreshed.

## State and Contracts

- `observeHandlers` is the source of truth for mode availability.
- `serverSideObserveModes` defines which modes skip disconnect warnings.
- Schema parity tests must stay aligned with `observeHandlers` keys.
- Accessibility summary payloads are normalized through `internal/a11ysummary` so canonical keys (`violations`, `passes`, `incomplete`, `inapplicable`) and legacy aliases (`*_count`) remain synchronized.
- `websocket_status` honors `summary:true` by returning compact connection/url previews instead of full connection objects.
- `MESSAGE_MAP` in `src/content/message-forwarding.ts` is the source of truth for inject-to-background capture event routing used by extension-backed observe modes, including the Kaboom-branded `kaboom_enhanced_action` event.
- `KABOOM_LOG_PREFIX` in `src/lib/brand.ts` is the shared runtime log label for content-side observe helpers like context annotation validation and sender rejection diagnostics.
- The pre-inject early-patch stash globals used for fetch/XHR/WebSocket adoption are now Kaboom-scoped (`__KABOOM_ORIGINAL_*`, `__KABOOM_EARLY_*`) across `src/early-patch.ts`, `src/lib/network.ts`, and `src/lib/websocket.ts`.

## Code Paths

- `cmd/browser-agent/tools_observe.go`
- `cmd/browser-agent/tool_dispatch_helpers.go`
- `cmd/browser-agent/tools_observe_registry.go`
- `cmd/browser-agent/tools_observe_response.go`
- `cmd/browser-agent/tools_observe_analysis.go`
- `cmd/browser-agent/tools_shared_queries.go`
- `cmd/browser-agent/tools_observe_bundling.go`
- `internal/a11ysummary/summary.go`
- `internal/tools/observe/`
- `src/lib/brand.ts`
- `src/content/message-forwarding.ts`
- `src/content/window-message-listener.ts`

## Test Paths

- `cmd/browser-agent/tools_observe_handler_test.go`
- `cmd/browser-agent/tools_observe_blackbox_test.go`
- `cmd/browser-agent/tools_observe_audit_test.go`
- `cmd/browser-agent/tools_observe_analysis_test.go`
- `internal/a11ysummary/summary_test.go`
- `cmd/browser-agent/tools_observe_unit_test.go`
- `cmd/browser-agent/tools_schema_parity_test.go`
- `tests/extension/content.test.js`

## Edit Guardrails

- Keep mode registry changes in `tools_observe_registry.go`.
- Keep argument parsing/validation in `tools_observe.go`.
- Keep response decoration in `tools_observe_response.go`.
- Keep content-bridge recovery warnings user-facing, Kaboom-branded, and one-shot after extension invalidation.
- Update this flow map and observe feature index when mode keys or file ownership changes.
