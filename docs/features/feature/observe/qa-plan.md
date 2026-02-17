---
doc_type: qa-plan
feature_id: feature-observe
status: shipped
owners: []
last_reviewed: 2026-02-17
links:
  product: ./product-spec.md
  tech: ./tech-spec.md
  qa: ./qa-plan.md
  feature_index: ./index.md
---

# Observe QA Plan (TARGET)

## Automated Coverage
- `cmd/dev-console/tools_observe_handler_test.go`
- `cmd/dev-console/tools_observe_blackbox_test.go`
- `cmd/dev-console/tools_observe_audit_test.go`

## Required Scenarios
1. Enum contract
- Every `what` value from schema dispatches to a handler.

2. Pagination contract
- Cursor-based navigation works for logs.
- `restart_on_eviction` behavior is verified.

3. Command-result contract
- Pending, complete, error, expired, timeout states are all surfaced.
- Correlation lookup failures return structured terminal guidance.

4. Filtering correctness
- URL/status/method/level filters affect only intended modes.

5. Screenshot contract
- `observe(what:"screenshot")` returns capture metadata or structured timeout/error.

## Manual UAT
1. Call `configure(action:"health")`.
2. Call `observe(what:"logs")` with and without cursor options.
3. Queue an async command and verify `observe(what:"command_result", correlation_id)`.
4. Disconnect extension and verify warning + guidance surfaces.
