// Purpose: Handles coordinate-driven click flows that route through CDP actions.
// Why: Separates hardware-click behavior from generic DOM selector-based primitives.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// handleHardwareClick dispatches a coordinate-based click via CDP Input.dispatchMouseEvent.
// This gives LLMs an explicit "I see coordinates in a screenshot, click there" path.
func (h *interactActionHandler) handleHardwareClick(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params, err := parseHardwareClickParams(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	if params.X == nil {
		return fail(req, ErrMissingParam, "Required parameter 'x' is missing", "Add the 'x' coordinate (pixels from left)", withParam("x"))
	}
	if params.Y == nil {
		return fail(req, ErrMissingParam, "Required parameter 'y' is missing", "Add the 'y' coordinate (pixels from top)", withParam("y"))
	}

	return h.handleCDPClick(req, args, "hardware_click", *params.X, *params.Y, params.TabID)
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
func (h *interactActionHandler) handleCDPClick(req JSONRPCRequest, args json.RawMessage, action string, x, y float64, tabID int) JSONRPCResponse {
	if resp, blocked := h.parent.requirePilot(req, withAction(action)); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req, withAction(action)); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req, withAction(action)); blocked {
		return resp
	}

	correlationID := newCorrelationID("cdp_click")
	h.armEvidenceForCommand(correlationID, action, args, req.ClientID)

	cdpParams := buildQueryParams(map[string]any{
		"action": "click",
		"x":      x,
		"y":      y,
	})

	query := queries.PendingQuery{
		Type:          "cdp_action",
		Params:        cdpParams,
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction(action, "", map[string]any{"x": x, "y": y, "method": "cdp"})

	return h.parent.MaybeWaitForCommand(req, correlationID, args, action+" queued")
}
