// Purpose: Transitional compatibility facade for legacy internal/ai imports.
// Why: Core implementations now live in focused packages (checkpoint/noise/persistence).
// Docs: docs/features/feature/push-alerts/index.md

/*
Package ai is a compatibility layer that re-exports types/functions from:

  - internal/checkpoint
  - internal/noise
  - internal/persistence

New code should import those focused packages directly.
*/
package ai
