---
doc_type: feature_flow_map_pointer
feature_id: feature-backend-log-streaming
status: active
last_reviewed: 2026-03-05
canonical_flow_map: ../../../architecture/flow-maps/capture-buffer-store.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Backend Log Streaming Flow Map

Canonical flow maps:

- [Capture Buffer Store Extraction](../../../architecture/flow-maps/capture-buffer-store.md)
- [Shared Extraction and Contract Normalization](../../../architecture/flow-maps/shared-extraction-and-contract-normalization.md)
- [DRY Test Helpers and Daemon Header Consolidation](../../../architecture/flow-maps/dry-test-helper-and-daemon-header-consolidation.md)

Latest sync update (2026-03-05): `/sync` tests now reuse shared request/decode helpers (`sync_test_helpers_test.go`) to keep command lifecycle assertions consistent.
