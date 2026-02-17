// Purpose: Implements observe tool queries against captured runtime buffers.
// Docs: docs/features/feature/observe/index.md

// tools_observe_analysis.go — Observe analysis handlers: waterfall, vitals, tabs, a11y, timeline, errors, history, async commands.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/performance"
	"github.com/dev-console/dev-console/internal/queries"
)

func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URLFilter string `json:"url"`
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	allEntries := h.refreshWaterfallIfStale()

	entries := filterWaterfallEntries(allEntries, params.URLFilter, params.Limit)

	var newestTS time.Time
	if len(allEntries) > 0 {
		newestTS = allEntries[len(allEntries)-1].Timestamp
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network waterfall", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": buildResponseMetadata(h.capture, newestTS),
	})}
}

func (h *ToolHandler) refreshWaterfallIfStale() []capture.NetworkWaterfallEntry {
	allEntries := h.capture.GetNetworkWaterfallEntries()
	if len(allEntries) > 0 && time.Since(allEntries[len(allEntries)-1].Timestamp) < 1*time.Second {
		return allEntries
	}

	queryID := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "waterfall",
			Params: json.RawMessage(`{}`),
		},
		5*time.Second,
		"",
	)

	result, err := h.capture.WaitForResult(queryID, 5*time.Second)
	if err != nil || result == nil {
		return allEntries
	}

	var waterfallResult struct {
		Entries []capture.NetworkWaterfallEntry `json:"entries"`
		PageURL string                          `json:"page_url"`
	}
	if err := json.Unmarshal(result, &waterfallResult); err == nil && len(waterfallResult.Entries) > 0 {
		h.capture.AddNetworkWaterfallEntries(waterfallResult.Entries, waterfallResult.PageURL)
		return h.capture.GetNetworkWaterfallEntries()
	}
	return allEntries
}

func filterWaterfallEntries(allEntries []capture.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	entries := make([]map[string]any, 0)
	for i := len(allEntries) - 1; i >= 0 && len(entries) < limit; i-- {
		entry := allEntries[i]
		if urlFilter != "" && (entry.URL == "" || !containsIgnoreCase(entry.URL, urlFilter)) {
			continue
		}
		entries = append(entries, waterfallEntryToMap(entry))
	}
	return entries
}

func waterfallEntryToMap(entry capture.NetworkWaterfallEntry) map[string]any {
	return map[string]any{
		"url":               entry.URL,
		"initiator_type":    entry.InitiatorType,
		"duration_ms":       entry.Duration,
		"start_time":        entry.StartTime,
		"transfer_size":     entry.TransferSize,
		"decoded_body_size": entry.DecodedBodySize,
		"encoded_body_size": entry.EncodedBodySize,
		"timestamp":         entry.Timestamp,
		"page_url":          entry.PageURL,
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func (h *ToolHandler) toolGetWSStatus(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid arguments JSON: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	filter := capture.WebSocketStatusFilter{
		URLFilter:    arguments.URL,
		ConnectionID: arguments.ConnectionID,
	}
	status := h.capture.GetWebSocketStatus(filter)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket status", map[string]any{
		"connections":  status.Connections,
		"closed":       status.Closed,
		"active_count": len(status.Connections),
		"closed_count": len(status.Closed),
		"metadata":     buildResponseMetadata(h.capture, time.Now()),
	})}
}

func (h *ToolHandler) toolGetWebVitals(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	snapshots := h.capture.GetPerformanceSnapshots()
	vitals := buildVitalsMap(snapshots)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Web vitals", map[string]any{
		"metrics":  vitals,
		"metadata": buildResponseMetadata(h.capture, time.Now()),
	})}
}

