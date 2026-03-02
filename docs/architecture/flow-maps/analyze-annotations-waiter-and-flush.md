---
doc_type: architecture_flow_map
status: active
last_reviewed: 2026-03-02
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

## Entrypoints

- `toolAnalyze` dispatch: `cmd/dev-console/tools_analyze_dispatch.go`
- Annotation handler: `cmd/dev-console/tools_analyze_annotations_handlers.go`
- Draw completion HTTP path: `cmd/dev-console/server_routes_media_draw_mode.go`
- Command-result polling: `cmd/dev-console/tools_async_observe_commands.go`

## Primary Flow

1. Client calls `analyze({what:"annotations", wait:true})`.
2. Server checks for session newer than `MarkDrawStarted`.
3. If unavailable, server:
   - registers command tracker record (`ann_*`)
   - registers annotation waiter
   - returns `waiting_for_user` + `correlation_id`.
4. User exits draw mode; extension posts draw result payload.
5. Server stores session (`StoreSession` / `AppendToNamedSession`).
6. Store callback completes matching `ann_*` command with session payload.
7. Client polls `observe({what:"command_result", correlation_id})` and receives terminal result.

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
- Analyze schema `operation` values include `flush` for annotation recovery.

## Code Paths

- `cmd/dev-console/tools_analyze_dispatch.go`
- `cmd/dev-console/tools_analyze_annotations_handlers.go`
- `cmd/dev-console/tools_async_observe_commands.go`
- `cmd/dev-console/tools_async_formatting.go`
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

