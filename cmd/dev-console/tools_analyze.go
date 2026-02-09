// tools_analyze.go — MCP analyze tool dispatcher and handlers.
// Handles active analysis operations: dom queries, API validation, link health checks,
// performance analysis, accessibility audits, security checks, error analysis, and history analysis.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// AnalyzeHandler is the function signature for analyze tool handlers.
type AnalyzeHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// analyzeHandlers maps analyze mode names to their handler functions.
var analyzeHandlers = map[string]AnalyzeHandler{
	// Moved from configure
	"dom":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolQueryDOM(req, args) },
	"api_validation":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolValidateAPI(req, args) },

	// Moved from observe
	"performance":      func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolCheckPerformance(req, args) },
	"accessibility":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolRunA11yAudit(req, args) },
	"error_clusters":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeErrors(req) },
	"history":          func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeHistory(req, args) },
	"security_audit":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolSecurityAudit(req, args) },
	"third_party_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAuditThirdParties(req, args) },
	"security_diff":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolDiffSecurity(req, args) },

	// New
	"link_health":      func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeLinkHealth(req, args) },
}

// getValidAnalyzeModes returns a sorted, comma-separated list of valid analyze modes.
func getValidAnalyzeModes() string {
	modes := make([]string, 0, len(analyzeHandlers))
	for mode := range analyzeHandlers {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}

// toolAnalyze dispatches analyze requests based on the 'what' parameter.
func (h *ToolHandler) toolAnalyze(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.What == "" {
		validModes := getValidAnalyzeModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validModes))}
	}

	handler, ok := analyzeHandlers[params.What]
	if !ok {
		validModes := getValidAnalyzeModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown analyze mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes))}
	}

	return handler(h, req, args)
}

// ============================================
// Link Health (Placeholder)
// ============================================

// toolAnalyzeLinkHealth checks all links on the current page for health issues.
// This is a placeholder that will be fully implemented in Phase 1.
func (h *ToolHandler) toolAnalyzeLinkHealth(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Generate correlation ID for tracking
	correlationID := fmt.Sprintf("link_health_%d_%d", time.Now().UnixNano(), randomInt63())

	// Create pending query for link health check
	query := queries.PendingQuery{
		Type:          "link_health",
		Params:        args,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Link health check initiated", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"hint":           "Use observe({what:'command_result', correlation_id:'" + correlationID + "'}) to check the result",
	})}
}
