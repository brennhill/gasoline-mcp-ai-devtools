// Purpose: Implements observe tool queries against captured runtime buffers.
// Docs: docs/features/feature/observe/index.md

// tools_observe.go — MCP observe tool dispatcher and handlers.
// Handles all observe modes: errors, logs, network, websocket, actions, etc.
package main

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/pagination"
	"github.com/dev-console/dev-console/internal/tools/observe"
)

// maxObserveLimit caps the limit parameter to prevent oversized responses.
const maxObserveLimit = 1000

// ObserveHandler is the function signature for observe tool handlers.
type ObserveHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// observeHandlers maps observe mode names to their handler functions.
// Modes handled by internal/tools/observe delegate to the extracted package.
// Modes that depend on async/recording subsystems remain local.
var observeHandlers = map[string]ObserveHandler{
	// Delegated to internal/tools/observe
	"errors": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetBrowserErrors(h, req, args)
	},
	"logs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetBrowserLogs(h, req, args)
	},
	"extension_logs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetExtensionLogs(h, req, args)
	},
	"network_waterfall": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetNetworkWaterfall(h, req, args)
	},
	"network_bodies": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetNetworkBodies(h, req, args)
	},
	"websocket_events": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetWSEvents(h, req, args)
	},
	"websocket_status": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetWSStatus(h, req, args)
	},
	"actions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetEnhancedActions(h, req, args)
	},
	"vitals": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetWebVitals(h, req, args)
	},
	"page": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetPageInfo(h, req, args)
	},
	"tabs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetTabs(h, req, args)
	},
	"pilot": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.ObservePilot(h, req, args)
	},
	"timeline": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetSessionTimeline(h, req, args)
	},
	"error_bundles": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetErrorBundles(h, req, args)
	},
	"screenshot": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetScreenshot(h, req, args)
	},
	// Local handlers — depend on async/recording subsystems
	"command_result": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveCommandResult(req, args)
	},
	"pending_commands": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObservePendingCommands(req, args)
	},
	"failed_commands": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveFailedCommands(req, args)
	},
	"saved_videos": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveSavedVideos(req, args)
	},
	"recordings": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetRecordings(req, args)
	},
	"recording_actions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetRecordingActions(req, args)
	},
	"playback_results": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetPlaybackResults(req, args)
	},
	"log_diff_report": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetLogDiffReport(req, args)
	},
}

// getValidObserveModes returns a sorted, comma-separated list of valid observe modes.
func getValidObserveModes() string {
	modes := make([]string, 0, len(observeHandlers))
	for mode := range observeHandlers {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.What == "" {
		validModes := getValidObserveModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validModes))}
	}

	handler, ok := observeHandlers[params.What]
	if !ok {
		validModes := getValidObserveModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown observe mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes))}
	}

	resp := handler(h, req, args)

	// Warn when extension is disconnected (except for server-side modes that don't need it)
	if !h.capture.IsExtensionConnected() && !isServerSideObserveMode(params.What) {
		resp = h.prependDisconnectWarning(resp)
	}

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

	return resp
}

// isServerSideObserveMode returns true for observe modes that don't depend on live extension data.
func isServerSideObserveMode(mode string) bool {
	switch mode {
	case "command_result", "pending_commands", "failed_commands",
		"saved_videos", "recordings", "recording_actions", "playback_results",
		"log_diff_report", "pilot":
		return true
	}
	return false
}

// prependDisconnectWarning adds a warning to the first content block when the extension is disconnected.
func (h *ToolHandler) prependDisconnectWarning(resp JSONRPCResponse) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return resp
	}

	warning := "⚠ Extension is not connected — results may be stale or empty. Ensure the Gasoline extension shows 'Connected' and a tab is tracked.\n\n"
	result.Content[0].Text = warning + result.Content[0].Text

	// Error impossible: simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// appendAlertsToResponse adds an alerts content block to an existing MCP response.
func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	alertText := formatAlertsBlock(alerts)
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: alertText,
	})

	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// ============================================
// Observe sub-handlers
// ============================================

