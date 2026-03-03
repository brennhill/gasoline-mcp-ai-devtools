---
doc_type: feature_index
feature_id: feature-state-time-travel
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - cmd/dev-console/tools_interact_state_handler.go
  - cmd/dev-console/tools_interact_state_capture.go
  - cmd/dev-console/tools_interact_state_save_load.go
  - cmd/dev-console/tools_interact_state_list_delete.go
test_paths:
  - cmd/dev-console/tools_interact_state_test.go
  - cmd/dev-console/tools_interact_gate_test.go
---

# State Time Travel

## TL;DR

- Status: proposed
- Tool: observe
- Mode/Action: history
- Location: `docs/features/feature/state-time-travel`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_STATE_TIME_TRAVEL_001
- FEATURE_STATE_TIME_TRAVEL_002
- FEATURE_STATE_TIME_TRAVEL_003

## Code and Tests

- State sub-handler wiring:
  - `cmd/dev-console/tools_interact_state_handler.go`
- State capture and restore queueing:
  - `cmd/dev-console/tools_interact_state_capture.go`
- Save/load handlers:
  - `cmd/dev-console/tools_interact_state_save_load.go`
- List/delete handlers:
  - `cmd/dev-console/tools_interact_state_list_delete.go`
- Tests:
  - `cmd/dev-console/tools_interact_state_test.go`
  - `cmd/dev-console/tools_interact_gate_test.go`
