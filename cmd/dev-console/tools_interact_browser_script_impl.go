// Purpose: Implements highlight and execute_js browser actions.
// Why: Isolate script/highlight flows from navigation and tab lifecycle handlers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
	act "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/interact"
)

// validWorldValues delegates to the interact package.
var validWorldValues = act.ValidWorldValues

// truncateToLen delegates to the interact package.
var truncateToLen = act.TruncateToLen

func (h *interactActionHandler) handleHighlightImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector   string `json:"selector"`
		DurationMs int    `json:"duration_ms,omitempty"`
		TabID      int    `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if params.Selector == "" {
		return fail(req, ErrMissingParam, "Required parameter 'selector' is missing", "Add the 'selector' parameter", withParam("selector"))
	}

	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}

	// Queue highlight command for extension.
	correlationID := newCorrelationID("highlight")
	h.armEvidenceForCommand(correlationID, "highlight", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "highlight",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	// Record AI action.
	h.parent.recordAIAction("highlight", "", map[string]any{"selector": params.Selector})

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Highlight queued")
}

func (h *interactActionHandler) handleExecuteJSImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Script    string `json:"script"`
		TimeoutMs int    `json:"timeout_ms,omitempty"`
		TabID     int    `json:"tab_id,omitempty"`
		World     string `json:"world,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if params.Script == "" {
		return fail(req, ErrMissingParam, "Required parameter 'script' is missing", "Add the 'script' parameter and call again", withParam("script"))
	}

	if params.World == "" {
		params.World = "auto"
	}
	if !validWorldValues[params.World] {
		return fail(req, ErrInvalidParam, "Invalid 'world' value: "+params.World, "Use 'auto' (default, tries main then isolated), 'main' (page JS access), or 'isolated' (bypasses CSP, DOM only)", withParam("world"))
	}

	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireCSPClear(req, params.World); blocked {
		return resp
	}

	correlationID := newCorrelationID("exec")
	h.armEvidenceForCommand(correlationID, "execute_js", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "execute",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction("execute_js", "", map[string]any{"script_preview": truncateToLen(params.Script, 100)})

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Command queued")
}
