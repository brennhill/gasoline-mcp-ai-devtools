// deps.go — Dependency interface for the observe tool package.
package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
)

// Deps provides all dependencies the observe handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.DiagnosticProvider

	// Capture buffer access.
	GetCapture() *capture.Capture

	// Log buffer access (read-only snapshots).
	GetLogEntries() ([]mcp.LogEntry, []time.Time)
	GetLogTotalAdded() int64

	// A11y query execution (shared with generate tool).
	ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
}