func buildVitalsMap(snapshots []capture.PerformanceSnapshot) map[string]any {
	if len(snapshots) == 0 {
		return map[string]any{"has_data": false}
	}
	latest := snapshots[len(snapshots)-1]
	vitals := map[string]any{
		"has_data":         true,
		"url":              latest.URL,
		"timestamp":        latest.Timestamp,
		"domContentLoaded": latest.Timing.DomContentLoaded,
		"load":             latest.Timing.Load,
	}
	if latest.Timing.LargestContentfulPaint != nil {
		vitals["lcp"] = *latest.Timing.LargestContentfulPaint
	}
	if latest.Timing.FirstContentfulPaint != nil {
		vitals["fcp"] = *latest.Timing.FirstContentfulPaint
	}
	if latest.CLS != nil {
		vitals["cls"] = *latest.CLS
	}
	return vitals
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	enabled, tabID, trackedURL := h.capture.GetTrackingStatus()
	trackedTitle := h.capture.GetTrackedTabTitle()

	pageURL := h.resolvePageURL(trackedURL)
	pageTitle := h.resolvePageTitle(trackedTitle)

	result := map[string]any{
		"url":      pageURL,
		"title":    pageTitle,
		"tracked":  enabled,
		"metadata": buildResponseMetadata(h.capture, time.Now()),
	}
	if tabID > 0 {
		result["tab_id"] = tabID
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Page info", result)}
}

func (h *ToolHandler) resolvePageURL(trackedURL string) string {
	if trackedURL != "" {
		return trackedURL
	}
	waterfallEntries := h.capture.GetNetworkWaterfallEntries()
	if len(waterfallEntries) > 0 {
		return waterfallEntries[len(waterfallEntries)-1].PageURL
	}
	return ""
}

func (h *ToolHandler) resolvePageTitle(trackedTitle string) string {
	if trackedTitle != "" {
		return trackedTitle
	}
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()
	for i := len(h.server.entries) - 1; i >= 0; i-- {
		if title, ok := h.server.entries[i]["title"].(string); ok && title != "" {
			return title
		}
	}
	return ""
}

func (h *ToolHandler) toolGetTabs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	enabled, tabID, tabURL := h.capture.GetTrackingStatus()

	tabs := []any{}
	if enabled && tabID > 0 {
		tabs = append(tabs, map[string]any{
			"id":      tabID,
			"url":     tabURL,
			"tracked": true,
			"active":  true,
		})
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Tabs", map[string]any{
		"tabs":            tabs,
		"tracking_active": enabled,
		"metadata":        buildResponseMetadata(h.capture, time.Now()),
	})}
}

func (h *ToolHandler) toolRunA11yAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
		Frame        any      `json:"frame"`
	}
	lenientUnmarshal(args, &params)

	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", h.diagnosticHint())}
	}

	result, err := h.executeA11yQuery(params.Scope, params.Tags, params.Frame)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "A11y audit timeout: "+err.Error(), "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", h.diagnosticHint())}
	}

	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")}
	}

	ensureA11ySummary(auditResult)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("A11y audit", auditResult)}
}

func buildA11yQueryParams(scope string, tags []string, frame any) map[string]any {
	queryParams := map[string]any{}
	if scope != "" {
		queryParams["scope"] = scope
	}
	if len(tags) > 0 {
		queryParams["tags"] = tags
	}
	if frame != nil {
		queryParams["frame"] = frame
	}
	return queryParams
}

func (h *ToolHandler) executeA11yQuery(scope string, tags []string, frame any) (json.RawMessage, error) {
	queryParams := buildA11yQueryParams(scope, tags, frame)
	// Error impossible: map contains only primitive types and string slices from input
	paramsJSON, _ := json.Marshal(queryParams)

	queryID := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "a11y",
			Params: paramsJSON,
		},
		30*time.Second,
		"",
	)
	return h.capture.WaitForResult(queryID, 30*time.Second)
}

func ensureA11ySummary(auditResult map[string]any) {
	if _, ok := auditResult["summary"]; ok {
		return
	}
	violations, _ := auditResult["violations"].([]any)
	passes, _ := auditResult["passes"].([]any)
	auditResult["summary"] = map[string]any{
		"violation_count": len(violations),
		"pass_count":      len(passes),
	}
}

func (h *ToolHandler) toolGetScreenshot(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", h.diagnosticHint())}
	}

	queryID := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: json.RawMessage(`{}`),
		},
		20*time.Second, // Sync poll (up to 5s) + capture (1-3s) + upload (0.5-1s) + margin
		"",
	)

	result, err := h.capture.WaitForResult(queryID, 20*time.Second)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtTimeout, "Screenshot capture timeout: "+err.Error(), "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", h.diagnosticHint())}
	}

	var screenshotResult map[string]any
	if err := json.Unmarshal(result, &screenshotResult); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Failed to parse screenshot result: "+err.Error(), "Check extension logs for errors")}
	}

	if errMsg, ok := screenshotResult["error"].(string); ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrExtError, "Screenshot capture failed: "+errMsg, "Check that the tab is visible and accessible. The extension reported an error.", h.diagnosticHint())}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Screenshot captured", screenshotResult)}
}

