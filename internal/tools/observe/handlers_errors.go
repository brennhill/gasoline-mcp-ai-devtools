// Purpose: Observe handlers for browser error-level logs.

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/buffers"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

// GetBrowserErrors returns error-level log entries from the capture buffer.
func GetBrowserErrors(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int    `json:"limit"`
		URL     string `json:"url"`
		Scope   string `json:"scope"`
		Summary bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidParam, "Invalid scope: "+params.Scope, "Use 'current_page' (default) or 'all'", mcp.WithParam("scope"))}
	}

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}
	entries, _ := deps.GetLogEntries()

	noiseSuppressed := 0
	matched := buffers.ReverseFilterLimit(entries, func(entry map[string]any) bool {
		level, _ := entry["level"].(string)
		if level != "error" {
			return false
		}
		if deps.IsConsoleNoise(entry) {
			noiseSuppressed++
			return false
		}
		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				return false
			}
		}
		if params.URL != "" {
			entryURL, _ := entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, params.URL) {
				return false
			}
		}
		return true
	}, params.Limit)

	errors := make([]map[string]any, len(matched))
	for i, entry := range matched {
		errors[i] = map[string]any{
			"message":   entry["message"],
			"source":    entry["source"],
			"url":       entry["url"],
			"line":      entry["line"],
			"column":    entry["column"],
			"stack":     entry["stack"],
			"timestamp": entry["ts"],
			"tab_id":    entry["tabId"],
		}
	}

	var newestTS time.Time
	if len(errors) > 0 {
		if ts, ok := errors[0]["timestamp"].(string); ok {
			newestTS, _ = time.Parse(time.RFC3339, ts)
		}
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	responseMeta.NoiseSuppressed = noiseSuppressed

	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser errors", buildErrorsSummary(errors, noiseSuppressed, responseMeta))}
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser errors", map[string]any{
		"errors":   errors,
		"count":    len(errors),
		"metadata": responseMeta,
		"scope":    params.Scope,
	})}
}
