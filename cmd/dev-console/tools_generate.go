// Purpose: Implements generate tool formats and output assembly.
// Docs: docs/features/feature/test-generation/index.md

// tools_generate.go — MCP generate tool dispatcher and handlers.
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
)

// GenerateHandler is the function signature for generate format handlers.
type GenerateHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

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

// toolGenerate dispatches generate requests based on the 'format' parameter.
func (h *ToolHandler) toolGenerate(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Format string `json:"format"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Format == "" {
		validFormats := getValidGenerateFormats()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'format' is missing", "Add the 'format' parameter and call again", withParam("format"), withHint("Valid values: "+validFormats))}
	}

	handler, ok := generateHandlers[params.Format]
	if !ok {
		validFormats := getValidGenerateFormats()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown generate format: "+params.Format, "Use a valid format from the 'format' enum", withParam("format"), withHint("Valid values: "+validFormats))}
	}

	return handler(h, req, args)
}

// ============================================
// Generate sub-handlers
// ============================================

func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolGetReproductionScriptImpl(req, args)
}

// TestGenParams are the parsed arguments for generate({format: "test"}).
type TestGenParams struct {
	Format              string `json:"format"`
	TestName            string `json:"test_name"`
	LastN               int    `json:"last_n"`
	BaseURL             string `json:"base_url"`
	AssertNetwork       bool   `json:"assert_network"`
	AssertNoErrors      bool   `json:"assert_no_errors"`
	AssertResponseShape bool   `json:"assert_response_shape"`
}

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestGenParams
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.TestName == "" {
		params.TestName = "generated test"
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := filterLastN(allActions, params.LastN)

	script := generateTestScript(actions, params)

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

	summary := fmt.Sprintf("Playwright test '%s' (%d actions)", params.TestName, len(actions))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}

// generateTestScript builds a complete Playwright test file from captured actions.
func generateTestScript(actions []capture.EnhancedAction, params TestGenParams) string {
	var b strings.Builder

	b.WriteString("import { test, expect } from '@playwright/test';\n\n")
	fmt.Fprintf(&b, "test.describe('%s', () => {\n", escapeJS(params.TestName))

	if len(actions) == 0 {
		b.WriteString("  test('should load page', async ({ page }) => {\n")
		b.WriteString("    // No actions captured — add test steps here\n")
		b.WriteString("    await page.goto('/');\n")
		b.WriteString("    await expect(page).toHaveTitle(/.+/);\n")
		b.WriteString("  });\n")
	} else {
		writeTestSteps(&b, actions, params)
	}

	b.WriteString("});\n")
	return b.String()
}

// writeTestSteps groups actions into logical test blocks and writes them.
func writeTestSteps(b *strings.Builder, actions []capture.EnhancedAction, params TestGenParams) {
	// Group actions by navigation — each page gets its own test() block.
	groups := groupActionsByNavigation(actions)

	for i, group := range groups {
		testLabel := testLabelForGroup(group, i)
		fmt.Fprintf(b, "  test('%s', async ({ page }) => {\n", escapeJS(testLabel))

		opts := ReproductionParams{BaseURL: params.BaseURL}
		var prevTs int64
		for _, action := range group {
			writePauseComment(b, prevTs, action.Timestamp, "    // [%ds pause]\n")
			prevTs = action.Timestamp
			line := playwrightStep(action, opts)
			if line != "" {
				b.WriteString("    " + line + "\n")
			}
		}

		// Add assertions at the end of each test block.
		writeTestAssertions(b, group, params)

		b.WriteString("  });\n\n")
	}
}

// groupActionsByNavigation splits actions into groups at each navigate action.
// Each group starts with a navigate (if present) and includes all subsequent
// actions until the next navigate.
func groupActionsByNavigation(actions []capture.EnhancedAction) [][]capture.EnhancedAction {
	if len(actions) == 0 {
		return nil
	}
	var groups [][]capture.EnhancedAction
	var current []capture.EnhancedAction

	for _, action := range actions {
		if action.Type == "navigate" && len(current) > 0 {
			groups = append(groups, current)
			current = nil
		}
		current = append(current, action)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

// testLabelForGroup generates a descriptive test label for a group of actions.
func testLabelForGroup(group []capture.EnhancedAction, index int) string {
	if len(group) == 0 {
		return fmt.Sprintf("step %d", index+1)
	}
	first := group[0]
	if first.Type == "navigate" && first.ToURL != "" {
		// Extract path from URL for a readable name.
		path := first.ToURL
		if idx := strings.Index(path, "://"); idx >= 0 {
			path = path[idx+3:]
		}
		if idx := strings.Index(path, "/"); idx >= 0 {
			path = path[idx:]
		}
		if path == "/" || path == "" {
			path = "homepage"
		}
		return fmt.Sprintf("should work on %s", chopString(path, 60))
	}
	return fmt.Sprintf("step %d", index+1)
}

// writeTestAssertions adds expect() assertions at the end of a test block.
func writeTestAssertions(b *strings.Builder, group []capture.EnhancedAction, params TestGenParams) {
	hasNavigate := false
	for _, a := range group {
		if a.Type == "navigate" {
			hasNavigate = true
			break
		}
	}

	if hasNavigate {
		b.WriteString("    // Verify page loaded successfully\n")
		b.WriteString("    await expect(page).toHaveTitle(/.+/);\n")
	}

	if params.AssertNoErrors {
		b.WriteString("    // Assert no console errors\n")
		b.WriteString("    const errors = [];\n")
		b.WriteString("    page.on('console', msg => { if (msg.type() === 'error') errors.push(msg.text()); });\n")
		b.WriteString("    expect(errors).toHaveLength(0);\n")
	}

	if params.AssertNetwork {
		b.WriteString("    // Assert no failed network requests\n")
		b.WriteString("    const failedRequests = [];\n")
		b.WriteString("    page.on('requestfailed', req => failedRequests.push(req.url()));\n")
		b.WriteString("    expect(failedRequests).toHaveLength(0);\n")
	}
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
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Run a11y audit to get violations — fall back to empty if no extension connected
	var a11yResult json.RawMessage
	if h.capture.IsExtensionConnected() {
		var err error
		a11yResult, err = h.executeA11yQuery(arguments.Scope, nil, nil, false)
		if err != nil {
			a11yResult = json.RawMessage("{}")
		}
	} else {
		a11yResult = json.RawMessage("{}")
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

	directives := buildCSPDirectives(networkBodies)
	policy := buildCSPPolicyString(directives)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy generated", map[string]any{
		"status": "ok", "mode": mode, "policy": policy,
		"directives": directives, "origins_observed": len(networkBodies),
	})}
}

// buildCSPDirectives extracts unique origins from network bodies and groups them by CSP directive.
func buildCSPDirectives(networkBodies []capture.NetworkBody) map[string][]string {
	originsByType := make(map[string]map[string]bool)
	for _, body := range networkBodies {
		origin := extractOrigin(body.URL)
		if origin == "" {
			continue
		}
		directive := resourceTypeToCSPDirective(body.ContentType)
		if originsByType[directive] == nil {
			originsByType[directive] = make(map[string]bool)
		}
		originsByType[directive][origin] = true
	}

	directives := map[string][]string{"default-src": {"'self'"}}
	for directive, origins := range originsByType {
		originList := make([]string, 0, len(origins))
		for origin := range origins {
			originList = append(originList, origin)
		}
		if len(originList) > 0 {
			directives[directive] = append([]string{"'self'"}, originList...)
		}
	}
	return directives
}

// buildCSPPolicyString serializes CSP directives into a semicolon-separated policy string.
func buildCSPPolicyString(directives map[string][]string) string {
	var policyParts []string
	for directive, sources := range directives {
		policyParts = append(policyParts, directive+" "+joinStrings(sources, " "))
	}
	return joinStrings(policyParts, "; ")
}

// extractOrigin extracts the origin (scheme://host:port) from a URL
func extractOrigin(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	// Simple extraction - find scheme://host
	idx := 0
	if len(urlStr) > 8 && urlStr[:8] == "https://" {
		idx = 8
	} else if len(urlStr) > 7 && urlStr[:7] == "http://" {
		idx = 7
	} else {
		return ""
	}
	// Find end of host (first / or end of string)
	endIdx := idx
	for endIdx < len(urlStr) && urlStr[endIdx] != '/' && urlStr[endIdx] != '?' {
		endIdx++
	}
	return urlStr[:endIdx]
}

// resourceTypeToCSPDirective maps content-type to CSP directive
func resourceTypeToCSPDirective(contentType string) string {
	switch {
	case containsIgnoreCase(contentType, "javascript"):
		return "script-src"
	case containsIgnoreCase(contentType, "css"):
		return "style-src"
	case containsIgnoreCase(contentType, "font"):
		return "font-src"
	case containsIgnoreCase(contentType, "image"):
		return "img-src"
	case containsIgnoreCase(contentType, "video"), containsIgnoreCase(contentType, "audio"):
		return "media-src"
	default:
		return "connect-src"
	}
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
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
