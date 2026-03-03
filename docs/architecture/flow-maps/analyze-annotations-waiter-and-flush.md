---
doc_type: architecture_flow_map
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
feature_ids:
  - feature-analyze-tool
  - feature-draw-mode
---

# Analyze Annotation Waiter and Flush Recovery

## Scope

This map covers annotation retrieval through `analyze(what:"annotations")`, including:
- Waiter registration (`wait:true`)
- Command-result completion after draw-mode store callbacks
- Recovery path for stuck pending waiters (`operation:"flush"`)
- Cross-project scope safety (`url` / `url_pattern`)

## Entrypoints

- `toolAnalyze` dispatch: `cmd/dev-console/tools_analyze_dispatch.go`
- Annotation handler: `cmd/dev-console/tools_analyze_annotations_handlers.go`
- Draw completion HTTP path: `cmd/dev-console/server_routes_media_draw_mode.go`
- Command-result polling: `cmd/dev-console/tools_async_observe_commands.go`

## Primary Flow

1. Client calls `analyze({what:"annotations", wait:true})` (optionally with `timeout_ms`).
2. Server checks for session newer than `MarkDrawStarted`.
3. If unavailable, server performs a bounded blocking wait:
   - `WaitForSession` / `WaitForNamedSession`
   - default 15s
   - caller override via `timeout_ms`
   - max 10m clamp
4. If annotations arrive during that wait, server returns final annotation payload directly (no polling).
5. If the wait times out, server:
   - registers command tracker record (`ann_*`)
   - registers annotation waiter
   - returns `waiting_for_user` + `correlation_id`.
6. User exits draw mode; extension posts draw result payload.
7. Server stores session (`StoreSession` / `AppendToNamedSession`).
8. Store callback completes matching `ann_*` command with session payload.
9. Client polls `observe({what:"command_result", correlation_id})` and receives terminal result.

## Scope Safety Flow

1. Client may pass `url` or `url_pattern` to scope annotation retrieval.
2. Server applies URL matching to returned annotation pages:
   - exact URL
   - base URL (e.g. `http://localhost:3000`)
   - wildcard/prefix patterns (e.g. `http://localhost:3000/*`)
   - origin-aware checks use parsed scheme/host equality, preventing prefix collisions (for example `:3000` does not match `:30001`)
3. Server builds project grouping metadata (`projects`) from page URLs.
4. If multiple projects are detected and no scope filter is provided:
   - response includes `scope_ambiguous: true`
   - response includes `scope_warning` with recommended filters
5. If both `url` and `url_pattern` are provided and differ:
   - request is rejected with `invalid_param`
   - caller must send a single unambiguous scope filter

## Error and Recovery Paths

- Stuck waiter / no callback completion:
  1. Client calls `analyze({what:"annotations", operation:"flush", correlation_id})`.
  2. Server removes waiter via `TakeWaiter`.
  3. If command is still pending, server force-completes it with current annotation snapshot.
  4. Response includes terminal reason:
     - `flushed` when data exists
     - `abandoned` when no annotations are available
- Already terminal command:
  - Flush is idempotent and returns existing terminal command state unchanged.

## State and Contracts

- Command lifecycle statuses: `pending`, `complete`, `error`, `timeout`, `expired`, `cancelled`.
- Annotation result payload adds `terminal_reason`:
  - `completed` (normal completion)
  - `flushed` (manual recovery completion with data)
  - `abandoned` (manual recovery completion with empty data)
- Annotation session payload includes scope metadata:
  - `filter_applied` (`none` when absent)
  - `projects` (grouped by base URL with recommended filters)
  - `scope_ambiguous` and `scope_warning` when cross-project risk is detected
- Analyze schema `operation` values include `flush` for annotation recovery.

## Code Paths

- `cmd/dev-console/tools_analyze_dispatch.go`
- `cmd/dev-console/tools_analyze_annotations_handlers.go`
- `cmd/dev-console/tools_async_observe_commands.go`
- `cmd/dev-console/tools_async_formatting.go`
- `internal/schema/analyze.go`
- `internal/tools/configure/mode_specs_analyze.go`
- `internal/annotation/store.go`
- `internal/annotation/store_sessions.go`
- `internal/annotation/store_named.go`
- `internal/annotation/store_wait.go`
- `internal/annotation/store_results.go`
- `internal/schema/analyze.go`

## Test Paths

- `cmd/dev-console/tools_analyze_annotations_test.go`
- `cmd/dev-console/tools_analyze_handler_test.go`
- `internal/annotation/store_test.go`
- `internal/annotation/named_test.go`

## Edit Guardrails

- Keep waiter registration and completion symmetric: every new waiter path must define a terminal completion path.
- Do not introduce stdout/stderr writes in command/result handlers (MCP framing safety).
- Any new terminal reason code must be documented here and in the feature tech spec.
- Schema changes for `analyze` must update golden tool list snapshots.
