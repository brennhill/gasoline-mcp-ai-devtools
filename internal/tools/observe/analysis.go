// Purpose: Provides observe tool implementation helpers for filtering and storage queries.
// Why: Centralizes observe query behavior so evidence filtering stays predictable.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
		Summary   bool   `json:"summary"`
	}
	mcp.LenientUnmarshal(args, &params)
	params.Limit = clampLimit(params.Limit, 100)

	allEntries := refreshWaterfallIfStale(deps)

	var newestTS time.Time
	if len(allEntries) > 0 {
		newestTS = allEntries[len(allEntries)-1].Timestamp
	}

	if params.Summary {
		entries := filterWaterfallSummaryEntries(allEntries, params.URLFilter, params.Limit)
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Network waterfall", map[string]any{
			"entries":  entries,
			"count":    len(entries),
			"metadata": BuildResponseMetadata(deps.GetCapture(), newestTS),
		})}
	}

	entries := filterWaterfallEntries(allEntries, params.URLFilter, params.Limit)
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

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "waterfall",
			Params: json.RawMessage(`{}`),
		},
		5*time.Second,
		"",
	)
	if qerr != nil {
		return allEntries
	}

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

func waterfallSummaryEntry(entry capture.NetworkWaterfallEntry) map[string]any {
	url := entry.URL
	if len(url) > 80 {
		url = url[:80] + "..."
	}
	return map[string]any{"url": url, "ms": entry.Duration, "type": entry.InitiatorType}
}

func filterWaterfallSummaryEntries(allEntries []capture.NetworkWaterfallEntry, urlFilter string, limit int) []map[string]any {
	entries := make([]map[string]any, 0)
	for i := len(allEntries) - 1; i >= 0 && len(entries) < limit; i-- {
		entry := allEntries[i]
		if urlFilter != "" && (entry.URL == "" || !ContainsIgnoreCase(entry.URL, urlFilter)) {
			continue
		}
		entries = append(entries, waterfallSummaryEntry(entry))
	}
	return entries
}

// GetWSStatus returns the current WebSocket connection status.
// TODO(#278): GetWSStatus does not support summary mode -- add for consistency.
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

	response := map[string]any{
		"connections":  status.Connections,
		"closed":       status.Closed,
		"active_count": len(status.Connections),
		"closed_count": len(status.Closed),
		"metadata":     BuildResponseMetadata(deps.GetCapture(), time.Now()),
	}

	if len(status.Connections) == 0 && len(status.Closed) == 0 {
		response["hint"] = wsStatusEmptyHint()
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("WebSocket status", response)}
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
		Summary      bool     `json:"summary"`
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
		// Issue #276: return partial results with error field instead of hard failure.
		// This lets the caller know what happened while providing a usable response shape.
		partialResult := map[string]any{
			"violations":   []any{},
			"passes":       []any{},
			"incomplete":   []any{},
			"inapplicable": []any{},
			"error":        err.Error(),
			"partial":      true,
			"summary": map[string]any{
				"violation_count":    0,
				"pass_count":         0,
				"incomplete_count":   0,
				"inapplicable_count": 0,
			},
		}
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit (partial — "+err.Error()+")", partialResult)}
	}

	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")}
	}

	if params.Summary {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit", buildA11ySummary(auditResult))}
	}

	ensureA11ySummary(auditResult)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("A11y audit", auditResult)}
}

// ensureA11ySummary adds a summary map if the audit result doesn't already have one.
// NOTE: Go uses snake_case (violation_count, pass_count, incomplete_count, inapplicable_count)
// while TS uses bare names (violations, passes, incomplete, inapplicable).
// TODO(#276): Unify summary field naming between Go (violation_count) and TS (violations).
func ensureA11ySummary(auditResult map[string]any) {
	if _, ok := auditResult["summary"]; ok {
		return
	}
	violations, _ := auditResult["violations"].([]any)
	passes, _ := auditResult["passes"].([]any)
	incomplete, _ := auditResult["incomplete"].([]any)
	inapplicable, _ := auditResult["inapplicable"].([]any)
	auditResult["summary"] = map[string]any{
		"violation_count":    len(violations),
		"pass_count":         len(passes),
		"incomplete_count":   len(incomplete),
		"inapplicable_count": len(inapplicable),
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

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: queryParams,
		},
		20*time.Second,
		"",
	)
	if qerr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrQueueFull, "Command queue full: "+qerr.Error(), "Wait for in-flight commands to complete, then retry.")}
	}

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

	// Extract data_url before building text block to avoid duplicating
	// the large base64 payload in both text and image content blocks.
	var dataURL string
	if du, ok := screenshotResult["data_url"].(string); ok && du != "" {
		dataURL = du
		delete(screenshotResult, "data_url")
	}

	// Build text response with file path info (backward compatible)
	resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Screenshot captured", screenshotResult)}

	// Append inline image content block if data_url was present
	if dataURL != "" {
		base64Data, mimeType := parseDataURL(dataURL)
		if base64Data != "" {
			resp = mcp.AppendImageToResponse(resp, base64Data, mimeType)
		}
	}

	return resp
}

// parseDataURL extracts the base64 data and MIME type from a data URL.
// Example: "data:image/jpeg;base64,/9j/4AAQ..." -> ("/9j/4AAQ...", "image/jpeg")
// Returns empty strings if the data URL format is invalid.
func parseDataURL(dataURL string) (base64Data, mimeType string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", ""
	}
	// Format: data:<mimeType>;base64,<data>
	rest := dataURL[5:] // strip "data:"
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return "", ""
	}
	mimeType = rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return "", ""
	}
	base64Data = rest[7:] // strip "base64,"
	return base64Data, mimeType
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

// buildA11ySummary creates a compact representation of an a11y audit result.
func buildA11ySummary(auditResult map[string]any) map[string]any {
	passes, _ := auditResult["passes"].([]any)
	violations, _ := auditResult["violations"].([]any)
	incomplete, _ := auditResult["incomplete"].([]any)

	type issueInfo struct {
		rule     string
		severity string
		count    int
	}
	issues := make([]issueInfo, 0, len(violations))
	for _, v := range violations {
		vMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		rule, _ := vMap["id"].(string)
		impact, _ := vMap["impact"].(string)
		nodes, _ := vMap["nodes"].([]any)
		issues = append(issues, issueInfo{rule: rule, severity: impact, count: len(nodes)})
	}
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].count > issues[j].count
	})
	topN := 5
	if len(issues) < topN {
		topN = len(issues)
	}
	topIssues := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topIssues[i] = map[string]any{
			"rule":     issues[i].rule,
			"count":    issues[i].count,
			"severity": issues[i].severity,
		}
	}

	return map[string]any{
		"pass":       len(passes),
		"violations": len(violations),
		"incomplete": len(incomplete),
		"top_issues": topIssues,
	}
}
