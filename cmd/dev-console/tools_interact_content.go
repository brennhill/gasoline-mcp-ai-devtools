// Purpose: Handles get_readable, get_markdown, and page_summary content extraction via structured extension query types.
// Why: Replaces unsafe IIFE script injection with CSP-safe content-script message-passing for text extraction.
// Docs: docs/features/feature/interact-explore/index.md
// Implements get_readable, get_markdown, and page_summary using dedicated query types
// routed through content script message-passing (CSP-safe, ISOLATED world).
// Issue #257: Moved from "execute" query type with embedded IIFE scripts to
// structured query types that the content script handles directly.
package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

const (
	// navigatePageSummaryWait is the time to wait for the page summary content
	// extraction after navigation. The extension-side query uses a 4s timeout,
	// so this must be slightly longer to allow for round-trip overhead.
	navigatePageSummaryWait = 5 * time.Second
)

// handleContentExtraction is the shared handler for get_readable, get_markdown, and page_summary.
// All three use the same pattern: gate checks, timeout validation, create a pending query with
// the dedicated query type, and wait for the content script to respond.
func (h *interactActionHandler) handleContentExtraction(req JSONRPCRequest, args json.RawMessage, queryType string, correlationPrefix string) JSONRPCResponse {
	var params struct {
		TabID     int `json:"tab_id,omitempty"`
		TimeoutMs int `json:"timeout_ms,omitempty"`
	}
	lenientUnmarshal(args, &params)

	if resp, blocked := checkGuards(req, h.parent.requirePilot, h.parent.requireExtension); blocked {
		return resp
	}
	if resp, blocked := h.parent.requireTabTracking(req); blocked {
		return resp
	}

	if params.TimeoutMs <= 0 {
		params.TimeoutMs = 10_000
	}
	if params.TimeoutMs > 30_000 {
		params.TimeoutMs = 30_000
	}

	correlationID := newCorrelationID(correlationPrefix)
	h.armEvidenceForCommand(correlationID, queryType, args, req.ClientID)

	// Structured params — no embedded script. The content script handles extraction directly.
	queryParams, _ := json.Marshal(map[string]any{
		"timeout_ms": params.TimeoutMs,
	})

	query := queries.PendingQuery{
		Type:          queryType,
		Params:        queryParams,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := h.parent.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	return h.parent.MaybeWaitForCommand(req, correlationID, args, queryType+" queued")
}

func (h *interactActionHandler) handleGetReadable(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.handleContentExtraction(req, args, "get_readable", "readable")
}

func (h *interactActionHandler) handleGetMarkdown(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.handleContentExtraction(req, args, "get_markdown", "markdown")
}

// enrichNavigateResponse appends page content to a successful navigate response.
// Uses the "page_summary" query type to extract text content, headings, and metadata
// via the content script (ISOLATED world, CSP-safe).
func (h *ToolHandler) enrichNavigateResponse(resp JSONRPCResponse, req JSONRPCRequest, tabID int) JSONRPCResponse {
	// Only enrich successful (non-error) responses
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || result.IsError {
		return resp
	}

	// Get current page info from tracking state
	_, _, tabURL := h.capture.GetTrackingStatus()
	tabTitle := h.capture.GetTrackedTabTitle()

	// Get performance vitals
	vitals := h.capture.GetPerformanceSnapshots()

	// Request page summary via dedicated query type (CSP-safe, no eval).
	// Use 4s query timeout to finish before the 5s Go-side wait.
	summaryCorrelationID := newCorrelationID("nav_content")
	summaryParams, _ := json.Marshal(map[string]any{
		"timeout_ms": 4000,
	})
	summaryQuery := queries.PendingQuery{
		Type:          "page_summary",
		Params:        summaryParams,
		TabID:         tabID,
		CorrelationID: summaryCorrelationID,
	}
	if enqueueResp, blocked := h.enqueuePendingQuery(req, summaryQuery, queries.AsyncCommandTimeout); blocked {
		return enqueueResp
	}

	// Wait for page summary (5s — page should already be loaded).
	// Best-effort enrichment: if extraction fails, navigate still succeeds with empty content.
	var textContent string
	cmd, found := h.capture.WaitForCommand(summaryCorrelationID, navigatePageSummaryWait)
	if found && cmd.Status != "pending" && cmd.Result != nil {
		var summaryResult map[string]any
		if json.Unmarshal(cmd.Result, &summaryResult) == nil {
			if preview, ok := summaryResult["main_content_preview"].(string); ok {
				textContent = preview
			}
		}
	}

	// Parse existing result text and append enrichment data
	if len(result.Content) > 0 {
		enrichment := map[string]any{
			"url":          tabURL,
			"title":        tabTitle,
			"text_content": textContent,
		}
		if len(vitals) > 0 {
			enrichment["vitals"] = vitals[len(vitals)-1]
		}
		enrichJSON, _ := json.Marshal(enrichment)
		result.Content = append(result.Content, MCPContentBlock{
			Type: "text",
			Text: "Page content:\n" + string(enrichJSON),
		})
	}

	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
