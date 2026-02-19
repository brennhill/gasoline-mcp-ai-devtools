// tools_shared_queries.go — Cross-tool query helpers used by multiple MCP tools.
package main

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// buildA11yQueryParams constructs query params for an accessibility audit.
// Used by observe (toolA11yAudit) and generate (SARIF export).
func buildA11yQueryParams(scope string, tags []string, frame any, forceRefresh bool) map[string]any {
	queryParams := map[string]any{}
	if scope != "" {
		queryParams["scope"] = scope
	}
	if len(tags) > 0 {
		queryParams["tags"] = tags
	}
	if forceRefresh {
		queryParams["force_refresh"] = true
	}
	if frame != nil {
		queryParams["frame"] = frame
	}
	return queryParams
}

// executeA11yQuery runs an accessibility audit via the extension and waits for the result.
// Used by observe (toolA11yAudit) and generate (SARIF export).
func (h *ToolHandler) ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error) {
	queryParams := buildA11yQueryParams(scope, tags, frame, forceRefresh)
	// Error impossible: map contains only primitive types and string slices from input
	paramsJSON, _ := json.Marshal(queryParams)

	queryID := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "a11y",
			Params: paramsJSON,
		},
		30*time.Second,
		"",
	)
	return h.capture.WaitForResult(queryID, 30*time.Second)
}

// ensureA11ySummary adds a summary section to a11y audit results if not already present.
func ensureA11ySummary(auditResult map[string]any) {
	if _, ok := auditResult["summary"]; ok {
		return
	}
	violations, _ := auditResult["violations"].([]any)
	passes, _ := auditResult["passes"].([]any)
	auditResult["summary"] = map[string]any{
		"violation_count": len(violations),
		"pass_count":      len(passes),
	}
}
