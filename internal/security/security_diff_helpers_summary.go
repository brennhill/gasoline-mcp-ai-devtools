// Purpose: Builds severity-categorized summaries from security diff regressions and improvements.
// Why: Separates summary construction from individual diff comparison helpers.
package security

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

func buildDiffSummary(regressions, improvements []SecurityChange) SecurityDiffSummary {
	bySeverity := make(map[string]int)
	byCategory := make(map[string]int)

	for _, r := range regressions {
		bySeverity[r.Severity]++
		byCategory[r.Category]++
	}

	return SecurityDiffSummary{
		TotalRegressions:  len(regressions),
		TotalImprovements: len(improvements),
		BySeverity:        bySeverity,
		ByCategory:        byCategory,
	}
}

// formatDuration delegates to util.FormatDuration for human-readable duration formatting.
func formatDuration(d time.Duration) string {
	return util.FormatDuration(d)
}
