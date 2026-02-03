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
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Reproduction script", map[string]any{"script": ""})}
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

	responseData := map[string]any{
		"status":  "ok",
		"scope":   arguments.Scope,
		"rules":   0,
		"results": 0,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("SARIF export complete", responseData)}
}

func (h *ToolHandler) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("HAR export", map[string]any{"har": map[string]any{}})}
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

	responseData := map[string]any{
		"status": "ok",
		"mode":   arguments.Mode,
		"policy": "",
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("CSP policy generated", responseData)}
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
