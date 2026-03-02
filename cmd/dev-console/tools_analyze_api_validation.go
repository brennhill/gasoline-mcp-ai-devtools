// Purpose: Implements analyze(api_validation) operations and API-validation state helpers.
// Why: Isolates API contract validation flow from other analyze behaviors.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/analysis"
)

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
	validator := h.apiContractValidatorSnapshot()
	if validator == nil {
		return analysis.APIContractAnalyzeResult{}
	}
	return validator.Analyze(filter)
}

func (h *ToolHandler) apiContractReport(filter analysis.APIContractFilter) analysis.APIContractReportResult {
	validator := h.apiContractValidatorSnapshot()
	if validator == nil {
		return analysis.APIContractReportResult{}
	}
	return validator.Report(filter)
}

func (h *ToolHandler) apiContractValidatorSnapshot() *analysis.APIContractValidator {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()
	return h.apiContractValidator
}

func (h *ToolHandler) clearAPIValidationState() {
	h.apiContractMu.Lock()
	defer h.apiContractMu.Unlock()

	h.apiContractValidator = analysis.NewAPIContractValidator()
	h.apiContractOffset = len(h.capture.GetNetworkBodies())
}
