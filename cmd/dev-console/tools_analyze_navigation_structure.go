// Purpose: Implements analyze modes for navigation discovery and page structure analysis.
// Why: Keeps queued DOM-structure analysis handlers separate from inspect and visual flows.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// ============================================
// Navigation / SPA Route Discovery (#335)
// ============================================

func (h *ToolHandler) toolAnalyzeNavigation(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id"`
	}
	if len(args) > 0 {
				if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	correlationID := newCorrelationID("navigation")
	query := queries.PendingQuery{
		Type:          "navigation",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

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
				if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	correlationID := newCorrelationID("page_structure")
	query := queries.PendingQuery{
		Type:          "page_structure",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return h.MaybeWaitForCommand(req, correlationID, args, "Page structure analysis queued")
}
