// Purpose: Handles explore_page — a single compound query returning screenshot, interactive elements, metadata, text, and links.
// Why: Reduces agent round-trips by combining page discovery signals into one call instead of multiple observe/interact calls.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"encoding/json"
	"net/url"
)

// handleExplorePage handles interact(what="explore_page").
// Creates a pending query for the extension to return combined page metadata,
// interactive elements, readable text, and navigation links in one response.
// If url is provided, the extension navigates first before collecting data.
// Screenshot is appended server-side after the extension returns.
func (h *interactActionHandler) handleExplorePage(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

	resp := h.newCommand("explore_page").
		correlationPrefix("explore_page").
		reason("explore_page").
		queryType("explore_page").
		queryParams(args).
		tabID(params.TabID).
		guards(h.parent.requirePilot, h.parent.requireExtension, h.parent.requireTabTracking).
		recordAction("explore_page", params.URL, nil).
		queuedMessage("Explore page queued").
		execute(req, args)

	// Append inline screenshot only if the command completed (not queued or error)
	if !isErrorResponse(resp) && !isResponseQueued(resp) {
		resp = h.appendScreenshotToResponse(resp, req)
	}

	return resp
}
