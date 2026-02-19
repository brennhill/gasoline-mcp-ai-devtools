// analysis.go — Observe analysis handlers: waterfall, vitals, tabs, a11y, timeline, errors, history.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package observe

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/queries"
)

// GetNetworkWaterfall returns network waterfall entries from the performance API.
func GetNetworkWaterfall(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit     int    `json:"limit"`
		URLFilter string `json:"url"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allEntries := refreshWaterfallIfStale(deps)
	entries := filterWaterfallEntries(allEntries, params.URLFilter, params.Limit)

	var newestTS time.Time
	if len(allEntries) > 0 {
		newestTS = allEntries[len(allEntries)-1].Timestamp
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network waterfall", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
	})}
}

func refreshWaterfallIfStale(deps Deps) []capture.NetworkWaterfallEntry {
	cap := deps.GetCapture()
	allEntries := cap.GetNetworkWaterfallEntries()
	if len(allEntries) > 0 && time.Since(allEntries[len(allEntries)-1].Timestamp) < 1*time.Second {
		return allEntries
	}

	queryID := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "waterfall",
			Params: json.RawMessage(`{}`),
		},
		5*time.Second,
		"",
	)

	result, err := cap.WaitForResult(queryID, 5*time.Second)
	if err != nil || result == nil {
		return allEntries
	}

	var waterfallResult struct {
		Entries []capture.NetworkWaterfallEntry `json:"entries"`
		PageURL string                          `json:"page_url"`
	}
	if err := json.Unmarshal(result, &waterfallResult); err == nil && len(waterfallResult.Entries) > 0 {
		cap.AddNetworkWaterfallEntries(waterfallResult.Entries, waterfallResult.PageURL)
		return cap.GetNetworkWaterfallEntries()
	}
	return allEntries
}

func filterWaterfallEntries(allEntries []capture.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	entries := make([]map[string]any, 0)
	for i := len(allEntries) - 1; i >= 0 && len(entries) < limit; i-- {
		entry := allEntries[i]
		if urlFilter != "" && (entry.URL == "" || !ContainsIgnoreCase(entry.URL, urlFilter)) {
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

// GetWSStatus returns the current WebSocket connection status.
func GetWSStatus(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var arguments struct {
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Invalid arguments JSON: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	filter := capture.WebSocketStatusFilter{
		URLFilter:    arguments.URL,
		ConnectionID: arguments.ConnectionID,
	}
	status := deps.GetCapture().GetWebSocketStatus(filter)

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket status", map[string]any{
		"connections":  status.Connections,
		"closed":       status.Closed,
		"active_count": len(status.Connections),
		"closed_count": len(status.Closed),
		"metadata":     BuildResponseMetadata(deps.GetCapture(), time.Now()),
	})}
}

// GetWebVitals returns Core Web Vitals metrics from performance snapshots.
func GetWebVitals(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	snapshots := deps.GetCapture().GetPerformanceSnapshots()
	vitals := buildVitalsMap(snapshots)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Web vitals", map[string]any{
		"metrics":  vitals,
		"metadata": BuildResponseMetadata(deps.GetCapture(), time.Now()),
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

// GetPageInfo returns information about the currently tracked page.
func GetPageInfo(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, tabID, trackedURL := cap.GetTrackingStatus()
	trackedTitle := cap.GetTrackedTabTitle()

	pageURL := resolvePageURL(cap, trackedURL)
	pageTitle := resolvePageTitle(deps, trackedTitle)

	result := map[string]any{
		"url":      pageURL,
		"title":    pageTitle,
		"tracked":  enabled,
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}
	if tabID > 0 {
		result["tab_id"] = tabID
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Page info", result)}
}

func resolvePageURL(cap *capture.Capture, trackedURL string) string {
	if trackedURL != "" {
		return trackedURL
	}
	waterfallEntries := cap.GetNetworkWaterfallEntries()
	if len(waterfallEntries) > 0 {
		return waterfallEntries[len(waterfallEntries)-1].PageURL
	}
	return ""
}

func resolvePageTitle(deps Deps, trackedTitle string) string {
	if trackedTitle != "" {
		return trackedTitle
	}
	entries, _ := deps.GetLogEntries()
	for i := len(entries) - 1; i >= 0; i-- {
		if title, ok := entries[i]["title"].(string); ok && title != "" {
			return title
		}
	}
	return ""
}

// GetTabs returns information about tracked browser tabs.
func GetTabs(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, tabID, tabURL := cap.GetTrackingStatus()

	tabs := []any{}
	if enabled && tabID > 0 {
		tabs = append(tabs, map[string]any{
			"id":      tabID,
			"url":     tabURL,
			"tracked": true,
			"active":  true,
		})
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Tabs", map[string]any{
		"tabs":            tabs,
		"tracking_active": enabled,
		"metadata":        BuildResponseMetadata(cap, time.Now()),
	})}
}

// RunA11yAudit executes an accessibility audit via the extension.
func RunA11yAudit(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Selector     string   `json:"selector"`
		Scope        string   `json:"scope"`
		Tags         []string `json:"tags"`
		ForceRefresh bool     `json:"force_refresh"`
		Frame        any      `json:"frame"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Scope == "" && params.Selector != "" {
		params.Scope = params.Selector
	}

	enabled, _, _ := deps.GetCapture().GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrNoData, "No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	result, err := deps.ExecuteA11yQuery(params.Scope, params.Tags, params.Frame, params.ForceRefresh)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtTimeout, "A11y audit timeout: "+err.Error(), "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")}
	}

	ensureA11ySummary(auditResult)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit", auditResult)}
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