type timelineEntry struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Summary   string `json:"summary"`
	Data      any    `json:"data,omitempty"`
}

type timelineIncludes struct {
	actions bool
	errors  bool
	network bool
	ws      bool
}

func parseTimelineIncludes(include []string) timelineIncludes {
	if len(include) == 0 {
		return timelineIncludes{actions: true, errors: true, network: true, ws: true}
	}
	var inc timelineIncludes
	for _, v := range include {
		switch v {
		case "actions":
			inc.actions = true
		case "errors":
			inc.errors = true
		case "network":
			inc.network = true
		case "websocket":
			inc.ws = true
		}
	}
	return inc
}

func (h *ToolHandler) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit   int      `json:"limit"`
		Include []string `json:"include"`
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > maxObserveLimit {
		params.Limit = maxObserveLimit
	}

	inc := parseTimelineIncludes(params.Include)
	entries := h.collectTimelineEntries(inc)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if len(entries) > params.Limit {
		entries = entries[:params.Limit]
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Timeline", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": buildResponseMetadata(h.capture, time.Now()),
	})}
}

func (h *ToolHandler) collectTimelineEntries(inc timelineIncludes) []timelineEntry {
	entries := make([]timelineEntry, 0)
	if inc.actions {
		entries = append(entries, h.collectTimelineActions()...)
	}
	if inc.errors {
		entries = append(entries, h.collectTimelineErrors()...)
	}
	if inc.network {
		entries = append(entries, collectTimelineNetwork(h.capture.GetNetworkWaterfallEntries())...)
	}
	if inc.ws {
		entries = append(entries, collectTimelineWebSocket(h.capture.GetAllWebSocketEvents())...)
	}
	return entries
}

func (h *ToolHandler) collectTimelineActions() []timelineEntry {
	actions := h.capture.GetAllEnhancedActions()
	entries := make([]timelineEntry, 0, len(actions))
	for _, a := range actions {
		ts := time.UnixMilli(a.Timestamp).Format(time.RFC3339Nano)
		selector := ""
		if css, ok := a.Selectors["css"].(string); ok {
			selector = css
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "action",
			Summary:   a.Type + " on " + selector,
		})
	}
	return entries
}

func (h *ToolHandler) collectTimelineErrors() []timelineEntry {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	entries := make([]timelineEntry, 0)
	for _, entry := range h.server.entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		ts, _ := entry["timestamp"].(string)
		msg, _ := entry["message"].(string)
		if len(msg) > 80 {
			msg = msg[:80] + "..."
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "error",
			Summary:   msg,
		})
	}
	return entries
}

func collectTimelineNetwork(networkEntries []capture.NetworkWaterfallEntry) []timelineEntry {
	entries := make([]timelineEntry, 0, len(networkEntries))
	for _, n := range networkEntries {
		var ts string
		if !n.Timestamp.IsZero() {
			ts = n.Timestamp.Format(time.RFC3339Nano)
		} else {
			ts = time.Now().Add(-time.Duration(n.StartTime) * time.Millisecond).Format(time.RFC3339Nano)
		}
		entries = append(entries, timelineEntry{
			Timestamp: ts,
			Type:      "network",
			Summary:   n.InitiatorType + " " + n.URL,
		})
	}
	return entries
}

func collectTimelineWebSocket(wsEvents []capture.WebSocketEvent) []timelineEntry {
	entries := make([]timelineEntry, 0, len(wsEvents))
	for _, ws := range wsEvents {
		summary := ws.Event
		if ws.Direction != "" {
			summary += " (" + ws.Direction + ")"
		}
		entries = append(entries, timelineEntry{
			Timestamp: ws.Timestamp,
			Type:      "websocket",
			Summary:   summary,
		})
	}
	return entries
}

type errorCluster struct {
	message    string
	level      string
	count      int
	firstSeen  string
	lastSeen   string
	urls       map[string]bool
	stackTrace string
}

func (h *ToolHandler) toolAnalyzeErrors(req JSONRPCRequest) JSONRPCResponse {
	h.server.mu.RLock()
	clusters := buildErrorClusters(h.server.entries)
	h.server.mu.RUnlock()

	result := clustersToResponse(clusters)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Error clusters", map[string]any{
		"clusters":    result,
		"total_count": len(result),
		"metadata":    buildResponseMetadata(h.capture, time.Now()),
	})}
}

