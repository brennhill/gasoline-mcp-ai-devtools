// Purpose: Implements navigation/history browser actions.
// Why: Keep navigation flow logic separate from wrapper entrypoints and utility helpers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

func (h *interactActionHandler) handleBrowserActionNavigateImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		TabID          int    `json:"tab_id,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := requireString(req, params.URL, "url", "Add the 'url' parameter and call again"); blocked {
		return resp
	}
	resolvedURL, err := h.resolveNavigateURLImpl(params.URL)
	if err != nil {
		return fail(req, ErrInvalidParam,
			err.Error(),
			"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
			withParam("url"))
	}

	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}

	correlationID := newCorrelationID("nav")
	h.armEvidenceForCommand(correlationID, "navigate", args, req.ClientID)

	h.stashPerfSnapshotImpl(correlationID)

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "navigate"
	// Ensure required URL is present even if caller used alias forms.
	actionParams["url"] = resolvedURL
	actionPayload, _ := json.Marshal(actionParams)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        actionPayload,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction("navigate", resolvedURL, map[string]any{
		"target_url":    resolvedURL,
		"requested_url": params.URL,
	})

	resp := h.parent.MaybeWaitForCommand(req, correlationID, args, "Navigate queued")

	// If include_content is requested and navigate succeeded, enrich with page content.
	if params.IncludeContent {
		resp = h.parent.enrichNavigateResponse(resp, req, params.TabID)
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	resp = h.parent.injectCSPBlockedActions(resp)

	return resp
}

func (h *interactActionHandler) handleBrowserActionRefreshImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("refresh")
	h.armEvidenceForCommand(correlationID, "refresh", args, req.ClientID)

	h.stashPerfSnapshotImpl(correlationID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"refresh"}`),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction("refresh", "", nil)

	return h.parent.MaybeWaitForCommand(req, correlationID, args, "Refresh queued")
}

func (h *interactActionHandler) handleBrowserActionBackImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "back",
		correlationPfx: "back",
		queuedMsg:      "Back queued",
	})
}

func (h *interactActionHandler) handleBrowserActionForwardImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.queueBrowserAction(req, args, browserActionOpts{
		action:         "forward",
		correlationPfx: "forward",
		queuedMsg:      "Forward queued",
	})
}
