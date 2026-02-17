// Purpose: Owns session-types.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// session-types.go — Session tracking types.
// SessionTracker records performance snapshots for delta computation.
package capture

import (
	"github.com/dev-console/dev-console/internal/performance"
)

// SessionTracker records the first and last performance snapshots for delta computation.
// Local to capture package; tracks per-URL first snapshot for session-level regression detection.
type SessionTracker struct {
	FirstSnapshots map[string]performance.PerformanceSnapshot // first snapshot per URL
	SnapshotCount  int                                        // total snapshots added this session
}
