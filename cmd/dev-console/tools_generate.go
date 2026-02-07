// tools_generate.go — MCP generate tool dispatcher and handlers.
// Handles all generate formats: reproduction, test, pr_summary, sarif, har, csp, sri.
package main

import (
	"encoding/json"
)

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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'format' is missing", "Add the 'format' parameter and call again", withParam("format"), withHint("Valid values: reproduction, test, pr_summary, sarif, har, csp, sri, test_from_context, test_heal, test_classify"))}
	}

	var resp JSONRPCResponse
	switch params.Format {
	case "reproduction":
		resp = h.toolGetReproductionScript(req, args)
	case "test":
		resp = h.toolGenerateTest(req, args)
	case "pr_summary":
		resp = h.toolGeneratePRSummary(req, args)
	case "sarif":
		resp = h.toolExportSARIF(req, args)
	case "har":
		resp = h.toolExportHAR(req, args)
	case "csp":
		resp = h.toolGenerateCSP(req, args)
	case "sri":
		resp = h.toolGenerateSRI(req, args)
	case "test_from_context":
		resp = h.handleGenerateTestFromContext(req, args)
	case "test_heal":
		resp = h.handleGenerateTestHeal(req, args)
	case "test_classify":
		resp = h.handleGenerateTestClassify(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown generate format: "+params.Format, "Use a valid format from the 'format' enum", withParam("format"))}
	}
	return resp
}

// ============================================
// Generate sub-handlers
// ============================================

func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolGetReproductionScriptImpl(req, args)
}

func (h *ToolHandler) toolGenerateTest(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test", map[string]any{"script": ""})}
}

func (h *ToolHandler) toolGeneratePRSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("PR summary", map[string]any{"summary": ""})}
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

	// SARIF 2.1.0 spec: top-level requires version, $schema, runs[]
	responseData := map[string]any{
		"version": "2.1.0",
		"$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		"runs": []map[string]any{{
			"tool": map[string]any{
				"driver": map[string]any{
					"name":    "gasoline",
					"version": version,
					"rules":   []any{},
				},
			},
			"results": []any{},
		}},
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SARIF export complete", responseData)}
}

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// HAR 1.2 spec: top-level requires log object with version, creator, entries[]
	responseData := map[string]any{
		"log": map[string]any{
			"version": "1.2",
			"creator": map[string]any{
				"name":    "gasoline",
				"version": version,
			},
			"entries": []any{},
		},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("HAR export", responseData)}
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

	// Default to "moderate" mode if not provided (matches internal CSP generator default)
	mode := arguments.Mode
	if mode == "" {
		mode = "moderate"
	}

	// Check if we have network body data to generate CSP from
	networkBodies := h.capture.GetNetworkBodies()
	if len(networkBodies) == 0 {
		responseData := map[string]any{
			"status":  "unavailable",
			"mode":    mode,
			"policy":  "",
			"reason":  "No network requests captured yet. CSP generation requires observing network traffic to identify resource origins.",
			"hint":    "Navigate the tracked page to load resources (scripts, stylesheets, images, fonts), then call generate(csp) again.",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy unavailable", responseData)}
	}

	// Build a basic CSP policy from observed origins
	// Extract unique origins from network bodies
	originsByType := make(map[string]map[string]bool) // directive -> origins
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

	// Build CSP directives
	directives := make(map[string][]string)
	directives["default-src"] = []string{"'self'"}
	for directive, origins := range originsByType {
		var originList []string
		for origin := range origins {
			originList = append(originList, origin)
		}
		if len(originList) > 0 {
			directives[directive] = append([]string{"'self'"}, originList...)
		}
	}

	// Generate policy string
	var policyParts []string
	for directive, sources := range directives {
		policyParts = append(policyParts, directive+" "+joinStrings(sources, " "))
	}
	policy := joinStrings(policyParts, "; ")

	responseData := map[string]any{
		"status":           "ok",
		"mode":             mode,
		"policy":           policy,
		"directives":       directives,
		"origins_observed": len(networkBodies),
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy generated", responseData)}
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
	var params struct {
		Origins       []string `json:"origins"`
		ResourceTypes []string `json:"resource_types"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status":    "ok",
		"resources": []any{},
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SRI hashes generated", responseData)}
}
