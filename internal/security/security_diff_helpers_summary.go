package security

import (
	"fmt"
	"time"
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

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm%02ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}
