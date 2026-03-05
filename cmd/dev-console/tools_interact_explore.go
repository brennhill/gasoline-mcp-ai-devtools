// Purpose: Handles explore_page — a single compound query returning screenshot, interactive elements, metadata, text, and links.
// Why: Reduces agent round-trips by combining page discovery signals into one call instead of multiple observe/interact calls.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"net/url"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// handleExplorePage handles interact(what="explore_page").
// Creates a pending query for the extension to return combined page metadata,
// interactive elements, readable text, and navigation links in one response.
// If url is provided, the extension navigates first before collecting data.
// Screenshot is appended server-side after the extension returns.
func (h *interactActionHandler) handleExplorePage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}

	var params struct {
		URL         string `json:"url,omitempty"`
		TabID       int    `json:"tab_id,omitempty"`
		VisibleOnly bool   `json:"visible_only,omitempty"`
		Limit       int    `json:"limit,omitempty"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Validate URL scheme — only http/https allowed (#341 security review)
	if params.URL != "" {
		parsed, err := url.Parse(params.URL)
		if err != nil || parsed.Scheme == "" {
			return fail(req, ErrInvalidParam, "Invalid URL: "+params.URL, "Provide a valid http or https URL", withParam("url"))
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fail(req, ErrInvalidParam, "Only http and https URLs are allowed, got: "+parsed.Scheme, "Use an http or https URL", withParam("url"))
		}
	}

	correlationID := newCorrelationID("explore_page")

	query := queries.PendingQuery{
		Type:          "explore_page",
		Params:        args,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	h.parent.recordAIAction("explore_page", params.URL, nil)

	resp := h.parent.MaybeWaitForCommand(req, correlationID, args, "Explore page queued")

	// Append inline screenshot only if the command completed (not queued or error)
	if !isErrorResponse(resp) && !isResponseQueued(resp) {
		resp = h.appendScreenshotToResponse(resp, req)
	}

	return resp
}
