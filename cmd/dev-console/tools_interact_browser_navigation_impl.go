// Purpose: Implements navigation/history browser actions.
// Why: Keep navigation flow logic separate from wrapper entrypoints and utility helpers.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/queries"
)

func (h *ToolHandler) handleBrowserActionNavigateImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL            string `json:"url"`
		TabID          int    `json:"tab_id,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.URL == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'url' is missing", "Add the 'url' parameter and call again", withParam("url"))}
	}
	resolvedURL, err := h.resolveNavigateURL(params.URL)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			err.Error(),
			"Enable configure(what='security_mode', mode='insecure_proxy', confirm=true), or use a standard http(s) URL.",
			withParam("url"),
		)}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("nav")
	h.armEvidenceForCommand(correlationID, "navigate", args, req.ClientID)

	h.stashPerfSnapshot(correlationID)

	actionParams := make(map[string]any)
	_ = json.Unmarshal(args, &actionParams)
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
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("navigate", resolvedURL, map[string]any{
		"target_url":    resolvedURL,
		"requested_url": params.URL,
	})

	resp := h.MaybeWaitForCommand(req, correlationID, args, "Navigate queued")

	// If include_content is requested and navigate succeeded, enrich with page content.
	if params.IncludeContent {
		resp = h.enrichNavigateResponse(resp, req, params.TabID)
	}

	// Include blocked_actions/blocked_reason when CSP restricts — omit entirely
	// when CSP is clear to avoid wasting tokens on normal pages. (#262)
	resp = h.injectCSPBlockedActions(resp)

	return resp
}

func (h *ToolHandler) handleBrowserActionRefreshImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("refresh")
	h.armEvidenceForCommand(correlationID, "refresh", args, req.ClientID)

	h.stashPerfSnapshot(correlationID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"refresh"}`),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("refresh", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Refresh queued")
}

func (h *ToolHandler) handleBrowserActionBackImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("back")
	h.armEvidenceForCommand(correlationID, "back", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"back"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("back", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Back queued")
}

func (h *ToolHandler) handleBrowserActionForwardImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	correlationID := newCorrelationID("forward")
	h.armEvidenceForCommand(correlationID, "forward", args, req.ClientID)

	query := queries.PendingQuery{
		Type:          "browser_action",
		Params:        json.RawMessage(`{"action":"forward"}`),
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("forward", "", nil)

	return h.MaybeWaitForCommand(req, correlationID, args, "Forward queued")
}
