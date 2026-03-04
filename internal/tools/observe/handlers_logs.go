// Purpose: Observe handlers for browser logs with cursor pagination.

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pagination"
)

// GetBrowserLogs returns console log entries with cursor-based pagination.
// #lizard forgives
func GetBrowserLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit             int    `json:"limit"`
		Level             string `json:"level"`
		MinLevel          string `json:"min_level"`
		Source            string `json:"source"`
		URL               string `json:"url"`
		Scope             string `json:"scope"`
		AfterCursor       string `json:"after_cursor"`
		BeforeCursor      string `json:"before_cursor"`
		SinceCursor       string `json:"since_cursor"`
		RestartOnEviction bool   `json:"restart_on_eviction"`
		IncludeInternal   bool   `json:"include_internal"`
		IncludeExtension  bool   `json:"include_extension_logs"`
		ExtensionLimit    int    `json:"extension_limit"`
		Summary           bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidParam, "Invalid scope: "+params.Scope, "Use 'current_page' (default) or 'all'", mcp.WithParam("scope"))}
	}

	_, trackedTabID, trackedTabURL := deps.GetCapture().GetTrackingStatus()
	params.Limit = clampLimit(params.Limit, 100)

	// Default URL filter to the tracked page URL so logs are scoped to
	// the current page, not stale entries from previous navigations.
	if params.URL == "" && params.Scope == "current_page" && trackedTabURL != "" {
		params.URL = trackedTabURL
	}

	rawEntries, _ := deps.GetLogEntries()
	totalAdded := deps.GetLogTotalAdded()

	// Convert to []map[string]any for pagination package.
	allEntries := make([]map[string]any, len(rawEntries))
	for i, e := range rawEntries {
		allEntries[i] = e
	}

	enriched := pagination.EnrichLogEntries(allEntries, totalAdded)

	filtered := make([]pagination.LogEntryWithSequence, 0, len(enriched))
	noiseSuppressed := 0
	for _, e := range enriched {
		entryType, _ := e.Entry["type"].(string)
		if !params.IncludeInternal && isInternalLogType(entryType) {
			continue
		}

		if deps.IsConsoleNoise(e.Entry) {
			noiseSuppressed++
			continue
		}

		if params.Scope == "current_page" && trackedTabID != 0 {
			if !(params.IncludeInternal && isInternalLogType(entryType)) {
				entryTabID, _ := e.Entry["tabId"].(float64)
				if int(entryTabID) != trackedTabID {
					continue
				}
			}
		}

		level, _ := e.Entry["level"].(string)
		if level == "" && isInternalLogType(entryType) {
			level = "info"
		}
		if params.Level != "" && level != params.Level {
			continue
		}

		if params.MinLevel != "" && LogLevelRank(level) < LogLevelRank(params.MinLevel) {
			continue
		}

		if params.Source != "" {
			source, _ := e.Entry["source"].(string)
			if source != params.Source {
				continue
			}
		}

		if params.URL != "" {
			entryURL, _ := e.Entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, params.URL) {
				continue
			}
		}

		filtered = append(filtered, e)
	}

	paginated, pMeta, err := pagination.ApplyLogCursorPagination(
		filtered,
		params.AfterCursor, params.BeforeCursor, params.SinceCursor,
		params.Limit,
		params.RestartOnEviction,
	)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, err.Error(), "Check cursor format or use restart_on_eviction:true")}
	}

	logs := make([]map[string]any, len(paginated))
	for i, e := range paginated {
		logs[i] = normalizeBrowserLogEntry(e.Entry)
	}

	var newestTS time.Time
	if len(paginated) > 0 {
		last := paginated[len(paginated)-1]
		if ts := logEntryTimestamp(last.Entry); ts != "" {
			newestTS = parseRFC3339Flexible(ts)
		}
	}

	isFirstPage := params.AfterCursor == "" && params.BeforeCursor == "" && params.SinceCursor == ""
	meta := BuildPaginatedMetadataWithSummary(deps.GetCapture(), newestTS, pMeta, isFirstPage, func() map[string]any {
		return quickLogsSummary(logs)
	})
	meta["scope"] = params.Scope
	if noiseSuppressed > 0 {
		meta["noise_suppressed"] = noiseSuppressed
	}

	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser logs", buildLogsSummary(logs, meta))}
	}

	response := map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": meta,
	}

	if params.IncludeExtension {
		limit := params.ExtensionLimit
		if limit <= 0 {
			limit = params.Limit
		}
		limit = clampLimit(limit, 100)
		extLogs := buildExtensionLogEntries(deps.GetCapture().GetExtensionLogs(), limit, params.Level)
		response["extension_logs"] = extLogs
		response["extension_logs_count"] = len(extLogs)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser logs", response)}
}
