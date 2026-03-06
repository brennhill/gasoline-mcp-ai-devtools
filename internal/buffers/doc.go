// Purpose: Package buffers — fixed-capacity ring buffer with cursor-safe access and generic filtering.
// Why: Prevents unbounded memory growth while preserving recent evidence for concurrent readers.
// Docs: docs/features/feature/ring-buffer/index.md

/*
Package buffers provides a generic fixed-capacity circular buffer with cursor-based reads.

Key types:
  - RingBuffer[T]: generic circular buffer with FIFO eviction and monotonic position tracking.
  - BufferCursor: tracks a client's read position with timestamp for eviction detection.

Key functions:
  - NewRingBuffer: creates a ring buffer with the specified capacity.
  - ReverseFilterLimit: iterates a slice newest-first, applying a filter predicate with limit.
*/
package buffers
