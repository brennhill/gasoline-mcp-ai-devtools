// Purpose: Dispatches analyze tool modes (dom, accessibility, performance, error_clusters, history, page_structure, etc.) to sub-handlers.
// Why: Acts as the top-level router for all active analysis that requires extension-side computation.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dev-console/dev-console/internal/analysis"
	"github.com/dev-console/dev-console/internal/queries"
	az "github.com/dev-console/dev-console/internal/tools/analyze"
	"github.com/dev-console/dev-console/internal/tools/observe"
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
	"history": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.AnalyzeHistory(h, req, args)
	},
	"security_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolSecurityAudit(req, args)
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
		return h.toolComputedStyles(req, args)
	},
	"forms": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolFormDiscovery(req, args)
	},
	"form_validation": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolFormValidation(req, args)
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
		return h.handleContentExtraction(req, args, "feature_gates", "feature_gates")
	},
}

// analyzeAliases maps shorthand names to their canonical analyze mode names.
var analyzeAliases = map[string]string{
	"a11y": "accessibility",
}

// getValidAnalyzeModes returns a sorted, comma-separated list of valid analyze modes.
func getValidAnalyzeModes() string {
	modes := make([]string, 0, len(analyzeHandlers))
	for mode := range analyzeHandlers {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What   string `json:"what"`
		Mode   string `json:"mode"`
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	what := params.What
	usedAliasParam := ""
	if what != "" && params.Mode != "" && params.Mode != what {
		return whatAliasConflictResponse(req, "mode", what, params.Mode, getValidAnalyzeModes())
	}
	if what != "" && params.Action != "" && params.Action != what {
		return whatAliasConflictResponse(req, "action", what, params.Action, getValidAnalyzeModes())
	}
	if what == "" {
		if params.Mode != "" {
			what = params.Mode
			usedAliasParam = "mode"
		} else if params.Action != "" {
			what = params.Action
			usedAliasParam = "action"
		}
	}

	if what == "" {
		validModes := getValidAnalyzeModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validModes))}
	}

	if alias, ok := analyzeAliases[what]; ok {
		what = alias
	}

	handler, ok := analyzeHandlers[what]
	if !ok {
		validModes := getValidAnalyzeModes()
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown analyze mode: "+what, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes))}
		return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
	}

	resp := handler(h, req, args)
	return appendCanonicalWhatAliasWarning(resp, usedAliasParam, what)
}

