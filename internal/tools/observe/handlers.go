// Purpose: Defines the Handler function type and dispatches observe mode calls with limit/filter/cursor support.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/buffers"
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
	"history":           AnalyzeHistory,
	"pilot":             ObservePilot,
	"timeline":          GetSessionTimeline,
	"error_bundles":     GetErrorBundles,
	"screenshot":        GetScreenshot,
	"storage":           GetStorage,
	"indexeddb":         GetIndexedDB,
	"summarized_logs":   GetSummarizedLogs,
	"transients":        GetTransients,
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

	_, trackedTabID, _ := deps.GetCapture().GetTrackingStatus()

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

func isInternalLogType(entryType string) bool {
	return entryType == "lifecycle" || entryType == "tracking" || entryType == "extension"
}

func logEntryTimestamp(entry map[string]any) string {
	if ts, ok := entry["ts"].(string); ok && ts != "" {
		return ts
	}
	if ts, ok := entry["timestamp"].(string); ok && ts != "" {
		return ts
	}
	return ""
}

func parseRFC3339Flexible(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return parsed
	}
	parsed, _ := time.Parse(time.RFC3339, ts)
	return parsed
}

func normalizeBrowserLogEntry(entry map[string]any) map[string]any {
	entryType, _ := entry["type"].(string)
	level, _ := entry["level"].(string)
	if level == "" && isInternalLogType(entryType) {
		level = "info"
	}

	message, _ := entry["message"].(string)
	if message == "" {
		if event, ok := entry["event"].(string); ok {
			message = event
		}
	}

	source, _ := entry["source"].(string)
	if source == "" && isInternalLogType(entryType) {
		source = "daemon"
	}

	normalized := map[string]any{
		"level":     level,
		"message":   message,
		"source":    source,
		"url":       entry["url"],
		"line":      entry["line"],
		"column":    entry["column"],
		"timestamp": logEntryTimestamp(entry),
		"tab_id":    entry["tabId"],
	}

	if entryType != "" {
		normalized["type"] = entryType
	}
	if event, ok := entry["event"]; ok {
		normalized["event"] = event
	}
	if pid, ok := entry["pid"]; ok {
		normalized["pid"] = pid
	}
	if port, ok := entry["port"]; ok {
		normalized["port"] = port
	}

	extras := make(map[string]any)
	for k, v := range entry {
		switch k {
		case "type", "level", "message", "source", "url", "line", "column", "ts", "timestamp", "tabId", "event", "pid", "port":
			// handled above
		default:
			extras[k] = v
		}
	}
	if len(extras) > 0 {
		normalized["data"] = extras
	}

	return normalized
}

func buildExtensionLogEntries(allLogs []capture.ExtensionLog, limit int, level string) []map[string]any {
	matched := buffers.ReverseFilterLimit(allLogs, func(entry capture.ExtensionLog) bool {
		return level == "" || entry.Level == level
	}, limit)

	logs := make([]map[string]any, len(matched))
	for i, entry := range matched {
		logs[i] = map[string]any{
			"level":     entry.Level,
			"message":   entry.Message,
			"source":    entry.Source,
			"category":  entry.Category,
			"data":      entry.Data,
			"timestamp": entry.Timestamp,
		}
	}
	return logs
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

	matched := buffers.ReverseFilterLimit(allLogs, func(entry capture.ExtensionLog) bool {
		return params.Level == "" || entry.Level == params.Level
	}, params.Limit)

	logs := make([]map[string]any, len(matched))
	for i, entry := range matched {
		logs[i] = map[string]any{
			"level":     entry.Level,
			"message":   entry.Message,
			"source":    entry.Source,
			"category":  entry.Category,
			"data":      entry.Data,
			"timestamp": entry.Timestamp,
		}
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
		Summary   bool   `json:"summary"`
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
	var bodyFilterErr error
	filtered := buffers.ReverseFilterLimit(allBodies, func(b capture.NetworkBody) bool {
		if bodyFilterErr != nil {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(b.URL, params.URL) {
			return false
		}
		if params.Method != "" && !ContainsIgnoreCase(b.Method, params.Method) {
			return false
		}
		if params.StatusMin > 0 && b.Status < params.StatusMin {
			return false
		}
		if params.StatusMax > 0 && b.Status > params.StatusMax {
			return false
		}
		_, include, err := ApplyNetworkBodyFilter(b, params.BodyKey, params.BodyPath)
		if err != nil {
			bodyFilterErr = err
			return false
		}
		return include
	}, params.Limit)

	if bodyFilterErr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam,
			"Invalid network body filter: "+bodyFilterErr.Error(),
			"Use a valid body_path syntax like data.items[0].id",
			mcp.WithParam("body_path"),
		)}
	}

	// Re-apply body filter to transform matched entries (extract body_key/body_path).
	if params.BodyKey != "" || params.BodyPath != "" {
		for i, b := range filtered {
			filteredBody, _, _ := ApplyNetworkBodyFilter(b, params.BodyKey, params.BodyPath)
			filtered[i] = filteredBody
		}
	}
	var newestTS time.Time
	if len(allBodies) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allBodies[len(allBodies)-1].Timestamp)
	}

	waterfallCount := len(deps.GetCapture().GetNetworkWaterfallEntries())
	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		summary := buildNetworkBodiesSummary(filtered, responseMeta)
		if len(filtered) == 0 {
			// TODO: Extend hints to reflect method/status/body_key/body_path filters, not just URL.
			summary["hint"] = networkBodiesEmptyHint(waterfallCount, len(allBodies), params.URL)
		}
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network bodies", summary)}
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}

	if len(filtered) == 0 {
		// TODO: Extend hints to reflect method/status/body_key/body_path filters, not just URL.
		response["hint"] = networkBodiesEmptyHint(waterfallCount, len(allBodies), params.URL)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network bodies", response)}
}