func buildErrorClusters(entries []LogEntry) map[string]*errorCluster {
	clusters := make(map[string]*errorCluster)
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		msg, _ := entry["message"].(string)
		if msg == "" {
			continue
		}

		clusterKey := msg
		if len(clusterKey) > 100 {
			clusterKey = clusterKey[:100]
		}

		timestamp, _ := entry["timestamp"].(string)
		url, _ := entry["url"].(string)
		stack, _ := entry["stackTrace"].(string)

		addToCluster(clusters, clusterKey, msg, level, timestamp, url, stack)
	}
	return clusters
}

func addToCluster(clusters map[string]*errorCluster, key, msg, level, timestamp, url, stack string) {
	if cluster, exists := clusters[key]; exists {
		cluster.count++
		cluster.lastSeen = timestamp
		if url != "" {
			cluster.urls[url] = true
		}
		return
	}
	urls := make(map[string]bool)
	if url != "" {
		urls[url] = true
	}
	clusters[key] = &errorCluster{
		message:    msg,
		level:      level,
		count:      1,
		firstSeen:  timestamp,
		lastSeen:   timestamp,
		urls:       urls,
		stackTrace: stack,
	}
}

func clustersToResponse(clusters map[string]*errorCluster) []map[string]any {
	result := make([]map[string]any, 0, len(clusters))
	for _, c := range clusters {
		urlList := make([]string, 0, len(c.urls))
		for u := range c.urls {
			urlList = append(urlList, u)
		}
		result = append(result, map[string]any{
			"message":     c.message,
			"count":       c.count,
			"first_seen":  c.firstSeen,
			"last_seen":   c.lastSeen,
			"urls":        urlList,
			"stack_trace": c.stackTrace,
		})
	}
	return result
}

type historyEntry struct {
	Timestamp string `json:"timestamp"`
	FromURL   string `json:"from_url,omitempty"`
	ToURL     string `json:"to_url"`
	Type      string `json:"type"`
}

func (h *ToolHandler) toolAnalyzeHistory(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	actions := h.capture.GetAllEnhancedActions()
	entries := buildHistoryEntries(actions)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("History", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": buildResponseMetadata(h.capture, time.Now()),
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

// ============================================
// Async Command Observation Tools
// ============================================

// annotationCommandWaitTimeout is how long observe blocks for pending annotation commands.
// Fits within the bridge's 65s timeout for blocking observe calls.
const annotationCommandWaitTimeout = 55 * time.Second

// toolObserveCommandResult retrieves the result of an async command by correlation_id.
// For annotation commands (ann_*), blocks up to 55s waiting for the user to finish drawing.
func (h *ToolHandler) toolObserveCommandResult(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil && len(args) > 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.CorrelationID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'correlation_id' is missing", "Add the 'correlation_id' parameter and call again", withParam("correlation_id"))}
	}

	corrID := params.CorrelationID

	// Annotation commands: block up to 55s waiting for the user to finish drawing.
	// This is token-efficient — the LLM makes one call and waits instead of rapid polling.
	// See docs/core/async-tool-pattern.md for the full pattern.
	if strings.HasPrefix(corrID, "ann_") {
		cmd, found := h.capture.WaitForCommand(corrID, annotationCommandWaitTimeout)
		if !found {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Annotation command not found: "+corrID, "The command may have expired (10 min TTL). Start a new draw mode session.", h.diagnosticHint())}
		}
		return h.formatCommandResult(req, *cmd, corrID)
	}

	cmd, found := h.capture.GetCommandResult(corrID)
	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Command not found: "+corrID, "The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid. Use observe with what='pending_commands' to see active commands.", h.diagnosticHint())}
	}

	return h.formatCommandResult(req, *cmd, corrID)
}

