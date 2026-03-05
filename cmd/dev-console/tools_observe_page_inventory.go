// Purpose: Handles observe(what="page_inventory") — returns combined page info and interactive elements in a single extension query.
// Why: Reduces two separate observe calls (page + list_interactive) into one for faster page discovery.
// Docs: docs/features/feature/mcp-persistent-server/index.md

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
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	correlationID := newCorrelationID("page_inventory")

	query := queries.PendingQuery{
		Type:          "page_inventory",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return h.MaybeWaitForCommand(req, correlationID, args, "Page inventory queued")
}
