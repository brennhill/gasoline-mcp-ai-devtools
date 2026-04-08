// Purpose: Implements highlight and execute_js browser actions.
// Why: Isolate script/highlight flows from navigation and tab lifecycle handlers.
// Docs: docs/features/feature/interact-explore/index.md

package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"encoding/json"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// validWorldValues delegates to the interact package.
var validWorldValues = act.ValidWorldValues

// truncateToLen delegates to the interact package.
var truncateToLen = act.TruncateToLen

func (h *InteractActionHandler) HandleHighlightImpl(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector   string `json:"selector"`
		DurationMs int    `json:"duration_ms,omitempty"`
		TabID      int    `json:"tab_id,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := mcp.RequireString(req, params.Selector, "selector", "Add the 'selector' parameter"); blocked {
		return resp
	}

	return h.newCommand("highlight").
		correlationPrefix("highlight").
		reason("highlight").
		queryType("highlight").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		recordAction("highlight", "", map[string]any{"selector": params.Selector}).
		queuedMessage("Highlight queued").
		execute(req, args)
}

func (h *InteractActionHandler) HandleExecuteJSImpl(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Script    string `json:"script"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := mcp.RequireString(req, params.Script, "script", "Add the 'script' parameter and call again"); blocked {
		return resp
	}

	if params.World == "" {
		params.World = "auto"
	}
	if !validWorldValues[params.World] {
		return mcp.Fail(req, mcp.ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'auto' (default, tries main then isolated), 'main' (page JS access), or 'isolated' (bypasses CSP, DOM only)", mcp.WithParam("world"))
	}

	return h.newCommand("execute_js").
		correlationPrefix("exec").
		reason("execute_js").
		queryType("execute").
		queryParams(args).
		tabID(params.TabID).
		guards(h.deps.RequirePilot, h.deps.RequireExtension, h.deps.RequireTabTracking).
		cspGuard(params.World).
		recordAction("execute_js", "", map[string]any{"script_preview": truncateToLen(params.Script, 100)}).
		queuedMessage("Command queued").
		execute(req, args)
}
