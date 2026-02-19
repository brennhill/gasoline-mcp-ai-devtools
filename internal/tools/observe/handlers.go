// handlers.go — Core observe tool handlers for buffer-backed queries.
package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/pagination"
)

// MaxObserveLimit caps the limit parameter to prevent oversized responses.
const MaxObserveLimit = 1000

// Handler is the function signature for observe tool handlers.
type Handler func(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

// Handlers maps observe mode names to their handler functions.
// Modes not in this map (command_result, pending_commands, etc.) are handled by cmd/dev-console.
var Handlers = map[string]Handler{
	"errors":            GetBrowserErrors,
	"logs":              GetBrowserLogs,
	"extension_logs":    GetExtensionLogs,
	"network_waterfall": GetNetworkWaterfall,
	"network_bodies":    GetNetworkBodies,
	"websocket_events":  GetWSEvents,
	"websocket_status":  GetWSStatus,
	"actions":           GetEnhancedActions,
	"vitals":            GetWebVitals,
	"page":              GetPageInfo,
	"tabs":              GetTabs,
	"pilot":             ObservePilot,
	"timeline":          GetSessionTimeline,
	"error_bundles":     GetErrorBundles,
	"screenshot":        GetScreenshot,
}

// clampLimit applies default and max bounds to a limit parameter.
func clampLimit(limit, defaultVal int) int {
	if limit <= 0 {
		return defaultVal
	}
	if limit > MaxObserveLimit {
		return MaxObserveLimit
	}
	return limit
}

// GetBrowserErrors returns error-level log entries from the capture buffer.
func GetBrowserErrors(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit int    `json:"limit"`
		URL   string `json:"url"`
		Scope string `json:"scope"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidParam, "Invalid scope: "+params.Scope, "Use 'current_page' (default) or 'all'", mcp.WithParam("scope"))}
	}

	_, trackedTabID, _ := deps.GetCapture().GetTrackingStatus()

	entries, _ := deps.GetLogEntries()

	errors := make([]map[string]any, 0)
	noiseSuppressed := 0
	for i := len(entries) - 1; i >= 0 && len(errors) < params.Limit; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}

		if deps.IsConsoleNoise(entry) {
			noiseSuppressed++
			continue
		}

		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}

		if params.URL != "" {
			entryURL, _ := entry["url"].(string)
			if !ContainsIgnoreCase(entryURL, params.URL) {
				continue
			}
		}

		errors = append(errors, map[string]any{
			"message":   entry["message"],
			"source":    entry["source"],
			"url":       entry["url"],
			"line":      entry["line"],
			"column":    entry["column"],
			"stack":     entry["stack"],
			"timestamp": entry["ts"],
			"tab_id":    entry["tabId"],
		})
	}

	var newestTS time.Time
	if len(errors) > 0 {
		if ts, ok := errors[0]["timestamp"].(string); ok {
			newestTS, _ = time.Parse(time.RFC3339, ts)
		}
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	responseMeta.NoiseSuppressed = noiseSuppressed

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser errors", map[string]any{
		"errors":   errors,
		"count":    len(errors),
		"metadata": responseMeta,
		"scope":    params.Scope,
	})}
}

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
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidParam, "Invalid scope: "+params.Scope, "Use 'current_page' (default) or 'all'", mcp.WithParam("scope"))}
	}

	_, trackedTabID, _ := deps.GetCapture().GetTrackingStatus()
	params.Limit = clampLimit(params.Limit, 100)

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
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}

		if deps.IsConsoleNoise(e.Entry) {
			noiseSuppressed++
			continue
		}

		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := e.Entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}

		level, _ := e.Entry["level"].(string)
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
		logs[i] = map[string]any{
			"level":     e.Entry["level"],
			"message":   e.Entry["message"],
			"source":    e.Entry["source"],
			"url":       e.Entry["url"],
			"line":      e.Entry["line"],
			"column":    e.Entry["column"],
			"timestamp": e.Entry["ts"],
			"tab_id":    e.Entry["tabId"],
		}
	}

	var newestTS time.Time
	if len(paginated) > 0 {
		last := paginated[len(paginated)-1]
		if ts, ok := last.Entry["ts"].(string); ok {
			newestTS, _ = time.Parse(time.RFC3339, ts)
		}
	}

	meta := BuildPaginatedResponseMetadata(deps.GetCapture(), newestTS, pMeta)
	meta["scope"] = params.Scope
	if noiseSuppressed > 0 {
		meta["noise_suppressed"] = noiseSuppressed
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser logs", map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": meta,
	})}
}

// GetExtensionLogs returns internal extension debug logs.
func GetExtensionLogs(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit int    `json:"limit"`
		Level string `json:"level"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allLogs := deps.GetCapture().GetExtensionLogs()

	logs := make([]map[string]any, 0)
	for i := len(allLogs) - 1; i >= 0 && len(logs) < params.Limit; i-- {
		entry := allLogs[i]
		if params.Level != "" && entry.Level != params.Level {
			continue
		}
		logs = append(logs, map[string]any{
			"level":     entry.Level,
			"message":   entry.Message,
			"source":    entry.Source,
			"category":  entry.Category,
			"data":      entry.Data,
			"timestamp": entry.Timestamp,
		})
	}

	var newestTS time.Time
	if len(allLogs) > 0 {
		newestTS = allLogs[len(allLogs)-1].Timestamp
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Extension logs", map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})}
}

// GetNetworkBodies returns captured HTTP response bodies with optional filtering.
// #lizard forgives
func GetNetworkBodies(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		BodyKey   string `json:"body_key"`
		BodyPath  string `json:"body_path"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.BodyKey != "" && params.BodyPath != "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam,
			"Only one body filter can be used at a time",
			"Use either 'body_key' or 'body_path', not both",
			mcp.WithParam("body_key"),
			mcp.WithParam("body_path"),
		)}
	}
	params.Limit = clampLimit(params.Limit, 100)

	allBodies := deps.GetCapture().GetNetworkBodies()
	filtered := make([]capture.NetworkBody, 0)
	for i := len(allBodies) - 1; i >= 0 && len(filtered) < params.Limit; i-- {
		b := allBodies[i]
		if params.URL != "" && !ContainsIgnoreCase(b.URL, params.URL) {
			continue
		}
		if params.Method != "" && !ContainsIgnoreCase(b.Method, params.Method) {
			continue
		}
		if params.StatusMin > 0 && b.Status < params.StatusMin {
			continue
		}
		if params.StatusMax > 0 && b.Status > params.StatusMax {
			continue
		}
		filteredBody, include, err := ApplyNetworkBodyFilter(b, params.BodyKey, params.BodyPath)
		if err != nil {
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
				mcp.ErrInvalidParam,
				"Invalid network body filter: "+err.Error(),
				"Use a valid body_path syntax like data.items[0].id",
				mcp.WithParam("body_path"),
			)}
		}
		if !include {
			continue
		}
		filtered = append(filtered, filteredBody)
	}
	var newestTS time.Time
	if len(allBodies) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allBodies[len(allBodies)-1].Timestamp)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network bodies", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})}
}

// GetWSEvents returns captured WebSocket events with optional filtering.
func GetWSEvents(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit        int    `json:"limit"`
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
		Direction    string `json:"direction"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allEvents := deps.GetCapture().GetAllWebSocketEvents()
	filtered := make([]capture.WebSocketEvent, 0)
	for i := len(allEvents) - 1; i >= 0 && len(filtered) < params.Limit; i-- {
		evt := allEvents[i]
		if params.URL != "" && !ContainsIgnoreCase(evt.URL, params.URL) {
			continue
		}
		if params.ConnectionID != "" && evt.ID != params.ConnectionID {
			continue
		}
		if params.Direction != "" && evt.Direction != params.Direction {
			continue
		}
		filtered = append(filtered, evt)
	}
	var newestTS time.Time
	if len(allEvents) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allEvents[len(allEvents)-1].Timestamp)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket events", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})}
}

// GetEnhancedActions returns captured user actions (clicks, inputs, navigations).
func GetEnhancedActions(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit int    `json:"limit"`
		URL   string `json:"url"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allActions := deps.GetCapture().GetAllEnhancedActions()
	filtered := make([]capture.EnhancedAction, 0)
	for i := len(allActions) - 1; i >= 0 && len(filtered) < params.Limit; i-- {
		a := allActions[i]
		if params.URL != "" && !ContainsIgnoreCase(a.URL, params.URL) {
			continue
		}
		filtered = append(filtered, a)
	}
	var newestTS time.Time
	if len(allActions) > 0 {
		newestTS = time.UnixMilli(allActions[len(allActions)-1].Timestamp)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Enhanced actions", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})}
}

// ObservePilot returns the current pilot/extension connection status.
func ObservePilot(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	status := deps.GetCapture().GetPilotStatus()
	if statusMap, ok := status.(map[string]any); ok {
		statusMap["metadata"] = BuildResponseMetadata(deps.GetCapture(), time.Now())
	}
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Pilot status", status)}
}

// CheckPerformance returns performance snapshots from the capture buffer.
func CheckPerformance(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	snapshots := deps.GetCapture().GetPerformanceSnapshots()
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Performance", map[string]any{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})}
}
