// Purpose: Implements analyze link health and server-side link validation modes.
// Why: Separates asynchronous browser link checks from synchronous server validation logic.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"fmt"

	"github.com/dev-console/dev-console/internal/queries"
	az "github.com/dev-console/dev-console/internal/tools/analyze"
)

// toolAnalyzeLinkHealth checks all links on the current page for health issues.
func (h *ToolHandler) toolAnalyzeLinkHealth(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	correlationID := newCorrelationID("link_health")
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Link health check initiated")
}

// toolValidateLinks verifies CORS-blocked URLs using server-side HTTP requests.
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
