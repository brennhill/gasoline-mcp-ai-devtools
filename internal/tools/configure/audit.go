// audit.go — Pure functions for audit log response building.
package configure

import "github.com/dev-console/dev-console/internal/audit"

// SummarizeAuditEntries aggregates audit entries into a summary map
// with tool call counts, success/failure rates, and session counts.
func SummarizeAuditEntries(entries []audit.AuditEntry) map[string]any {
	byTool := make(map[string]int)
	uniqueSessions := make(map[string]struct{})
	success := 0
	failed := 0
	for _, entry := range entries {
		byTool[entry.ToolName]++
		uniqueSessions[entry.AuditSessionID] = struct{}{}
		if entry.Success {
			success++
		} else {
			failed++
		}
	}

	return map[string]any{
		"total_calls":         len(entries),
		"success_count":       success,
		"failure_count":       failed,
		"audit_session_count": len(uniqueSessions),
		"calls_by_tool":       byTool,
	}
}
