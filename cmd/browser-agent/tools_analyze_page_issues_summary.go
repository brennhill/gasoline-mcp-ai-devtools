// tools_analyze_page_issues_summary.go — Compact summary builder for page_issues results.
// Why: Reduces token cost by ~80% when AI only needs a high-level overview.
// Docs: docs/features/feature/auto-fix/index.md

package main

import "sort"

func buildPageIssuesSummary(result pageIssuesResult) map[string]any {
	allIssues := collectAllIssuesFlat(result.Sections)

	topN := pageIssuesSummaryTopN
	if len(allIssues) < topN {
		topN = len(allIssues)
	}

	sort.Slice(allIssues, func(i, j int) bool {
		return severityOrder[allIssues[i].severity] < severityOrder[allIssues[j].severity]
	})

	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"category": allIssues[i].category,
			"severity": allIssues[i].severity,
			"message":  allIssues[i].message,
		}
	}

	sectionSummaries := make(map[string]any)
	for name, sectionRaw := range result.Sections {
		section, ok := sectionRaw.(map[string]any)
		if !ok {
			continue
		}
		entry := map[string]any{
			"total": section["total"],
		}
		if errMsg, ok := section["error"].(string); ok && errMsg != "" {
			entry["error"] = errMsg
		}
		sectionSummaries[name] = entry
	}

	return map[string]any{
		"total_issues":     result.TotalIssues,
		"by_severity":      result.BySeverity,
		"top_issues":       topIssues,
		"sections":         sectionSummaries,
		"checks_completed": result.ChecksCompleted,
		"checks_skipped":   result.ChecksSkipped,
		"page_url":         result.PageURL,
	}
}

type flatIssue struct {
	category string
	severity string
	message  string
}

func collectAllIssuesFlat(sections map[string]any) []flatIssue {
	var all []flatIssue
	for category, sectionRaw := range sections {
		section, ok := sectionRaw.(map[string]any)
		if !ok {
			continue
		}
		issues, ok := section["issues"].([]map[string]any)
		if !ok {
			continue
		}
		for _, issue := range issues {
			sev, _ := issue["severity"].(string)
			all = append(all, flatIssue{
				category: category,
				severity: sev,
				message:  extractIssueMessage(issue),
			})
		}
	}
	return all
}

func extractIssueMessage(issue map[string]any) string {
	for _, key := range []string{"message", "title", "description", "rule", "url"} {
		if v, ok := issue[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
