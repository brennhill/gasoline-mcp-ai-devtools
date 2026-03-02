// Purpose: Implements generate artifact handlers behind stable public wrappers.
// Why: Keep generate dispatch/validation separate from concrete artifact assembly logic.

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

func (h *ToolHandler) generateTestImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

func (h *ToolHandler) generatePRSummaryImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	actions := h.capture.GetAllEnhancedActions()
	completedCmds := h.capture.GetCompletedCommands()
	failedCmds := h.capture.GetFailedCommands()
	logs := h.capture.GetExtensionLogs()
	networkBodies := h.capture.GetNetworkBodies()
	_, _, tabURL := h.capture.GetTrackingStatus()

	// Count actions by type.
	actionCounts := map[string]int{}
	for _, a := range actions {
		actionCounts[a.Type]++
	}

	// Count errors in logs.
	errorCount := 0
	for _, l := range logs {
		if l.Level == "error" {
			errorCount++
		}
	}

	// Count failed network requests.
	networkErrors := 0
	for _, nb := range networkBodies {
		if nb.Status >= 400 {
			networkErrors++
		}
	}

	totalActivity := len(actions) + len(completedCmds) + len(failedCmds) + len(networkBodies)

	// Build markdown summary.
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
	}

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

func (h *ToolHandler) exportSARIFImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

	// Convert to SARIF.
	sarifLog, err := export.ExportSARIF(a11yResult, export.SARIFExportOptions{
		Scope:         arguments.Scope,
		IncludePasses: arguments.IncludePasses,
		SaveTo:        arguments.SaveTo,
	})
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "SARIF export failed: "+err.Error(), "Check a11y audit results and try again.")}
	}

	// Marshal SARIFLog to a generic map for the MCP response.
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

func (h *ToolHandler) exportHARImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
	// Convert to generic map for mcpJSONResponse.
	harJSON, _ := json.Marshal(harLog)
	var harMap map[string]any
	_ = json.Unmarshal(harJSON, &harMap)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, harMap)}
}

func (h *ToolHandler) generateCSPImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

// generateSRIImpl generates Subresource Integrity hashes for third-party scripts/styles.
func (h *ToolHandler) generateSRIImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
