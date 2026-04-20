// Purpose: Session-scoped storage primitives for tool state and metadata.
// Why: Centralizes on-disk read/write and validation logic behind a focused API.
// Docs: docs/features/feature/persistent-memory/index.md

/*
Package persistence provides validated namespace/key storage, metadata handling,
and background dirty-flush for session state.
*/
package persistence
