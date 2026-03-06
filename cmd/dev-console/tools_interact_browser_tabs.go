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
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}
	resolvedURL := params.URL
	if params.URL != "" {
		rewriteURL, err := h.resolveNavigateURLImpl(params.URL)
		if err != nil {
			return fail(req, ErrInvalidParam,
				err.Error(),
				"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
				withParam("url"))
		}
		resolvedURL = rewriteURL
	}

	correlationID := newCorrelationID("newtab")
	h.armEvidenceForCommand(correlationID, "new_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "new_tab"
	if resolvedURL != "" {
		actionParams["url"] = resolvedURL
	}
	actionPayload := buildQueryParams(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

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
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}
	if params.TabID <= 0 && params.TabIndex == nil {
		return fail(req, ErrMissingParam,
			"switch_tab requires tab_id or tab_index",
			"Provide tab_id from observe(what='tabs') or tab_index from your tab list ordering.",
			withParam("tab_id"),
			withHint("Alternative: provide tab_index"))
	}
	if params.TabIndex != nil && *params.TabIndex < 0 {
		return fail(req, ErrInvalidParam,
			"tab_index must be >= 0",
			"Provide a non-negative tab_index (0-based).",
			withParam("tab_index"))
	}

	// Default set_tracked to true so subsequent commands target the new tab.
	setTracked := params.SetTracked == nil || *params.SetTracked

	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}
	// No requireTabTracking gate: switch_tab IS how you establish tracking
	// for an existing tab. The handler calls applySwitchTabTracking on success.

	correlationID := newCorrelationID("switchtab")
	h.armEvidenceForCommand(correlationID, "switch_tab", args, req.ClientID)

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "switch_tab"
	actionPayload := buildQueryParams(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

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
		h.applySwitchTabTracking(correlationID)
	}

	return resp
}

func (h *interactActionHandler) handleActivateTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "activate_tab",
		correlationPfx: "activate",
		queuedMsg:      "Activate tab queued",
	})
}

func (h *interactActionHandler) handleBrowserActionCloseTabImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "close_tab"
	actionPayload := buildQueryParams(actionParams)

	// NOTE: close_tab is gated even with explicit tab_id.
	// Future: allow bypass when tab_id is explicitly provided.
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "close_tab",
		correlationPfx: "closetab",
		params:         actionPayload,
		tabID:          params.TabID,
		queuedMsg:      "Close tab queued",
		recordExtra:    map[string]any{"tab_id": params.TabID},
	})
}
