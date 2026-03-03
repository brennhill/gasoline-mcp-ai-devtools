// Purpose: Handles tab lifecycle actions for interact (new, switch, activate, close).
// Why: Keep tab management isolated from general browser actions for easier maintenance.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

func (h *interactActionHandler) handleBrowserActionNewTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	resolvedURL := params.URL
	if params.URL != "" {
		rewriteURL, err := h.resolveNavigateURLImpl(params.URL)
		if err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				err.Error(),
				"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
				withParam("url"),
			)}
		}
		resolvedURL = rewriteURL
	}

	correlationID := newCorrelationID("newtab")
	h.armEvidenceForCommand(correlationID, "new_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "new_tab"
	if resolvedURL != "" {
		actionParams["url"] = resolvedURL
	}
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		CorrelationID: correlationID,
	}
	h.parent.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.parent.recordAIAction("new_tab", resolvedURL, map[string]any{
		"target_url":    resolvedURL,
		"requested_url": params.URL,
	})

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "New tab queued")
}

func (h *interactActionHandler) handleBrowserActionSwitchTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID      int   `json:"tab_id,omitempty"`
		TabIndex   *int  `json:"tab_index,omitempty"`
		SetTracked *bool `json:"set_tracked,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	if params.TabID <= 0 && params.TabIndex == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrMissingParam,
			"switch_tab requires tab_id or tab_index",
			"Provide tab_id from observe(what='tabs') or tab_index from your tab list ordering.",
			withParam("tab_id"),
			withHint("Alternative: provide tab_index"),
		)}
	}
	if params.TabIndex != nil && *params.TabIndex < 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"tab_index must be >= 0",
			"Provide a non-negative tab_index (0-based).",
			withParam("tab_index"),
		)}
	}

	// Default set_tracked to true so subsequent commands target the new tab.
	setTracked := params.SetTracked == nil || *params.SetTracked

	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	// No requireTabTracking gate: switch_tab IS how you establish tracking
	// for an existing tab. The handler calls applySwitchTabTracking on success.

	correlationID := newCorrelationID("switchtab")
	h.armEvidenceForCommand(correlationID, "switch_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "switch_tab"
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.parent.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.parent.recordAIAction("switch_tab", "", map[string]any{
		"tab_id":    params.TabID,
		"tab_index": params.TabIndex,
	})

	resp := h.parent.MaybeWaitForCommand(req, correlationID, args, "Switch tab queued")

	// After the command completes, update tracked tab state so subsequent
	// commands target the newly activated tab. See issue #271.
	// NOTE: In async mode (sync=false), tracking update is deferred to
	// extension-side persistTrackedTab via the next /sync heartbeat.
	// Server-side update only occurs in sync mode because MaybeWaitForCommand
	// returns immediately when sync=false, so GetCommandResult has no result yet.
	if setTracked {
		h.parent.applySwitchTabTracking(correlationID)
	}

	return resp
}

func (h *interactActionHandler) handleActivateTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("activate")
	h.armEvidenceForCommand(correlationID, "activate_tab", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"activate_tab"}`),
		CorrelationID: correlationID,
	}
	h.parent.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.parent.recordAIAction("activate_tab", "", nil)

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Activate tab queued")
}

func (h *interactActionHandler) handleBrowserActionCloseTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.parent.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireExtension(req); blocked {
		return resp
	}
	// NOTE: close_tab is gated even with explicit tab_id.
	// Future: allow bypass when tab_id is explicitly provided.
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("closetab")
	h.armEvidenceForCommand(correlationID, "close_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
	actionParams["action"] = "close_tab"
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.parent.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.parent.recordAIAction("close_tab", "", map[string]any{
		"tab_id": params.TabID,
	})

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Close tab queued")
}