// GetScreenshot captures a screenshot of the current page via the extension.
func GetScreenshot(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrNoData, "No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var params struct {
		Format        string `json:"format,omitempty"`
		Quality       int    `json:"quality,omitempty"`
		FullPage      bool   `json:"full_page,omitempty"`
		Selector      string `json:"selector,omitempty"`
		WaitForStable bool   `json:"wait_for_stable,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Format != "" && params.Format != "png" && params.Format != "jpeg" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, "Invalid screenshot format: "+params.Format,
			"Use 'png' or 'jpeg'", mcp.WithParam("format"),
		)}
	}

	if params.Quality != 0 && (params.Quality < 1 || params.Quality > 100) {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, fmt.Sprintf("Invalid quality: %d (must be 1-100)", params.Quality),
			"Use a value between 1 and 100", mcp.WithParam("quality"),
		)}
	}

	screenshotParams := map[string]any{}
	if params.Format != "" {
		screenshotParams["format"] = params.Format
	}
	if params.Quality > 0 {
		screenshotParams["quality"] = params.Quality
	}
	if params.FullPage {
		screenshotParams["full_page"] = true
	}
	if params.Selector != "" {
		screenshotParams["selector"] = params.Selector
	}
	if params.WaitForStable {
		screenshotParams["wait_for_stable"] = true
	}

	queryParams, _ := json.Marshal(screenshotParams)

	queryID := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: queryParams,
		},
		20*time.Second,
		"",
	)

	result, err := cap.WaitForResult(queryID, 20*time.Second)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtTimeout, "Screenshot capture timeout: "+err.Error(), "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var screenshotResult map[string]any
	if err := json.Unmarshal(result, &screenshotResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse screenshot result: "+err.Error(), "Check extension logs for errors")}
	}

	if errMsg, ok := screenshotResult["error"].(string); ok {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtError, "Screenshot capture failed: "+errMsg, "Check that the tab is visible and accessible. The extension reported an error.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Screenshot captured", screenshotResult)}
}

// ============================================
// Timeline
// ============================================

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

// GetSessionTimeline returns a merged, time-sorted timeline of all captured events.
func GetSessionTimeline(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Limit   int      `json:"limit"`
		Include []string `json:"include"`
	}
	mcp.LenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > MaxObserveLimit {
		params.Limit = MaxObserveLimit
	}

	inc := parseTimelineIncludes(params.Include)
	entries := collectTimelineEntries(deps, inc)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if len(entries) > params.Limit {
		entries = entries[:params.Limit]
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Timeline", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": BuildResponseMetadata(deps.GetCapture(), time.Now()),
	})}
}

func collectTimelineEntries(deps Deps, inc timelineIncludes) []timelineEntry {
	cap := deps.GetCapture()
	entries := make([]timelineEntry, 0)
	if inc.actions {
		entries = append(entries, collectTimelineActions(cap)...)
	}
	if inc.errors {
		entries = append(entries, collectTimelineErrors(deps)...)
	}
	if inc.network {
		entries = append(entries, collectTimelineNetwork(cap.GetNetworkWaterfallEntries())...)
	}
	if inc.ws {
		entries = append(entries, collectTimelineWebSocket(cap.GetAllWebSocketEvents())...)
	}
	return entries
}

func collectTimelineActions(cap *capture.Capture) []timelineEntry {
	actions := cap.GetAllEnhancedActions()
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

func collectTimelineErrors(deps Deps) []timelineEntry {
	logEntries, _ := deps.GetLogEntries()
	entries := make([]timelineEntry, 0)
	for _, entry := range logEntries {
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

// ============================================
// Error clustering
// ============================================

type errorCluster struct {
	message    string
	level      string
	count      int
	firstSeen  string
	lastSeen   string
	urls       map[string]bool
	stackTrace string
}

// AnalyzeErrors clusters error entries by message for pattern detection.
func AnalyzeErrors(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	entries, _ := deps.GetLogEntries()
	clusters := buildErrorClusters(entries)
	result := clustersToResponse(clusters)

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Error clusters", map[string]any{
		"clusters":    result,
		"total_count": len(result),
		"metadata":    BuildResponseMetadata(deps.GetCapture(), time.Now()),
	})}
}

func buildErrorClusters(entries []map[string]any) map[string]*errorCluster {
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

// ============================================
// Navigation history
// ============================================

type historyEntry struct {
	Timestamp string `json:"timestamp"`
	FromURL   string `json:"from_url,omitempty"`
	ToURL     string `json:"to_url"`
	Type      string `json:"type"`
}

// AnalyzeHistory extracts navigation history from captured user actions.
func AnalyzeHistory(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	actions := deps.GetCapture().GetAllEnhancedActions()
	entries := buildHistoryEntries(actions)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("History", map[string]any{
		"entries":  entries,
		"count":    len(entries),
		"metadata": BuildResponseMetadata(deps.GetCapture(), time.Now()),
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

