// Purpose: Handles coordinate-driven click flows that route through CDP actions.
// Why: Separates hardware-click behavior from generic DOM selector-based primitives.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"encoding/json"
)

// handleHardwareClick dispatches a coordinate-based click via CDP Input.dispatchMouseEvent.
// This gives LLMs an explicit "I see coordinates in a screenshot, click there" path.
func (h *InteractActionHandler) HandleHardwareClick(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	params, err := parseHardwareClickParams(args)
	if err != nil {
		return mcp.Fail(req, mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	if params.X == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'x' is missing", "Add the 'x' coordinate (pixels from left)", mcp.WithParam("x"))
	}
	if params.Y == nil {
		return mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'y' is missing", "Add the 'y' coordinate (pixels from top)", mcp.WithParam("y"))
	}

	return h.HandleCDPClick(req, args, "hardware_click", *params.X, *params.Y, params.TabID)
}

// handleCDPClick creates a cdp_action query for a hardware-level click at coordinates.
func (h *InteractActionHandler) HandleCDPClick(req mcp.JSONRPCRequest, args json.RawMessage, action string, x, y float64, tabID int) mcp.JSONRPCResponse {
	return h.newCommand("cdp_click").
		correlationPrefix("cdp_click").
		reason(action).
		queryType("cdp_action").
		buildParams(map[string]any{
			"action": "click",
			"x":      x,
			"y":      y,
		}).
		tabID(tabID).
		guardsWithOpts(
			[]func(*mcp.StructuredError){mcp.WithAction(action)},
			h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking,
		).
		recordAction(action, "", map[string]any{"x": x, "y": y, "method": "cdp"}).
		queuedMessage(action + " queued").
		execute(req, args)
}
