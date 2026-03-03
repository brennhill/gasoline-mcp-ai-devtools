// Purpose: Builds and executes shared query patterns (accessibility audits) used by both observe and generate tools.
// Why: Prevents duplicated query construction logic when multiple tools need the same extension query type.
// Docs: docs/features/feature/enhanced-wcag-audit/index.md

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
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

	queryID, qerr := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "a11y",
			Params: paramsJSON,
		},
		30*time.Second,
		"",
	)
	if qerr != nil {
		return nil, qerr
	}
	return h.capture.WaitForResult(queryID, 30*time.Second)
}
