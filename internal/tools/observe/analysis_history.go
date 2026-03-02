// Purpose: Converts captured navigation actions into stable history responses for observe modes.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
)

type historyEntry struct {
	Timestamp string `json:"timestamp"`
	FromURL   string `json:"from_url,omitempty"`
	ToURL     string `json:"to_url"`
	Type      string `json:"type"`
}

// AnalyzeHistory extracts navigation history from captured user actions.
func AnalyzeHistory(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int  `json:"limit"`
		Summary bool `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)

	actions := deps.GetCapture().GetAllEnhancedActions()
	entries := buildHistoryEntries(actions)
	entries = limitHistoryEntries(entries, clampLimit(params.Limit, 0))

	responseMeta := BuildResponseMetadata(deps.GetCapture(), time.Now())
	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("History", buildHistorySummary(entries, responseMeta))}
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("History", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": responseMeta,
	})}
}

func buildHistoryEntries(actions []capture.EnhancedAction) []historyEntry {
	entries := make([]historyEntry, 0)
	seenURLs := make(map[string]bool)

	for _, a := range actions {
		ts := time.UnixMilli(a.Timestamp).Format(time.RFC3339)
		if a.Type == "navigate" && a.ToURL != "" && !seenURLs[a.ToURL] {
			entries = append(entries, historyEntry{Timestamp: ts, FromURL: a.FromURL, ToURL: a.ToURL, Type: "navigate"})
			seenURLs[a.ToURL] = true
		}
		if a.URL != "" && !seenURLs[a.URL] {
			entries = append(entries, historyEntry{Timestamp: ts, ToURL: a.URL, Type: "page_visit"})
			seenURLs[a.URL] = true
		}
	}
	return entries
}

func limitHistoryEntries(entries []historyEntry, limit int) []historyEntry {
	if limit <= 0 || len(entries) <= limit {
		return entries
	}
	return entries[len(entries)-limit:]
}