// GetWSEvents returns captured WebSocket events with optional filtering.
func GetWSEvents(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit        int    `json:"limit"`
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
		Direction    string `json:"direction"`
		Summary      bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allEvents := deps.GetCapture().GetAllWebSocketEvents()
	filtered := buffers.ReverseFilterLimit(allEvents, func(evt capture.WebSocketEvent) bool {
		if params.URL != "" && !ContainsIgnoreCase(evt.URL, params.URL) {
			return false
		}
		if params.ConnectionID != "" && evt.ID != params.ConnectionID {
			return false
		}
		if params.Direction != "" && evt.Direction != params.Direction {
			return false
		}
		return true
	}, params.Limit)
	var newestTS time.Time
	if len(allEvents) > 0 {
		newestTS, _ = time.Parse(time.RFC3339, allEvents[len(allEvents)-1].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		summary := buildWSEventsSummary(filtered, responseMeta)
		if len(filtered) == 0 {
			summary["hint"] = wsEventsEmptyHint(len(allEvents), params.URL)
		}
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket events", summary)}
	}

	response := map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	}

	if len(filtered) == 0 {
		response["hint"] = wsEventsEmptyHint(len(allEvents), params.URL)
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket events", response)}
}

// GetEnhancedActions returns captured user actions (clicks, inputs, navigations).
// Supports optional "type" filter to return only actions of a specific type.
func GetEnhancedActions(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int    `json:"limit"`
		URL     string `json:"url"`
		Type    string `json:"type"`
		Summary bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allActions := deps.GetCapture().GetAllEnhancedActions()
	filtered := buffers.ReverseFilterLimit(allActions, func(a capture.EnhancedAction) bool {
		if params.Type != "" && a.Type != params.Type {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(a.URL, params.URL) {
			return false
		}
		return true
	}, params.Limit)
	var newestTS time.Time
	if len(allActions) > 0 {
		newestTS = time.UnixMilli(allActions[len(allActions)-1].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Enhanced actions", buildActionsSummary(filtered, responseMeta))}
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Enhanced actions", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	})}
}

// GetTransients returns captured transient UI elements (toasts, alerts, snackbars).
// Filters enhanced actions for type == "transient" with optional classification and URL filters.
func GetTransients(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit          int    `json:"limit"`
		URL            string `json:"url"`
		Classification string `json:"classification"`
		Summary        bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	// Lower default than other handlers (50 vs 100): transients are less frequent than logs/actions.
	// MVP: duration_ms is always 0 — removal tracking is not yet implemented.
	params.Limit = clampLimit(params.Limit, 50)

	allActions := deps.GetCapture().GetAllEnhancedActions()
	filtered := buffers.ReverseFilterLimit(allActions, func(a capture.EnhancedAction) bool {
		if a.Type != "transient" {
			return false
		}
		if params.URL != "" && !ContainsIgnoreCase(a.URL, params.URL) {
			return false
		}
		if params.Classification != "" && a.Classification != params.Classification {
			return false
		}
		return true
	}, params.Limit)

	var newestTS time.Time
	if len(filtered) > 0 {
		newestTS = time.UnixMilli(filtered[0].Timestamp)
	}

	responseMeta := BuildResponseMetadata(deps.GetCapture(), newestTS)
	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Transient elements", buildTransientsSummary(filtered, responseMeta))}
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Transient elements", map[string]any{
		"entries":  filtered,
		"count":    len(filtered),
		"metadata": responseMeta,
	})}
}

// buildTransientsSummary returns {total, by_classification, metadata}.
func buildTransientsSummary(actions []capture.EnhancedAction, meta ResponseMetadata) map[string]any {
	byClassification := make(map[string]int)
	for _, a := range actions {
		cls := a.Classification
		if cls == "" {
			cls = "unknown"
		}
		byClassification[cls]++
	}

	return map[string]any{
		"total":             len(actions),
		"by_classification": byClassification,
		"metadata":          meta,
	}
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
