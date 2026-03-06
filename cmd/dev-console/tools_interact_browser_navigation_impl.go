// Purpose: Implements navigation/history browser actions.
// Why: Keep navigation flow logic separate from wrapper entrypoints and utility helpers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
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

	actionParams := make(map[string]any)
	lenientUnmarshal(args, &actionParams)
	actionParams["action"] = "navigate"
	// Ensure required URL is present even if caller used alias forms.
	actionParams["url"] = resolvedURL
	actionPayload := buildQueryParams(actionParams)

	resp := h.newCommand("navigate").
		correlationPrefix("nav").
		reason("navigate").
		queryType("browser_action").
		queryParams(actionPayload).
		tabID(params.TabID).
		guards(h.parent.requirePilot, h.parent.requireExtension).
		preEnqueue(h.stashPerfSnapshotImpl).
		recordAction("navigate", resolvedURL, map[string]any{
			"target_url":    resolvedURL,
			"requested_url": params.URL,
		}).
		queuedMessage("Navigate queued").
		execute(req, args)

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

	return h.newCommand("refresh").
		correlationPrefix("refresh").
		reason("refresh").
		queryType("browser_action").
		buildParams(map[string]any{"action": "refresh"}).
		tabID(params.TabID).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		preEnqueue(h.stashPerfSnapshotImpl).
		recordAction("refresh", "", nil).
		queuedMessage("Refresh queued").
		execute(req, args)
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