func (h *ToolHandler) formatCommandResult(req JSONRPCRequest, cmd queries.CommandResult, corrID string) JSONRPCResponse {
	responseData := map[string]any{
		"correlation_id": cmd.CorrelationID,
		"status":         cmd.Status,
		"created_at":     cmd.CreatedAt.Format(time.RFC3339),
	}

	switch cmd.Status {
	case "complete":
		responseData["final"] = true
		return h.formatCompleteCommand(req, cmd, corrID, responseData)
	case "error":
		responseData["final"] = true
		if cmd.Error == "" {
			cmd.Error = "Command failed in extension"
		}
		responseData["error"] = cmd.Error
		if len(cmd.Result) > 0 {
			responseData["result"] = cmd.Result
		}
		summary := fmt.Sprintf("FAILED — Command %s error: %s", corrID, cmd.Error)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONErrorResponse(summary, responseData)}
	case "expired":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrExtTimeout,
			fmt.Sprintf("Command %s expired before the extension could execute it. Error: %s", corrID, cmd.Error),
			"The browser extension may be disconnected or the page is not active. Check observe with what='pilot' to verify extension status, then retry the command.",
			h.diagnosticHint(),
		)}
	case "timeout":
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrExtTimeout,
			fmt.Sprintf("Command %s timed out waiting for the extension to respond. Error: %s", corrID, cmd.Error),
			"The command took too long. The page may be unresponsive or the action is stuck. Try refreshing the page with interact action='refresh', then retry.",
			h.diagnosticHint(),
		)}
	default:
		responseData["final"] = false
		summary := fmt.Sprintf("Command %s: %s", corrID, cmd.Status)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
	}
}

func (h *ToolHandler) formatCompleteCommand(req JSONRPCRequest, cmd queries.CommandResult, corrID string, responseData map[string]any) JSONRPCResponse {
	responseData["result"] = cmd.Result
	responseData["completed_at"] = cmd.CompletedAt.Format(time.RFC3339)
	responseData["timing_ms"] = cmd.CompletedAt.Sub(cmd.CreatedAt).Milliseconds()

	if embeddedErr, hasEmbeddedErr := enrichCommandResponseData(cmd.Result, responseData); cmd.Error == "" && hasEmbeddedErr {
		cmd.Error = embeddedErr
	}

	if beforeSnap, ok := h.capture.GetAndDeleteBeforeSnapshot(corrID); ok {
		if afterSnap, ok := h.capture.GetPerformanceSnapshotByURL(beforeSnap.URL); ok {
			before := performance.SnapshotToPageLoadMetrics(beforeSnap)
			after := performance.SnapshotToPageLoadMetrics(afterSnap)
			responseData["perf_diff"] = performance.ComputePerfDiff(before, after)
		}
	}

	if cmd.Error != "" {
		responseData["error"] = cmd.Error
		summary := fmt.Sprintf("FAILED — Command %s completed with error: %s", corrID, cmd.Error)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONErrorResponse(summary, responseData)}
	}

	summary := fmt.Sprintf("Command %s: complete", corrID)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

func enrichCommandResponseData(result json.RawMessage, responseData map[string]any) (embeddedErr string, hasEmbeddedErr bool) {
	if len(result) == 0 {
		return "", false
	}

	var extResult map[string]any
	if err := json.Unmarshal(result, &extResult); err != nil {
		return "", false
	}

	// Surface extension enrichment fields to top-level for easier LLM consumption.
	for _, key := range []string{"timing", "dom_changes", "analysis", "resolved_tab_id", "resolved_url", "target_context", "effective_tab_id", "effective_url"} {
		if v, ok := extResult[key]; ok {
			responseData[key] = v
		}
	}

	if success, ok := extResult["success"].(bool); ok && !success {
		msg := embeddedCommandError(extResult)
		if msg == "" {
			msg = "Command reported success=false"
		}
		return msg, true
	}

	if _, ok := extResult["error"]; ok {
		msg := embeddedCommandError(extResult)
		if msg != "" {
			return msg, true
		}
	}

	return "", false
}

func embeddedCommandError(extResult map[string]any) string {
	if msg, ok := extResult["error"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := extResult["message"].(string); ok && msg != "" {
		return msg
	}
	return ""
}

// toolObservePendingCommands lists all pending, completed, and failed async commands.
func (h *ToolHandler) toolObservePendingCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	pending := h.capture.GetPendingCommands()
	completed := h.capture.GetCompletedCommands()
	failed := h.capture.GetFailedCommands()

	responseData := map[string]any{
		"pending":   pending,
		"completed": completed,
		"failed":    failed,
	}

	summary := fmt.Sprintf("Pending: %d, Completed: %d, Failed: %d", len(pending), len(completed), len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// toolObserveFailedCommands lists recent failed/expired async commands.
func (h *ToolHandler) toolObserveFailedCommands(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	failed := h.capture.GetFailedCommands()

	responseData := map[string]any{
		"status":   "ok",
		"commands": failed,
		"count":    len(failed),
	}

	if len(failed) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No failed commands found", responseData)}
	}

	summary := fmt.Sprintf("Found %d failed/expired commands", len(failed))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}
