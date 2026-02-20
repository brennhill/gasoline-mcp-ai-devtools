// tools_generate.go — MCP generate tool dispatcher and handlers.
// Docs: docs/features/feature/test-generation/index.md
// Handles all generate formats: reproduction, test, pr_summary, sarif, har, csp, sri.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/export"
	"github.com/dev-console/dev-console/internal/security"
	gen "github.com/dev-console/dev-console/internal/tools/generate"
)

// GenerateHandler is the function signature for generate format handlers.
type GenerateHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// generateValidParams defines the allowed parameter names per generate format.
// The "format" and "telemetry_mode" params are always allowed.
var generateValidParams = map[string]map[string]bool{
	"reproduction":      {"error_message": true, "last_n": true, "base_url": true, "include_screenshots": true, "generate_fixtures": true, "visual_assertions": true, "save_to": true},
	"test":              {"test_name": true, "last_n": true, "base_url": true, "assert_network": true, "assert_no_errors": true, "assert_response_shape": true, "save_to": true},
	"pr_summary":        {"save_to": true},
	"har":               {"url": true, "method": true, "status_min": true, "status_max": true, "save_to": true},
	"csp":               {"mode": true, "include_report_uri": true, "exclude_origins": true, "save_to": true},
	"sri":               {"resource_types": true, "origins": true, "save_to": true},
	"sarif":             {"scope": true, "include_passes": true, "save_to": true},
	"visual_test":       {"test_name": true, "annot_session": true, "save_to": true},
	"annotation_report": {"annot_session": true, "save_to": true},
	"annotation_issues": {"annot_session": true, "save_to": true},
	"test_from_context": {"context": true, "error_id": true, "include_mocks": true, "output_format": true, "save_to": true},
	"test_heal":         {"action": true, "test_file": true, "test_dir": true, "broken_selectors": true, "auto_apply": true, "save_to": true},
	"test_classify":     {"action": true, "failure": true, "failures": true, "save_to": true},
}

// alwaysAllowedGenerateParams are params valid for every generate format.
var alwaysAllowedGenerateParams = map[string]bool{
	"what":           true,
	"format":         true,
	"telemetry_mode": true,
}

// ignoredGenerateDispatchWarningParams are accepted at generate-dispatch level
// but not consumed by every sub-handler.
var ignoredGenerateDispatchWarningParams = map[string]bool{
	"what":           true,
	"format":         true,
	"telemetry_mode": true,
	"save_to":        true,
}

func filterGenerateDispatchWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		param, ok := parseUnknownParamWarning(warning)
		if ok && ignoredGenerateDispatchWarningParams[param] {
			continue
		}
		filtered = append(filtered, warning)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func parseUnknownParamWarning(warning string) (string, bool) {
	const prefix = "unknown parameter '"
	const suffix = "' (ignored)"
	if !strings.HasPrefix(warning, prefix) || !strings.HasSuffix(warning, suffix) {
		return "", false
	}
	param := strings.TrimPrefix(warning, prefix)
	param = strings.TrimSuffix(param, suffix)
	if param == "" {
		return "", false
	}
	return param, true
}

// validateGenerateParams checks for unknown parameters and returns an error response if any are found.
func validateGenerateParams(req JSONRPCRequest, format string, args json.RawMessage) *JSONRPCResponse {
	if len(args) == 0 {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return nil // let handler deal with bad JSON
	}
	allowed, ok := generateValidParams[format]
	if !ok {
		return nil // unknown format handled elsewhere
	}
	var unknown []string
	for k := range raw {
		if alwaysAllowedGenerateParams[k] || allowed[k] {
			continue
		}
		unknown = append(unknown, k)
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	validList := make([]string, 0, len(allowed))
	for k := range allowed {
		validList = append(validList, k)
	}
	sort.Strings(validList)
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
		ErrInvalidParam,
		fmt.Sprintf("Unknown parameter(s) for format '%s': %s", format, strings.Join(unknown, ", ")),
		"Remove unknown parameters and call again",
		withParam(unknown[0]),
		withHint(fmt.Sprintf("Valid params for '%s': %s", format, strings.Join(validList, ", "))),
	)}
	return &resp
}

// generateHandlers maps generate format names to their handler functions.
var generateHandlers = map[string]GenerateHandler{
	"reproduction": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetReproductionScript(req, args)
	},
	"test": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateTest(req, args)
	},
	"pr_summary": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGeneratePRSummary(req, args)
	},
	"sarif": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolExportSARIF(req, args)
	},
	"har": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolExportHAR(req, args)
	},
	"csp": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateCSP(req, args)
	},
	"sri": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateSRI(req, args)
	},
	"test_from_context": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.handleGenerateTestFromContext(req, args)
	},
	"test_heal": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.handleGenerateTestHeal(req, args)
	},
	"test_classify": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.handleGenerateTestClassify(req, args)
	},
	"visual_test": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateVisualTest(req, args)
	},
	"annotation_report": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateAnnotationReport(req, args)
	},
	"annotation_issues": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGenerateAnnotationIssues(req, args)
	},
}

