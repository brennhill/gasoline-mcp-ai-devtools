---
doc_type: flow_map
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Capture Buffer Store Extraction

## Scope

Refactor `internal/capture` to group websocket/network/action ring-buffer state into
`BufferStore`, and extension-log append/eviction/copy/clear behavior into
`ExtensionLogBuffer` helpers, network-waterfall append/eviction/copy/clear behavior into
`NetworkWaterfallBuffer` store methods, and websocket status/reset behavior into
`WSConnectionTracker` store methods, plus performance snapshot/before-snapshot map behavior into
`PerformanceStore` store methods, reducing `Capture` field sprawl without changing behavior.

## Entrypoints

1. `internal/capture/capture-struct.go:NewCapture`
2. `internal/capture/network_bodies.go`
3. `internal/capture/websocket.go`
4. `internal/capture/enhanced_actions.go`
5. `internal/capture/buffer_clear.go`

## Primary Flow

1. `NewCapture` initializes `buffers: newBufferStore()`.
2. Ingestion methods append to `c.buffers.*` slices and bump `c.buffers.*TotalAdded`.
3. WebSocket lifecycle state updates are delegated to `WSConnectionTracker.trackEvent`.
4. Eviction methods trim `c.buffers.*` slices and keep memory counters in sync.
5. Accessor/read APIs expose cloned snapshots from `c.buffers.*` data.
6. Clear/reset APIs zero `c.buffers.*` slices and counters.
7. Extension log ingestion paths (`AddExtensionLogs`, sync ingest) call `ExtensionLogBuffer.append`.
8. Extension log reads/clears use `ExtensionLogBuffer.snapshot` and `ExtensionLogBuffer.clear`.
9. Network waterfall ingest/read/clear delegates to `NetworkWaterfallBuffer.appendEntries`, `snapshot`, and `clear`.
10. WebSocket status/read/clear delegates to `WSConnectionTracker.status`, `connectionCount`, and `clear`.
11. Performance snapshot ingest/read and before-snapshot consume-on-read delegates to `PerformanceStore` methods.

## Error and Recovery Paths

1. Parallel array mismatch repair still runs before append/read and truncates to min length.
2. Memory-pressure eviction still uses running memory totals and exact decrement per dropped entry.
3. Rate-limit/circuit paths remain unchanged because they consume accessor results, not raw fields.

## State and Contracts

1. `BufferStore` remains lock-free; synchronization stays in `Capture.mu`.
2. Monotonic counters (`*TotalAdded`) semantics are unchanged.
3. TTL filtering contracts still use timestamp slices (`wsAddedAt`, `networkAddedAt`, `actionAddedAt`).
4. Public `Capture` APIs are unchanged.

## Code Paths

- `internal/capture/buffer_store.go`
- `internal/capture/extension_log_store.go`
- `internal/capture/network_waterfall_store.go`
- `internal/capture/ws_connection_store.go`
- `internal/capture/performance_store.go`
- `internal/capture/capture-struct.go`
- `internal/capture/network_bodies.go`
- `internal/capture/websocket.go`
- `internal/capture/ws_connection_tracker.go`
- `internal/capture/enhanced_actions.go`
- `internal/capture/buffer_clear.go`
- `internal/capture/extension_logs.go`
- `internal/capture/network_waterfall.go`
- `internal/capture/sync_processing.go`
- `internal/capture/accessor_*.go`
- `internal/capture/memory.go`

## Test Paths

- `internal/capture/memory_test.go`
- `internal/capture/network_bodies_test.go`
- `internal/capture/websocket_test.go`
- `internal/capture/enhanced_actions_test.go`
- `internal/capture/extension_log_store_test.go`
- `internal/capture/network_waterfall_store_test.go`
- `internal/capture/ws_connection_store_test.go`
- `internal/capture/performance_store_test.go`
- `internal/capture/coverage_boost_unit_test.go`
- `internal/capture/test_helpers.go`

## Edit Guardrails

1. New buffer fields should be added to `BufferStore`, not directly to `Capture`.
2. Keep all `BufferStore` and `ExtensionLogBuffer` reads/writes under `Capture.mu` lock discipline.
3. Any counter reset behavior must be explicitly tested in `buffer_clear` and `memory` tests.
4. Preserve compatibility of `Capture` public methods; do not expose `BufferStore` directly.