// ============================================
// DOM Query (moved from configure)
// ============================================

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector string `json:"selector"`
		TabID    int    `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Issue #274: selector is optional — default to "*" for full DOM dump.
	queryArgs := args
	if params.Selector == "" {
		var raw map[string]any
		if json.Unmarshal(args, &raw) != nil || raw == nil {
			raw = make(map[string]any)
		}
		raw["selector"] = "*"
		// Marshal cannot realistically fail with string/map values; silent fallback is acceptable.
		if marshaled, err := json.Marshal(raw); err == nil {
			queryArgs = marshaled
		}
	}

	// Generate correlation ID for tracking
	correlationID := newCorrelationID("dom")

	// Create pending query for DOM query
	query := queries.PendingQuery{
		Type:          "dom",
		Params:        queryArgs,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, queryArgs, "DOM query queued")
}

func (h *ToolHandler) toolAnalyzePageSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Delegates to the shared content extraction helper which handles gate checks
	// (pilot, extension, tab tracking), timeout validation, and query creation.
	return h.handleContentExtraction(req, args, "page_summary", "page_summary")
}

// ============================================
// API Validation (moved from configure)
// ============================================

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	switch params.Operation {
	case "analyze":
		h.processAPIValidationBodies()
		filter := h.apiValidationFilter(params.URLFilter, params.IgnoreEndpoints)
		result := h.apiContractAnalyze(filter)
		responseData := map[string]any{
			"status":                   "ok",
			"operation":                "analyze",
			"action":                   result.Action,
			"analyzed_at":              result.AnalyzedAt,
			"summary":                  result.Summary,
			"violations":               result.Violations,
			"tracked_endpoints":        result.TrackedEndpoints,
			"total_requests_analyzed":  result.TotalRequestsAnalyzed,
			"clean_endpoints":          result.CleanEndpoints,
			"possible_violation_types": result.PossibleViolationTypes,
		}
		if result.DataWindowStartedAt != "" {
			responseData["data_window_started_at"] = result.DataWindowStartedAt
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		if result.Hint != "" {
			responseData["hint"] = result.Hint
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation analyze", responseData)}

	case "report":
		h.processAPIValidationBodies()
		filter := h.apiValidationFilter(params.URLFilter, params.IgnoreEndpoints)
		result := h.apiContractReport(filter)
		responseData := map[string]any{
			"status":             "ok",
			"operation":          "report",
			"action":             result.Action,
			"analyzed_at":        result.AnalyzedAt,
			"endpoints":          result.Endpoints,
			"consistency_levels": result.ConsistencyLevels,
		}
		if result.AppliedFilter != nil {
			responseData["applied_filter"] = result.AppliedFilter
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation report", responseData)}

	case "clear":
		h.clearAPIValidationState()
		clearResult := map[string]any{
			"action":    "cleared",
			"status":    "ok",
			"operation": "clear",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation cleared", clearResult)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))}
	}
}

func (h *ToolHandler) processAPIValidationBodies() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	if h.apiContractValidator == nil {
		h.apiContractValidator = analysis.NewAPIContractValidator()
	}

	bodies := h.capture.GetNetworkBodies()
	if h.apiContractOffset < 0 || h.apiContractOffset > len(bodies) {
		// Buffer can shrink due ring eviction; clamp to avoid replaying old data.
		h.apiContractOffset = len(bodies)
	}

	for _, body := range bodies[h.apiContractOffset:] {
		h.apiContractValidator.Validate(body)
	}
	h.apiContractOffset = len(bodies)
}

func (h *ToolHandler) apiValidationFilter(urlFilter string, ignore []string) analysis.APIContractFilter {
	return analysis.APIContractFilter{
		URLFilter:       urlFilter,
		IgnoreEndpoints: ignore,
	}
}

func (h *ToolHandler) apiContractAnalyze(filter analysis.APIContractFilter) analysis.APIContractAnalyzeResult {
	h.apiContractMu.Lock()
	validator := h.apiContractValidator
	h.apiContractMu.Unlock()
	if validator == nil {
		return analysis.APIContractAnalyzeResult{}
	}
	return validator.Analyze(filter)
}

func (h *ToolHandler) apiContractReport(filter analysis.APIContractFilter) analysis.APIContractReportResult {
	h.apiContractMu.Lock()
	validator := h.apiContractValidator
	h.apiContractMu.Unlock()
	if validator == nil {
		return analysis.APIContractReportResult{}
	}
	return validator.Report(filter)
}

func (h *ToolHandler) clearAPIValidationState() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	h.apiContractValidator = analysis.NewAPIContractValidator()
	h.apiContractOffset = len(h.capture.GetNetworkBodies())
}

// ============================================
// Link Health
// ============================================

// toolAnalyzeLinkHealth checks all links on the current page for health issues.
func (h *ToolHandler) toolAnalyzeLinkHealth(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Generate correlation ID for tracking
	correlationID := newCorrelationID("link_health")

	// Create pending query for link health check
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Link health check initiated")
}

// ============================================
// Link Validation (Server-Side)
// ============================================

// toolValidateLinks verifies CORS-blocked URLs using server-side HTTP requests.
// This provides a fallback for links that the browser couldn't check due to CORS.
func (h *ToolHandler) toolValidateLinks(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params az.LinkValidationParams
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if len(params.URLs) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'urls' is missing or empty", "Provide an array of URLs to validate")}
	}

	timeoutMS := az.ClampInt(params.TimeoutMS, 15000, 1000, 60000)
	maxWorkers := az.ClampInt(params.MaxWorkers, 20, 1, 100)

	validURLs := az.FilterHTTPURLs(params.URLs)
	if len(validURLs) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "No valid HTTP/HTTPS URLs provided", "URLs must start with http:// or https://", withParam("urls"))}
	}
	if len(validURLs) > az.MaxLinkValidationURLs {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			fmt.Sprintf("Too many URLs: got %d, max %d", len(validURLs), az.MaxLinkValidationURLs),
			fmt.Sprintf("Reduce URLs to %d or fewer and retry", az.MaxLinkValidationURLs),
			withParam("urls"),
		)}
	}

	results := az.ValidateLinksServerSide(validURLs, timeoutMS, maxWorkers, version)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Server-side link validation completed", map[string]any{
		"status":  "completed",
		"total":   len(results),
		"results": results,
	})}
}
