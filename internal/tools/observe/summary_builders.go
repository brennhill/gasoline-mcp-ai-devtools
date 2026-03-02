// Purpose: Builds compact summary maps for network waterfall, errors, and log entries when summary=true.
// Docs: docs/features/feature/observe/index.md

// summary_builders.go — Compact summary builders for observe modes.
//
// Layout:
// - summary_builders_errors_logs.go: errors/log summaries and text truncation helpers
// - summary_builders_network_events.go: network/ws/action/history summaries
// - summary_builders_bundles.go: error bundle summary
// - summary_builders.go: paginated metadata helper
package observe

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pagination"
)

// BuildPaginatedMetadataWithSummary adds a summary block to first-page paginated responses.
func BuildPaginatedMetadataWithSummary(
	cap *capture.Capture, newestEntry time.Time,
	pMeta *pagination.CursorPaginationMetadata,
	isFirstPage bool,
	summaryFn func() map[string]any,
) map[string]any {
	var meta map[string]any
	if cap != nil {
		meta = BuildPaginatedResponseMetadata(cap, newestEntry, pMeta)
	} else {
		// Nil capture — build minimal metadata for testing
		meta = map[string]any{
			"total":    pMeta.Total,
			"has_more": pMeta.HasMore,
		}
		if pMeta.Cursor != "" {
			meta["cursor"] = pMeta.Cursor
		}
	}
	if isFirstPage && summaryFn != nil {
		meta["summary"] = summaryFn()
	}
	return meta
}
