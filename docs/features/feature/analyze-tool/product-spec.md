---
feature: analyze-tool
status: shipped
tool: analyze
version: v7.0
doc_type: product-spec
feature_id: feature-analyze-tool
last_reviewed: 2026-02-17
---

# Analyze Product Spec (TARGET)

## Purpose
Run analysis and command-driven browser inspection workflows that go beyond passive observation.

## Modes (`what`)
`dom`, `performance`, `accessibility`, `error_clusters`, `history`, `security_audit`, `third_party_audit`, `link_health`, `link_validation`, `page_summary`, `annotations`, `annotation_detail`, `api_validation`, `draw_history`, `draw_session`

## Behavior Model
- Sync by default.
- `background:true` (or `sync:false`/`wait:false`) returns queued handles.
- Correlation results are retrievable via `observe({what:"command_result"})`.

## Mode Classes
1. Extension-backed command modes
- `dom`, `accessibility`, `link_health`, `page_summary`

2. Server-side analysis modes
- `performance`, `error_clusters`, `history`, `security_audit`, `third_party_audit`, `link_validation`, `api_validation`

3. Annotation/session modes
- `annotations`, `annotation_detail`, `draw_history`, `draw_session`

## Requirements
- `ANALYZE_PROD_001`: `what` is required and enum-validated.
- `ANALYZE_PROD_002`: async control flags must produce deterministic queued/sync behavior.
- `ANALYZE_PROD_003`: extension-backed command modes must emit correlation IDs.
- `ANALYZE_PROD_004`: `dom` mode must remain the canonical DOM query surface.
- `ANALYZE_PROD_005`: mode-specific parameters must be validated with structured errors.
