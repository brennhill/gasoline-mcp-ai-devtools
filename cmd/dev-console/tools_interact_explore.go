// Purpose: Handles explore_page — a single compound query returning screenshot, interactive elements, metadata, text, and links.
// Why: Reduces agent round-trips by combining page discovery signals into one call instead of multiple observe/interact calls.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"net/url"

	"github.com/dev-console/dev-console/internal/queries"
)

// handleExplorePage handles interact(what="explore_page").
// Creates a pending query for the extension to return combined page metadata,
// interactive elements, readable text, and navigation links in one response.
// If url is provided, the extension navigates first before collecting data.
// Screenshot is appended server-side after the extension returns.
func (h *ToolHandler) handleExplorePage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := h.requireExtension(req); blocked {
		return resp
	}
	if resp, blocked := h.requireTabTracking(req); blocked {
		return resp
	}

	var params struct {
		URL         string `json:"url,omitempty"`
		TabID       int    `json:"tab_id,omitempty"`
		VisibleOnly bool   `json:"visible_only,omitempty"`
		Limit       int    `json:"limit,omitempty"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Validate URL scheme — only http/https allowed (#341 security review)
	if params.URL != "" {
		parsed, err := url.Parse(params.URL)
		if err != nil || parsed.Scheme == "" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid URL: "+params.URL, "Provide a valid http or https URL", withParam("url"))}
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Only http and https URLs are allowed, got: "+parsed.Scheme, "Use an http or https URL", withParam("url"))}
		}
	}

	correlationID := newCorrelationID("explore_page")

	query := queries.PendingQuery{
		Type:          "explore_page",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("explore_page", params.URL, nil)

	resp := h.MaybeWaitForCommand(req, correlationID, args, "Explore page queued")

	// Append inline screenshot only if the command completed (not queued or error)
	if !isErrorResponse(resp) && !isResponseQueued(resp) {
		resp = h.appendScreenshotToResponse(resp, req)
	}

	return resp
}
