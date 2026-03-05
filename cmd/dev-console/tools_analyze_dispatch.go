// Purpose: Centralizes analyze mode routing, aliases, and canonical-mode validation.
// Why: Keeps top-level dispatch concerns separated from mode-specific analyze handlers.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/observe"
)

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]ModeHandler{
	"dom":                method((*ToolHandler).toolQueryDOM),
	"api_validation":     method((*ToolHandler).toolValidateAPI),
	"page_summary":       method((*ToolHandler).toolAnalyzePageSummary),
	"performance":        obs(observe.CheckPerformance),
	"accessibility":      obs(observe.RunA11yAudit),
	"error_clusters":     obs(observe.AnalyzeErrors),
	"navigation_patterns": obs(observe.AnalyzeHistory),
	"security_audit":     method((*ToolHandler).toolAnalyzeSecurityAudit),
	"third_party_audit":  method((*ToolHandler).toolAuditThirdParties),
	"link_health":        method((*ToolHandler).toolAnalyzeLinkHealth),
	"link_validation":    method((*ToolHandler).toolValidateLinks),
	"annotations":        method((*ToolHandler).toolGetAnnotations),
	"annotation_detail":  method((*ToolHandler).toolGetAnnotationDetail),
	"draw_history":       method((*ToolHandler).toolListDrawHistory),
	"draw_session":       method((*ToolHandler).toolGetDrawSession),
	"computed_styles":    toolComputedStyles,
	"forms":              toolFormDiscovery,
	"form_state":         toolFormState,
	"form_validation":    toolFormValidation,
	"data_table":         toolDataTable,
	"visual_baseline":    method((*ToolHandler).toolVisualBaseline),
	"visual_diff":        method((*ToolHandler).toolVisualDiff),
	"visual_baselines":   method((*ToolHandler).toolListVisualBaselines),
	"navigation":         method((*ToolHandler).toolAnalyzeNavigation),
	"page_structure":     method((*ToolHandler).toolAnalyzePageStructure),
	"audit":              method((*ToolHandler).toolAnalyzeAudit),
	"feature_gates": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.interactAction().handleContentExtraction(req, args, "feature_gates", "feature_gates")
	},
}

// analyzeAliases maps shorthand names to their canonical analyze mode names.
var analyzeAliases = map[string]string{
	"a11y":    "accessibility",
	"history": "navigation_patterns",
}

// analyzeAliasParams defines the deprecated alias parameters for the analyze tool.
var analyzeAliasParams = []modeAlias{
	{JSONField: "mode"},
	{JSONField: "action"},
}

// getValidAnalyzeModes returns a sorted, comma-separated list of valid analyze modes.
func getValidAnalyzeModes() string { return sortedMapKeys(analyzeHandlers) }

// analyzeRegistry is the tool registry for analyze dispatch.
var analyzeRegistry = toolRegistry{
	Handlers:  analyzeHandlers,
	AliasDefs: analyzeAliasParams,
	Resolution: modeResolution{
		ToolName:   "analyze",
		ValidModes: "", // populated lazily
		Aliases:    analyzeAliases,
	},
}

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	reg := analyzeRegistry
	reg.Resolution.ValidModes = getValidAnalyzeModes()
	return h.dispatchTool(req, args, reg)
}
