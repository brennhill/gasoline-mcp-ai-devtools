---
doc_type: feature_flow_map_pointer
feature_id: feature-tab-recording
status: active
last_reviewed: 2026-03-05
canonical_flow_map: docs/architecture/flow-maps/tab-recording-and-media-ingest.md
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Flow Map Pointer

Canonical flow map:

- [Tab Recording and Media Ingest](../../../architecture/flow-maps/tab-recording-and-media-ingest.md)
- [Shared Extraction and Contract Normalization](../../../architecture/flow-maps/shared-extraction-and-contract-normalization.md)

Latest sync update (2026-03-05): MCP-initiated recording now requires popup approval (`Approve` / `Deny`) via `gasoline_pending_recording` + `RECORDING_GESTURE_GRANTED|DENIED`, while interact aliases `record_start`/`record_stop` continue routing to `screen_recording_start`/`screen_recording_stop`.
