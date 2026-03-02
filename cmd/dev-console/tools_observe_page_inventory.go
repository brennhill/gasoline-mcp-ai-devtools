// Purpose: Handles observe(what="page_inventory") — returns combined page info and interactive elements in a single extension query.
// Why: Reduces two separate observe calls (page + list_interactive) into one for faster page discovery.
// Docs: docs/features/feature/observe/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// toolObservePageInventory handles observe(what="page_inventory").
// Creates a pending query for the extension to return combined page info
// and interactive elements in one response.
func (h *ToolHandler) toolObservePageInventory(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID       int  `json:"tab_id"`
		VisibleOnly bool `json:"visible_only"`
		Limit       int  `json:"limit"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	correlationID := newCorrelationID("page_inventory")

	query := queries.PendingQuery{
		Type:          "page_inventory",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	return h.MaybeWaitForCommand(req, correlationID, args, "Page inventory queued")
}
