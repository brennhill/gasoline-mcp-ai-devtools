---
doc_type: feature_index
feature_id: feature-self-testing
status: in-progress
feature_type: feature
owners: []
last_reviewed: 2026-03-02
code_paths:
  - cmd/dev-console/server_routes.go
  - cmd/dev-console/testpages_http.go
  - cmd/dev-console/testpages_websocket.go
test_paths:
  - cmd/dev-console/testpages_test.go
---

# Self Testing

## TL;DR

- Status: in-progress
- Tool: interact, generate
- Mode/Action: execute_js, test
- Location: `docs/features/feature/self-testing`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)
- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- FEATURE_SELF_TESTING_001
- FEATURE_SELF_TESTING_002
- FEATURE_SELF_TESTING_003

## Code and Tests

- HTTP fixtures and embedded test pages: `cmd/dev-console/testpages_http.go`
- WebSocket harness and frame handling: `cmd/dev-console/testpages_websocket.go`
- Behavior tests: `cmd/dev-console/testpages_test.go`
