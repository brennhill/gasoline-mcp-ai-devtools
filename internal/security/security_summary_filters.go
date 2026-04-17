// Purpose: Builds scan summaries and filters findings by severity and minimum threshold.
// Why: Separates summary construction and severity filtering from scan orchestration.
package security

import (
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

func buildSummary(findings []SecurityFinding, bodies []capture.NetworkBody) ScanSummary {
	bySeverity := make(map[string]int)
	byCheck := make(map[string]int)
	for _, f := range findings {
		bySeverity[f.Severity]++
		byCheck[f.Check]++
	}

	urlSet := make(map[string]bool)
	for _, b := range bodies {
		urlSet[b.URL] = true
	}

	return ScanSummary{
		TotalFindings: len(findings),
		BySeverity:    bySeverity,
		ByCheck:       byCheck,
		URLsScanned:   len(urlSet),
	}
}

func filterBodiesByURL(bodies []capture.NetworkBody, filter string) []capture.NetworkBody {
	if filter == "" {
		return bodies
	}
	var filtered []capture.NetworkBody
	for _, b := range bodies {
		if strings.Contains(b.URL, filter) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterBySeverity(findings []SecurityFinding, minSeverity string) []SecurityFinding {
	severityOrder := map[string]int{
		"info":     0,
		"low":      1,
		"medium":   2,
		"warning":  2,
		"high":     3,
		"critical": 4,
	}
	minLevel, ok := severityOrder[minSeverity]
	if !ok {
		return findings
	}

	var filtered []SecurityFinding
	for _, f := range findings {
		level := severityOrder[f.Severity]
		if level >= minLevel {
			filtered = append(filtered, f)
		}
	}
	return filtered
}
