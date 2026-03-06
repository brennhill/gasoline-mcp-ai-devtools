// Purpose: Package annotation — in-memory storage for draw-mode annotation sessions and details.
// Why: Preserves user-drawn annotations with TTL-based expiry so follow-up queries can reference them.
// Docs: docs/features/feature/annotated-screenshots/index.md

/*
Package annotation provides in-memory storage and TTL management for draw-mode
annotation sessions and their detailed DOM/style context.

Key types:
  - Annotation: lightweight rectangle + text annotation from draw mode.
  - AnnotationDetail: full computed styles and DOM context for a single annotation.
  - AnnotationStore: thread-safe store with per-entry TTL and named session support.

Key functions:
  - NewAnnotationStore: creates a store with configurable TTL duration.
  - Store: persists annotation results keyed by correlation ID.
  - Get: retrieves annotation details by correlation ID.
*/
package annotation
