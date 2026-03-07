// Purpose: Implements analyze link health and server-side link validation modes.
// Why: Separates asynchronous browser link checks from synchronous server validation logic.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
	az "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/analyze"
)

// toolAnalyzeLinkHealth checks all links on the current page for health issues.
func (h *ToolHandler) toolAnalyzeLinkHealth(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	correlationID := newCorrelationID("link_health")
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return h.MaybeWaitForCommand(req, correlationID, args, "Link health check initiated")
}

// toolValidateLinks verifies CORS-blocked URLs using server-side HTTP requests.
func (h *ToolHandler) toolValidateLinks(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params az.LinkValidationParams
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if len(params.URLs) == 0 {
		return fail(req, ErrMissingParam, "Required parameter 'urls' is missing or empty", "Provide an array of URLs to validate")
	}

	timeoutMS := az.ClampInt(params.TimeoutMS, 15000, 1000, 60000)
	maxWorkers := az.ClampInt(params.MaxWorkers, 20, 1, 100)

	validURLs := az.FilterHTTPURLs(params.URLs)
	if len(validURLs) == 0 {
		return fail(req, ErrInvalidParam, "No valid HTTP/HTTPS URLs provided", "URLs must start with http:// or https://", withParam("urls"))
	}
	if len(validURLs) > az.MaxLinkValidationURLs {
		return fail(req, ErrInvalidParam,
			fmt.Sprintf("Too many URLs: got %d, max %d", len(validURLs), az.MaxLinkValidationURLs),
			fmt.Sprintf("Reduce URLs to %d or fewer and retry", az.MaxLinkValidationURLs),
			withParam("urls"))
	}

	results := az.ValidateLinksServerSide(validURLs, timeoutMS, maxWorkers, version)
	return succeed(req, "Server-side link validation completed", map[string]any{
		"status":  "completed",
		"total":   len(results),
		"results": results,
	})
}
