// Purpose: Defines session-scoped snapshot tracking structures used for performance regression comparison.
// Why: Keeps per-session baseline bookkeeping explicit and separate from global capture buffers.
// Docs: docs/features/feature/performance-audit/index.md

package capture

import (
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/performance"
)

// SessionTracker records the first and last performance snapshots for delta computation.
// Local to capture package; tracks per-URL first snapshot for session-level regression detection.
type SessionTracker struct {
	FirstSnapshots map[string]performance.Snapshot // first snapshot per URL
	SnapshotCount  int                             // total snapshots added this session
}
