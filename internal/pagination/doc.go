// Purpose: Implement doc.go internal behavior used by MCP runtime features.
// Why: Maintains stable server behavior across tool and transport paths.
// Docs: docs/features/feature/pagination/index.md

// doc.go — Package documentation for cursor-based ring buffer pagination.

// Package pagination provides cursor-based pagination for ring buffers.
//
// Implements RFC-compliant cursor pagination for:
//   - Console logs (timestamp + sequence number)
//   - WebSocket events (timestamp + sequence number)
//   - User actions (timestamp + sequence number)
//   - Network bodies (no pagination - returns all matching entries)
//
// Cursor format: "timestamp:sequence" (e.g., "2026-01-30T10:15:23Z:42")
// Supports both after (forward) and before (backward) pagination with limit.
//
// Handles eviction gracefully:
//   - If cursor is expired (entry evicted from buffer), returns error
//   - Optionally allows restart=true to return oldest available instead
//
// All functions are pure - they don't modify the buffer, only filter and slice.
package pagination
