// Purpose: Handles accessibility audit execution and compact summary shaping for observe/analyze modes.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"sort"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

// RunA11yAudit executes an accessibility audit via the extension.
func RunA11yAudit(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector     string   `json:"selector"`
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
		Frame        any      `json:"frame"`
		Summary      bool     `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" && params.Selector != "" {
		params.Scope = params.Selector
	}

	enabled, _, _ := deps.GetCapture().GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrNoData, "No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	result, err := deps.ExecuteA11yQuery(params.Scope, params.Tags, params.Frame, params.ForceRefresh)
	if err != nil {
		// Issue #276: return partial results with error field instead of hard failure.
		// This lets the caller know what happened while providing a usable response shape.
		partialResult := map[string]any{
			"violations":   []any{},
			"passes":       []any{},
			"incomplete":   []any{},
			"inapplicable": []any{},
			"error":        err.Error(),
			"partial":      true,
			"summary": map[string]any{
				"violation_count":    0,
				"pass_count":         0,
				"incomplete_count":   0,
				"inapplicable_count": 0,
			},
		}
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit (partial — "+err.Error()+")", partialResult)}
	}

	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")}
	}

	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit", buildA11ySummary(auditResult))}
	}

	ensureA11ySummary(auditResult)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit", auditResult)}
}

// ensureA11ySummary adds a summary map if the audit result doesn't already have one.
// NOTE: Go uses snake_case (violation_count, pass_count, incomplete_count, inapplicable_count)
// while TS uses bare names (violations, passes, incomplete, inapplicable).
// TODO(#276): Unify summary field naming between Go (violation_count) and TS (violations).
func ensureA11ySummary(auditResult map[string]any) {
	if _, ok := auditResult["summary"]; ok {
		return
	}
	violations, _ := auditResult["violations"].([]any)
	passes, _ := auditResult["passes"].([]any)
	incomplete, _ := auditResult["incomplete"].([]any)
	inapplicable, _ := auditResult["inapplicable"].([]any)
	auditResult["summary"] = map[string]any{
		"violation_count":    len(violations),
		"pass_count":         len(passes),
		"incomplete_count":   len(incomplete),
		"inapplicable_count": len(inapplicable),
	}
}

// buildA11ySummary creates a compact representation of an a11y audit result.
func buildA11ySummary(auditResult map[string]any) map[string]any {
	passes, _ := auditResult["passes"].([]any)
	violations, _ := auditResult["violations"].([]any)
	incomplete, _ := auditResult["incomplete"].([]any)

	type issueInfo struct {
		rule     string
		severity string
		count    int
	}
	issues := make([]issueInfo, 0, len(violations))
	for _, v := range violations {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := vMap["id"].(string)
		impact, _ := vMap["impact"].(string)
		nodes, _ := vMap["nodes"].([]any)
		issues = append(issues, issueInfo{rule: rule, severity: impact, count: len(nodes)})
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].count > issues[j].count
	})
	topN := 5
	if len(issues) < topN {
		topN = len(issues)
	}
	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"rule":     issues[i].rule,
			"count":    issues[i].count,
			"severity": issues[i].severity,
		}
	}

	return map[string]any{
		"pass":       len(passes),
		"violations": len(violations),
		"incomplete": len(incomplete),
		"top_issues": topIssues,
	}
}
