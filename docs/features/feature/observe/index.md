---
doc_type: feature_index
feature_id: feature-observe
status: shipped
feature_type: feature
owners: []
last_reviewed: 2026-03-05
code_paths:
  - cmd/dev-console/tools_observe.go
  - cmd/dev-console/tools_observe_registry.go
  - cmd/dev-console/tools_observe_response.go
  - cmd/dev-console/tools_observe_analysis.go
  - cmd/dev-console/tools_shared_queries.go
  - cmd/dev-console/tools_observe_bundling.go
  - cmd/dev-console/observe_filtering.go
  - internal/a11ysummary/summary.go
  - internal/tools/observe/analysis_a11y.go
  - internal/tools/observe/analysis_screenshot.go
  - internal/tools/observe/handlers_extension_logs.go
  - internal/tools/observe/handlers_logs.go
  - src/background/commands/observe.ts
  - src/lib/daemon-http.ts
  - src/inject/observers.ts
  - src/lib/network.ts
  - internal/capture/queries.go
  - internal/capture/sync.go
test_paths:
  - cmd/dev-console/tools_observe_handler_test.go
  - cmd/dev-console/tools_observe_blackbox_test.go
  - cmd/dev-console/tools_observe_audit_test.go
  - cmd/dev-console/tools_observe_screenshot_test.go
  - cmd/dev-console/tools_observe_analysis_test.go
  - extension/background/commands/observe.fullpage.test.js
  - internal/a11ysummary/summary_test.go
  - internal/tools/observe/analysis_test.go
  - internal/tools/observe/analysis_save_test.go
  - tests/extension/inject-console-network-exceptions.test.js
  - tests/extension/network-bodies.test.js
  - tests/extension/sync-client.test.js
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Observe

## TL;DR
- Status: shipped
- Tool: `observe`
- Mode key: `what`
- Contract source: `cmd/dev-console/tools_schema.go`

## Specs
- Product: `product-spec.md`
- Tech: `tech-spec.md`
- QA: `qa-plan.md`
- Flow Map: `flow-map.md`

## Canonical Note
`observe` is the passive read surface for captured browser/server state. It is the canonical polling surface for async command completion via `what:"command_result"`.

Accessibility (`what:"accessibility"`) normalizes `summary` counts with canonical keys (`violations`, `passes`, `incomplete`, `inapplicable`) and preserves legacy aliases (`*_count`) for compatibility.
WebSocket status (`what:"websocket_status"`) supports `summary:true` with compact URL/connection-id previews while preserving the full default payload when `summary` is omitted.
Network-bodies empty-result hints now echo all active filters (`url`, `method`, `status_*`, `body_path`) so retry guidance is specific to the current query.
`level` is a quiet alias for `min_level` — accepted at runtime but hidden from schema. Both use threshold semantics (e.g., `warn` returns warn+error).
