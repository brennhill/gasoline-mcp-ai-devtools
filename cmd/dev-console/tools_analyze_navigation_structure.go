// Purpose: Implements analyze modes for navigation discovery and page structure analysis.
// Why: Keeps queued DOM-structure analysis handlers separate from inspect and visual flows.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/queries"
)

// ============================================
// Navigation / SPA Route Discovery (#335)
// ============================================

func (h *ToolHandler) toolAnalyzeNavigation(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	correlationID := newCorrelationID("navigation")
	query := queries.PendingQuery{
		Type:          "navigation",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Navigation discovery queued")
}

// ============================================
// Page Structure Analysis (#341)
// ============================================

func (h *ToolHandler) toolAnalyzePageStructure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	correlationID := newCorrelationID("page_structure")
	query := queries.PendingQuery{
		Type:          "page_structure",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Page structure analysis queued")
}
