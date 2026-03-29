// Purpose: Implements DOM-centric analyze modes (dom and page_summary).
// Why: Separates DOM queueing and page-summary dispatch from unrelated analyze handlers.
// Docs: docs/features/feature/analyze-tool/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// toolQueryDOM submits a DOM query to the extension and optionally waits for completion.
func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Selector string `json:"selector"`
		TabID    int    `json:"tab_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Issue #274: selector is optional. Default to "*" for a full DOM dump.
	queryArgs := args
	if params.Selector == "" {
		var raw map[string]any
		if json.Unmarshal(args, &raw) != nil || raw == nil {
			raw = make(map[string]any)
		}
		raw["selector"] = "*"
		// Marshal cannot realistically fail with string/map values; silent fallback is acceptable.
		if marshaled, err := json.Marshal(raw); err == nil {
			queryArgs = marshaled
		}
	}

	correlationID := newCorrelationID("dom")
	query := queries.PendingQuery{
		Type:          "dom",
		Params:        queryArgs,
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if resp, blocked := h.enqueuePendingQuery(req, query, queries.AsyncCommandTimeout); blocked {
		return resp
	}

	return h.MaybeWaitForCommand(req, correlationID, queryArgs, "DOM query queued")
}

func (h *ToolHandler) toolAnalyzePageSummary(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Delegates to shared content extraction which handles gate checks, timeout validation, and query creation.
	return h.interactAction().handleContentExtraction(req, args, "page_summary", "page_summary")
}
