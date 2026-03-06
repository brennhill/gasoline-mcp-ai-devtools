// Purpose: Exposes thread-safe accessor methods for capture buffer counters, timestamps, and snapshots.
// Why: Provides read-only observability APIs without leaking mutable capture internals.
// Docs: docs/features/feature/backend-log-streaming/index.md

// Layout:
// - accessor_counts_health.go: counters, snapshots, and health accessors
// - accessor_events.go: timestamp and event slice copy accessors
// - accessor_debug.go: debug log accessor/writer
// - accessor_performance.go: performance snapshot storage and retrieval
package capture