// getValidGenerateFormats returns a sorted, comma-separated list of valid generate formats.
func getValidGenerateFormats() string {
	formats := make([]string, 0, len(generateHandlers))
	for f := range generateHandlers {
		formats = append(formats, f)
	}
	sort.Strings(formats)
	return strings.Join(formats, ", ")
}

// toolGenerate dispatches generate requests based on the 'what' parameter.
func (h *ToolHandler) toolGenerate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What   string `json:"what"`
		Format string `json:"format"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	what := params.What
	if what == "" {
		what = params.Format
	}

	if what == "" {
		validFormats := getValidGenerateFormats()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validFormats))}
	}

	handler, ok := generateHandlers[what]
	if !ok {
		validFormats := getValidGenerateFormats()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown generate format: "+what, "Use a valid format from the 'what' enum", withParam("what"), withHint("Valid values: "+validFormats))}
	}

	// Strict parameter validation: reject unknown params for the given format
	if errResp := validateGenerateParams(req, what, args); errResp != nil {
		return *errResp
	}

	return handler(h, req, args)
}

// ============================================
// Generate sub-handlers
// ============================================

func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolGetReproductionScriptImpl(req, args)
}

// TestGenParams delegates to internal/tools/generate.
type TestGenParams = gen.TestGenParams

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params gen.TestGenParams
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.TestName == "" {
		params.TestName = "generated test"
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := gen.FilterLastN(allActions, params.LastN)
	script := gen.GenerateTestScript(actions, params)

	result := map[string]any{
		"script":       script,
		"test_name":    params.TestName,
		"action_count": len(actions),
		"metadata": map[string]any{
			"generated_at":      time.Now().Format(time.RFC3339),
			"actions_available": len(allActions),
			"actions_included":  len(actions),
			"assert_network":    params.AssertNetwork,
			"assert_no_errors":  params.AssertNoErrors,
		},
	}

	if len(actions) == 0 {
		result["reason"] = "no_actions_captured"
		result["hint"] = "Navigate and interact with the browser first, then call generate(test) again."
	}

	summary := fmt.Sprintf("Playwright test '%s' (%d actions)", params.TestName, len(actions))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	actions := h.capture.GetAllEnhancedActions()
	completedCmds := h.capture.GetCompletedCommands()
	failedCmds := h.capture.GetFailedCommands()
	logs := h.capture.GetExtensionLogs()
	networkBodies := h.capture.GetNetworkBodies()
	_, _, tabURL := h.capture.GetTrackingStatus()

	// Count actions by type
	actionCounts := map[string]int{}
	for _, a := range actions {
		actionCounts[a.Type]++
	}

	// Count errors in logs
	errorCount := 0
	for _, l := range logs {
		if l.Level == "error" {
			errorCount++
		}
	}

	// Count failed network requests
	networkErrors := 0
	for _, nb := range networkBodies {
		if nb.Status >= 400 {
			networkErrors++
		}
	}

	totalActivity := len(actions) + len(completedCmds) + len(failedCmds) + len(networkBodies)

	// Build markdown summary
	var sb strings.Builder
	sb.WriteString("## Session Summary\n\n")

	if totalActivity == 0 {
		sb.WriteString("No activity captured during this session.\n\n")
		sb.WriteString("Navigate to a page or interact with the browser to generate activity.\n")
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("PR summary generated", map[string]any{
			"summary": sb.String(),
			"reason":  "no_activity_captured",
			"hint":    "Navigate to a page or interact with the browser first, then call generate(pr_summary) again.",
			"stats": map[string]any{
				"actions": 0, "commands_completed": 0, "commands_failed": 0,
				"console_errors": 0, "network_errors": 0, "network_captured": 0,
			},
		})}
	} else {
		if tabURL != "" {
			sb.WriteString(fmt.Sprintf("- **Page:** %s\n", tabURL))
		}
		sb.WriteString(fmt.Sprintf("- **Actions:** %d total", len(actions)))
		if len(actionCounts) > 0 {
			parts := make([]string, 0, len(actionCounts))
			for t, c := range actionCounts {
				parts = append(parts, fmt.Sprintf("%s: %d", t, c))
			}
			sort.Strings(parts)
			sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(parts, ", ")))
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("- **Commands:** %d completed, %d failed\n", len(completedCmds), len(failedCmds)))
		if errorCount > 0 {
			sb.WriteString(fmt.Sprintf("- **Console Errors:** %d\n", errorCount))
		}
		if networkErrors > 0 {
			sb.WriteString(fmt.Sprintf("- **Network Errors:** %d (HTTP 4xx/5xx)\n", networkErrors))
		}
		sb.WriteString(fmt.Sprintf("- **Network Requests Captured:** %d\n", len(networkBodies)))
	}

	summary := sb.String()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("PR summary generated", map[string]any{
		"summary": summary,
		"stats": map[string]any{
			"actions":            len(actions),
			"commands_completed": len(completedCmds),
			"commands_failed":    len(failedCmds),
			"console_errors":     errorCount,
			"network_errors":     networkErrors,
			"network_captured":   len(networkBodies),
		},
	})}
}

func (h *ToolHandler) toolExportSARIF(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Scope         string `json:"scope"`
		IncludePasses bool   `json:"include_passes"`
		SaveTo        string `json:"save_to"`
		// Internal-use path for workflows that already executed accessibility.
		A11yResult json.RawMessage `json:"a11y_result"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Use precomputed a11y results when available; otherwise run a11y audit.
	a11yResult := arguments.A11yResult
	if len(a11yResult) == 0 {
		if h.capture.IsExtensionConnected() {
			var err error
			a11yResult, err = h.ExecuteA11yQuery(arguments.Scope, nil, nil, false)
			if err != nil {
				a11yResult = json.RawMessage("{}")
			}
		} else {
			a11yResult = json.RawMessage("{}")
		}
	}

	// Convert to SARIF
	sarifLog, err := export.ExportSARIF(a11yResult, export.SARIFExportOptions{
		Scope:         arguments.Scope,
		IncludePasses: arguments.IncludePasses,
		SaveTo:        arguments.SaveTo,
	})
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "SARIF export failed: "+err.Error(), "Check a11y audit results and try again.")}
	}

	// Marshal SARIFLog to a generic map for the MCP response
	sarifJSON, err := json.Marshal(sarifLog)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "SARIF marshal failed: "+err.Error(), "Report this bug.")}
	}
	var sarifMap map[string]any
	if err := json.Unmarshal(sarifJSON, &sarifMap); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "SARIF unmarshal failed: "+err.Error(), "Report this bug.")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SARIF export complete", sarifMap)}
}

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		SaveTo    string `json:"save_to"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	bodies := h.capture.GetNetworkBodies()
	waterfall := h.capture.GetNetworkWaterfallEntries()
	filter := capture.NetworkBodyFilter{
		URLFilter: params.URL,
		Method:    params.Method,
		StatusMin: params.StatusMin,
		StatusMax: params.StatusMax,
	}

	if params.SaveTo != "" {
		result, err := export.ExportHARMergedToFile(bodies, waterfall, filter, version, params.SaveTo)
		if err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrExportFailed, "HAR file export failed: "+err.Error(), "Check the save_to path and try again",
			)}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(
			fmt.Sprintf("HAR exported to %s (%d entries)", result.SavedTo, result.EntriesCount), result,
		)}
	}

	harLog := export.ExportHARMerged(bodies, waterfall, filter, version)
	summary := fmt.Sprintf("HAR export (%d entries)", len(harLog.Log.Entries))
	// Convert to generic map for mcpJSONResponse
	harJSON, _ := json.Marshal(harLog)
	var harMap map[string]any
	_ = json.Unmarshal(harJSON, &harMap)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, harMap)}
}

func (h *ToolHandler) toolGenerateCSP(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Mode string `json:"mode"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	mode := arguments.Mode
	if mode == "" {
		mode = "moderate"
	}
	switch mode {
	case "strict", "moderate", "report_only":
		// valid
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam, "Invalid mode: "+mode, "Use strict, moderate, or report_only",
			withParam("mode"),
		)}
	}

	networkBodies := h.capture.GetNetworkBodies()
	if len(networkBodies) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy unavailable", map[string]any{
			"status": "unavailable", "mode": mode, "policy": "",
			"reason": "No network requests captured yet. CSP generation requires observing network traffic to identify resource origins.",
			"hint":   "Navigate the tracked page to load resources (scripts, stylesheets, images, fonts), then call generate(csp) again.",
		})}
	}

	directives := gen.BuildCSPDirectives(networkBodies)
	policy := gen.BuildCSPPolicyString(directives)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy generated", map[string]any{
		"status": "ok", "mode": mode, "policy": policy,
		"directives": directives, "origins_observed": len(networkBodies),
	})}
}

// toolGenerateSRI generates Subresource Integrity hashes for third-party scripts/styles.
func (h *ToolHandler) toolGenerateSRI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	networkBodies := h.capture.GetNetworkBodies()
	if len(networkBodies) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SRI unavailable", map[string]any{
			"status": "unavailable",
			"hint":   "Navigate pages to capture network traffic first.",
		})}
	}

	_, _, tabURL := h.capture.GetTrackingStatus()
	pageURLs := []string{tabURL}
	result, err := security.HandleGenerateSRI(args, networkBodies, pageURLs)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "SRI generation failed: "+err.Error(), "Fix parameters and call again")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SRI hashes generated", result)}
}
