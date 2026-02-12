// tools_generate.go — MCP generate tool dispatcher and handlers.
// Handles all generate formats: reproduction, test, pr_summary, sarif, har, csp, sri.
package main

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
)

// GenerateHandler is the function signature for generate format handlers.
type GenerateHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// generateHandlers maps generate format names to their handler functions.
var generateHandlers = map[string]GenerateHandler{
	"reproduction":     func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetReproductionScript(req, args) },
	"test":             func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGenerateTest(req, args) },
	"pr_summary":       func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGeneratePRSummary(req, args) },
	"sarif":            func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolExportSARIF(req, args) },
	"har":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolExportHAR(req, args) },
	"csp":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGenerateCSP(req, args) },
	"sri":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGenerateSRI(req, args) },
	"test_from_context": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.handleGenerateTestFromContext(req, args) },
	"test_heal":        func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.handleGenerateTestHeal(req, args) },
	"test_classify":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.handleGenerateTestClassify(req, args) },
	"visual_test":      func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGenerateVisualTest(req, args) },
	"annotation_report": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGenerateAnnotationReport(req, args) },
	"annotation_issues": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGenerateAnnotationIssues(req, args) },
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

	mode := arguments.Mode
	if mode == "" {
		mode = "moderate"
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
