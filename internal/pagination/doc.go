// Purpose: Implement doc.go internal behavior used by MCP runtime features.
// Why: Maintains stable server behavior across tool and transport paths.
// Docs: docs/features/feature/pagination/index.md

// doc.go — Package documentation for cursor-based ring buffer pagination.

// Package pagination provides cursor-based pagination for ring buffers.
//
// Current live consumer: console logs only, via
// internal/tools/observe/handlers_logs.go. The generic
// ApplyCursorPagination[T Sequenced] shape is retained because the
// cursor parse/build helpers (ParseCursor, BuildCursor) and the
// eviction-restart semantics are reused there.
//
// Historical note: action and WebSocket-event specializations were
// deleted in the dead-code sweep because no production caller ever
// wired them up. If action/network/websocket pagination returns, the
// pattern to follow is the logs handler, not to re-introduce the
// removed Serialize* helpers.
//
// Cursor format: "timestamp:sequence" (e.g., "2026-01-30T10:15:23Z:42").
// Supports both after (forward) and before (backward) pagination with
// a limit.
//
// Handles eviction gracefully:
//   - If the cursor is expired (entry evicted from buffer), returns an
//     error.
//   - Optionally allows restart=true to return oldest available entries
//     instead.
//
// All functions are pure: they don't modify the buffer, only filter
// and slice.
package pagination