func (h *ToolHandler) toolGetBrowserErrors(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit int    `json:"limit"`
		URL   string `json:"url"`   // filter by URL substring
		Scope string `json:"scope"` // current_page (default) or all
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid scope: "+params.Scope, "Use 'current_page' (default) or 'all'", withParam("scope"))}
	}

	// Get tracked tab ID for current_page filtering
	_, trackedTabID, _ := h.capture.GetTrackingStatus()

	// Copy slice reference under lock, iterate outside.
	// Safe because addEntries creates new slices on rotation.
	h.server.mu.RLock()
	entries := h.server.entries
	h.server.mu.RUnlock()

	errors := make([]map[string]any, 0)
	for i := len(entries) - 1; i >= 0 && len(errors) < params.Limit; i-- {
		entry := entries[i]
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}

		// Filter by current page tab_id if scope=current_page
		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}

		// Filter by URL if specified
		if params.URL != "" {
			entryURL, _ := entry["url"].(string)
			if !containsIgnoreCase(entryURL, params.URL) {
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

	// Find newest entry timestamp for staleness metadata
	var newestTS time.Time
	if len(errors) > 0 {
		if ts, ok := errors[0]["timestamp"].(string); ok {
			newestTS, _ = time.Parse(time.RFC3339, ts)
		}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Browser errors", map[string]any{
		"errors": errors,
		"count":  len(errors),
		"metadata": buildResponseMetadata(h.capture, newestTS),
		"scope":    params.Scope,
	})}
}

// Note: logLevelRank has been moved to observe_filtering.go

// #lizard forgives
func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters including cursor pagination
	var params struct {
		Limit             int    `json:"limit"`
		Level             string `json:"level"`               // exact level match
		MinLevel          string `json:"min_level"`           // minimum level threshold
		Source            string `json:"source"`              // filter by source
		URL               string `json:"url"`                 // filter by URL substring
		Scope             string `json:"scope"`               // current_page (default) or all
		AfterCursor       string `json:"after_cursor"`        // cursor-based forward pagination
		BeforeCursor      string `json:"before_cursor"`       // cursor-based backward pagination
		SinceCursor       string `json:"since_cursor"`        // cursor-based since (inclusive)
		RestartOnEviction bool   `json:"restart_on_eviction"` // auto-restart on cursor expiration
	}
	lenientUnmarshal(args, &params)
	if params.Scope == "" {
		params.Scope = "current_page"
	}
	if params.Scope != "current_page" && params.Scope != "all" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Invalid scope: "+params.Scope, "Use 'current_page' (default) or 'all'", withParam("scope"))}
	}

	// Get tracked tab ID for current_page filtering
	_, trackedTabID, _ := h.capture.GetTrackingStatus()
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	// Copy slice reference and totalAdded under lock.
	// Safe because addEntries creates new slices on rotation.
	h.server.mu.RLock()
	rawEntries := h.server.entries
	totalAdded := h.server.logTotalAdded
	h.server.mu.RUnlock()

	// Convert []LogEntry (named type) to []map[string]any for pagination package.
	// Only copies slice of map references, not map contents.
	allEntries := make([]map[string]any, len(rawEntries))
	for i, e := range rawEntries {
		allEntries[i] = e
	}

	// Enrich entries with sequence numbers for cursor pagination
	enriched := pagination.EnrichLogEntries(allEntries, totalAdded)

	// Apply content filters on enriched entries
	filtered := make([]pagination.LogEntryWithSequence, 0, len(enriched))
	for _, e := range enriched {
		// Skip non-console entries (e.g., lifecycle events)
		entryType, _ := e.Entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}

		// Filter by current page tab_id if scope=current_page
		if params.Scope == "current_page" && trackedTabID != 0 {
			entryTabID, _ := e.Entry["tabId"].(float64)
			if int(entryTabID) != trackedTabID {
				continue
			}
		}

		// Filter by exact level if specified
		level, _ := e.Entry["level"].(string)
		if params.Level != "" && level != params.Level {
			continue
		}

		// Filter by minimum level if specified
		if params.MinLevel != "" && logLevelRank(level) < logLevelRank(params.MinLevel) {
			continue
		}

		// Filter by source if specified
		if params.Source != "" {
			source, _ := e.Entry["source"].(string)
			if source != params.Source {
				continue
			}
		}

		// Filter by URL if specified
		if params.URL != "" {
			entryURL, _ := e.Entry["url"].(string)
			if !containsIgnoreCase(entryURL, params.URL) {
				continue
			}
		}

		filtered = append(filtered, e)
	}

	// Apply cursor pagination
	paginated, pMeta, err := pagination.ApplyLogCursorPagination(
		filtered,
		params.AfterCursor, params.BeforeCursor, params.SinceCursor,
		params.Limit,
		params.RestartOnEviction,
	)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam, err.Error(), "Check cursor format or use restart_on_eviction:true")}
	}

	// Serialize entries
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

	// Use newest entry timestamp for data-age calculation
	var newestTS time.Time
	if len(paginated) > 0 {
		last := paginated[len(paginated)-1]
		if ts, ok := last.Entry["ts"].(string); ok {
			newestTS, _ = time.Parse(time.RFC3339, ts)
		}
	}

	meta := buildPaginatedResponseMetadata(h.capture, newestTS, pMeta)
	meta["scope"] = params.Scope

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Browser logs", map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": meta,
	})}
}

