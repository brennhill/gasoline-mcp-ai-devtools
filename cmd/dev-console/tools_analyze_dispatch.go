// Purpose: Centralizes analyze mode routing, aliases, and canonical-mode validation.
// Why: Keeps top-level dispatch concerns separated from mode-specific analyze handlers.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/observe"
)

// AnalyzeHandler is the function signature for analyze tool handlers.
type AnalyzeHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]AnalyzeHandler{
	// Moved from configure
	"dom": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolQueryDOM(req, args)
	},
	"api_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolValidateAPI(req, args)
	},
	"page_summary": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzePageSummary(req, args)
	},

	// Delegated to internal/tools/observe
	"performance": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.CheckPerformance(h, req, args)
	},
	"accessibility": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.RunA11yAudit(h, req, args)
	},
	"error_clusters": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.AnalyzeErrors(h, req, args)
	},
	"navigation_patterns": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.AnalyzeHistory(h, req, args)
	},
	"security_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeSecurityAudit(req, args)
	},
	"third_party_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAuditThirdParties(req, args)
	},
	// New
	"link_health": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeLinkHealth(req, args)
	},
	"link_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolValidateLinks(req, args)
	},

	// Draw mode annotations
	"annotations": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetAnnotations(req, args)
	},
	"annotation_detail": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetAnnotationDetail(req, args)
	},
	"draw_history": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolListDrawHistory(req, args)
	},
	"draw_session": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetDrawSession(req, args)
	},

	// Inspect and visual (#79, #81, #82)
	"computed_styles": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolComputedStyles(h, req, args)
	},
	"forms": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolFormDiscovery(h, req, args)
	},
	"form_state": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolFormState(h, req, args)
	},
	"form_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolFormValidation(h, req, args)
	},
	"data_table": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return toolDataTable(h, req, args)
	},
	"visual_baseline": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolVisualBaseline(req, args)
	},
	"visual_diff": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolVisualDiff(req, args)
	},
	"visual_baselines": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolListVisualBaselines(req, args)
	},

	// SPA route discovery (#335)
	"navigation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeNavigation(req, args)
	},

	// Structural page analysis (#341)
	"page_structure": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzePageStructure(req, args)
	},

	// Combined audit report (#280)
	"audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolAnalyzeAudit(req, args)
	},

	// Feature gate detection (#345)
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

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	what, usedAliasParam, errResp := resolveToolMode(req, args, analyzeAliasParams, modeResolution{
		ToolName:   "analyze",
		ValidModes: getValidAnalyzeModes(),
		Aliases:    analyzeAliases,
	})
	if errResp != nil {
		return *errResp
	}

	handler, ok := analyzeHandlers[what]
	if !ok {
		validModes := getValidAnalyzeModes()
		resp := fail(req, ErrUnknownMode, "Unknown analyze mode: "+what, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes), describeCapabilitiesRecovery("analyze"))
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	resp := handler(h, req, args)
	return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
}
