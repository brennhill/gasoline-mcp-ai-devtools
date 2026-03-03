---
doc_type: tech-spec
feature_id: feature-transient-capture
status: shipped
last_reviewed: 2026-03-03
---

# Transient Capture Tech Spec

## Architecture
- Content-side detection logic: `src/lib/transient-capture.ts`
- Daemon-side async wiring and retrieval: `cmd/dev-console/tools_async_transient.go`
- Observe integration tests: `internal/tools/observe/handlers_transients_test.go`

## Constraints
- Snapshot extraction must be lightweight and bounded.
- Captures should include correlation and timing metadata.
- Replay/observe consumers must see consistent event ordering.