func (h *ToolHandler) toolGetExtensionLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit int    `json:"limit"`
		Level string `json:"level"` // filter by level
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	// Read extension logs from capture buffer
	allLogs := h.capture.GetExtensionLogs()

	// Filter and limit (newest first)
	logs := make([]map[string]any, 0)
	for i := len(allLogs) - 1; i >= 0 && len(logs) < params.Limit; i-- {
		entry := allLogs[i]

		// Filter by level if specified
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

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Extension logs", map[string]any{
		"logs":     logs,
		"count":    len(logs),
		"metadata": buildResponseMetadata(h.capture, newestTS),
	})}
}

// ============================================
// Simple delegator handlers
// ============================================

// #lizard forgives
func (h *ToolHandler) toolGetNetworkBodies(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		BodyKey   string `json:"body_key"`
		BodyPath  string `json:"body_path"`
	}
	lenientUnmarshal(args, &params)
	if params.BodyKey != "" && params.BodyPath != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Only one body filter can be used at a time",
			"Use either 'body_key' or 'body_path', not both",
			withParam("body_key"),
			withParam("body_path"),
		)}
	}
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	allBodies := h.capture.GetNetworkBodies()
	filtered := make([]capture.NetworkBody, 0)
	for i := len(allBodies) - 1; i >= 0 && len(filtered) < params.Limit; i-- {
		b := allBodies[i]
		if params.URL != "" && !containsIgnoreCase(b.URL, params.URL) {
			continue
		}
		if params.Method != "" && !containsIgnoreCase(b.Method, params.Method) {
			continue
		}
		if params.StatusMin > 0 && b.Status < params.StatusMin {
			continue
		}
		if params.StatusMax > 0 && b.Status > params.StatusMax {
			continue
		}
		filteredBody, include, err := applyNetworkBodyFilter(b, params.BodyKey, params.BodyPath)
		if err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid network body filter: "+err.Error(),
				"Use a valid body_path syntax like data.items[0].id",
				withParam("body_path"),
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

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network bodies", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": buildResponseMetadata(h.capture, newestTS),
	})}
}

func (h *ToolHandler) toolGetWSEvents(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit        int    `json:"limit"`
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
		Direction    string `json:"direction"`
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	allEvents := h.capture.GetAllWebSocketEvents()
	filtered := make([]capture.WebSocketEvent, 0)
	for i := len(allEvents) - 1; i >= 0 && len(filtered) < params.Limit; i-- {
		evt := allEvents[i]
		if params.URL != "" && !containsIgnoreCase(evt.URL, params.URL) {
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

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket events", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": buildResponseMetadata(h.capture, newestTS),
	})}
}

func (h *ToolHandler) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit int    `json:"limit"`
		URL   string `json:"url"`
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	allActions := h.capture.GetAllEnhancedActions()
	filtered := make([]capture.EnhancedAction, 0)
	for i := len(allActions) - 1; i >= 0 && len(filtered) < params.Limit; i-- {
		a := allActions[i]
		if params.URL != "" && !containsIgnoreCase(a.URL, params.URL) {
			continue
		}
		filtered = append(filtered, a)
	}
	var newestTS time.Time
	if len(allActions) > 0 {
		newestTS = time.UnixMilli(allActions[len(allActions)-1].Timestamp)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Enhanced actions", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": buildResponseMetadata(h.capture, newestTS),
	})}
}

func (h *ToolHandler) toolObservePilot(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	status := h.capture.GetPilotStatus()
	if statusMap, ok := status.(map[string]any); ok {
		statusMap["metadata"] = buildResponseMetadata(h.capture, time.Now())
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Pilot status", status)}
}

func (h *ToolHandler) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	snapshots := h.capture.GetPerformanceSnapshots()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Performance", map[string]any{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})}
}
