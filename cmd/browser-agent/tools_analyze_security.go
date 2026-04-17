// Purpose: Security analysis summary builders and shared severity ordering.
// Why: Keeps summary construction helpers available for audit scoring and tests.
// Docs: docs/features/feature/security-hardening/index.md
package main

// severityOrder maps severity names to sort priority (lower = more severe).
// Shared by page issues summary and security/third-party summary builders.
var severityOrder = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

// extractIssueMessage extracts a human-readable message from an issue map.
// Kept in main for backward compatibility with tests that use it directly.
func extractIssueMessage(issue map[string]any) string {
	for _, key := range []string{"message", "title", "description", "rule", "url"} {
		if v, ok := issue[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
