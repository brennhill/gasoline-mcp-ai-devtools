---
doc_type: tech-spec
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

# Observe Tech Spec (TARGET)

## Dispatcher
- Entry: `toolObserve` in `cmd/dev-console/tools_observe.go`
- Dispatch map: `observeHandlers` keyed by `what`

## Data Sources
1. Server log buffers (`errors`, `logs`, `timeline`, `error_bundles`)
2. Capture network/websocket/action buffers (`network_*`, `websocket_*`, `actions`)
3. Extension status and tracking state (`pilot`, `page`, `tabs`)
4. Command tracker (`command_result`, `pending_commands`, `failed_commands`)
5. Recording/video stores (`saved_videos`, `recordings`, `recording_actions`, `playback_results`, `log_diff_report`)

## Async Command Observation Path
- Command IDs are registered when active commands are queued.
- `toolObserveCommandResult` reads from command tracker.
- Annotation wait path supports blocking retrieval for draw mode correlation IDs (`ann_*`).

## Pagination and Filtering
- Cursor pagination implemented for logs via `internal/pagination`.
- Filter keys (`url`, `method`, `status_min`, `status_max`, `level`, `min_level`, etc.) are mode-specific.

## Error and Resilience Behavior
- Missing/unknown `what` yields structured errors.
- Disconnect warning is prepended for extension-dependent modes.
- Stale/expired command results are surfaced with explicit retry guidance.

## Code Anchors
- `cmd/dev-console/tools_observe.go`
- `cmd/dev-console/tools_observe_analysis.go`
- `cmd/dev-console/tools_observe_bundling.go`
- `internal/capture/queries.go`
