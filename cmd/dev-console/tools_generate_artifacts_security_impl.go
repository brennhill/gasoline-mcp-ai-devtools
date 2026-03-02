// Purpose: Implements generate(csp) and generate(sri) artifact assembly.
// Why: Groups security-focused artifact generation paths under one focused module.

package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/security"
	gen "github.com/dev-console/dev-console/internal/tools/generate"
)

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
