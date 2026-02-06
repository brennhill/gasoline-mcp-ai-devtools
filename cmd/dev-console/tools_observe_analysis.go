// tools_observe_analysis.go — Observe analysis handlers: waterfall, vitals, tabs, a11y, timeline, errors, history, async commands.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/queries"
)

func (h *ToolHandler) toolGetNetworkWaterfall(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit     int    `json:"limit"`
		URLFilter string `json:"url_filter"` // filter by URL substring
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
	}

	// Check if buffer data is stale and fetch fresh if needed
	allEntries := h.capture.GetNetworkWaterfallEntries()
	needsFresh := true
	if len(allEntries) > 0 {
		// Check most recent entry timestamp - consider stale if older than 1 second
		mostRecent := allEntries[len(allEntries)-1].Timestamp
		if time.Since(mostRecent) < 1*time.Second {
			needsFresh = false
		}
	}

	// Fetch fresh data from extension on demand
	if needsFresh {
		queryID := h.capture.CreatePendingQueryWithTimeout(
			queries.PendingQuery{
				Type:   "waterfall",
				Params: json.RawMessage(`{}`),
			},
			5*time.Second,
			"",
		)

		// Wait for extension to respond with fresh data
		result, err := h.capture.WaitForResult(queryID, 5*time.Second)
		if err == nil && result != nil {
			// Parse result and add to buffer
			var waterfallResult struct {
				Entries []capture.NetworkWaterfallEntry `json:"entries"`
				PageURL string                          `json:"pageURL"`
			}
			if err := json.Unmarshal(result, &waterfallResult); err == nil {
				if len(waterfallResult.Entries) > 0 {
					h.capture.AddNetworkWaterfallEntries(waterfallResult.Entries, waterfallResult.PageURL)
					// Re-fetch from buffer to get properly timestamped entries
					allEntries = h.capture.GetNetworkWaterfallEntries()
				}
			}
		}
	}

	// Filter and limit (newest first)
	var entries []map[string]any
	for i := len(allEntries) - 1; i >= 0 && len(entries) < params.Limit; i-- {
		entry := allEntries[i]

		// Filter by URL if specified
		if params.URLFilter != "" {
			if entry.URL == "" || !containsIgnoreCase(entry.URL, params.URLFilter) {
				continue
			}
		}

		entries = append(entries, map[string]any{
			"url":               entry.URL,
			"initiator_type":    entry.InitiatorType,
			"duration_ms":       entry.Duration,
			"start_time":        entry.StartTime,
			"transfer_size":     entry.TransferSize,
			"decoded_body_size": entry.DecodedBodySize,
			"encoded_body_size": entry.EncodedBodySize,
			"timestamp":         entry.Timestamp,
			"page_url":          entry.PageURL,
		})
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network waterfall", map[string]any{"entries": entries, "count": len(entries)})}
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findIgnoreCase(s, substr) >= 0))
}

func findIgnoreCase(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			pc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 'a' - 'A'
			}
			if pc >= 'A' && pc <= 'Z' {
				pc += 'a' - 'A'
			}
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func (h *ToolHandler) toolGetWSStatus(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		URL          string `json:"url"`
		ConnectionID string `json:"connection_id"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &arguments)
	}

	filter := capture.WebSocketStatusFilter{
		URLFilter:    arguments.URL,
		ConnectionID: arguments.ConnectionID,
	}
	status := h.capture.GetWebSocketStatus(filter)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket status", map[string]any{
		"connections":    status.Connections,
		"closed":         status.Closed,
		"active_count":   len(status.Connections),
		"closed_count":   len(status.Closed),
	})}
}

func (h *ToolHandler) toolGetWebVitals(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	snapshots := h.capture.GetPerformanceSnapshots()

	// Extract Core Web Vitals from most recent snapshot
	vitals := map[string]any{
		"has_data": false,
	}

	if len(snapshots) > 0 {
		// Use the most recent snapshot
		latest := snapshots[len(snapshots)-1]
		vitals["has_data"] = true
		vitals["url"] = latest.URL
		vitals["timestamp"] = latest.Timestamp

		// Core Web Vitals
		if latest.Timing.LargestContentfulPaint != nil {
			vitals["lcp"] = *latest.Timing.LargestContentfulPaint
		}
		if latest.Timing.FirstContentfulPaint != nil {
			vitals["fcp"] = *latest.Timing.FirstContentfulPaint
		}
		if latest.CLS != nil {
			vitals["cls"] = *latest.CLS
		}

		// Additional timing metrics
		vitals["dom_content_loaded"] = latest.Timing.DomContentLoaded
		vitals["load"] = latest.Timing.Load
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Web vitals", map[string]any{"metrics": vitals})}
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Get current tracked tab URL and title from extension sync state
	_, _, trackedURL := h.capture.GetTrackingStatus()
	trackedTitle := h.capture.GetTrackedTabTitle()

	var pageURL, pageTitle string

	// Primary source: tracked tab info from extension
	if trackedURL != "" {
		pageURL = trackedURL
	}
	if trackedTitle != "" {
		pageTitle = trackedTitle
	}

	// Fallback for URL: try network waterfall entries
	if pageURL == "" {
		waterfallEntries := h.capture.GetNetworkWaterfallEntries()
		if len(waterfallEntries) > 0 {
			pageURL = waterfallEntries[len(waterfallEntries)-1].PageURL
		}
	}

	// Fallback for title: try recent log entries
	if pageTitle == "" {
		h.server.mu.RLock()
		for i := len(h.server.entries) - 1; i >= 0; i-- {
			entry := h.server.entries[i]
			if title, ok := entry["title"].(string); ok && title != "" {
				pageTitle = title
				break
			}
		}
		h.server.mu.RUnlock()
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Page info", map[string]any{
		"url":   pageURL,
		"title": pageTitle,
	})}
}

func (h *ToolHandler) toolGetTabs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	enabled, tabID, tabURL := h.capture.GetTrackingStatus()

	tabs := []any{}
	if enabled && tabID > 0 {
		tabs = append(tabs, map[string]any{
			"id":       tabID,
			"url":      tabURL,
			"tracked":  true,
			"active":   true,
		})
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Tabs", map[string]any{
		"tabs":            tabs,
		"tracking_active": enabled,
	})}
}

func (h *ToolHandler) toolRunA11yAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Scope        string   `json:"scope"`         // CSS selector to scope audit
		Tags         []string `json:"tags"`          // WCAG tags to test
		ForceRefresh bool     `json:"force_refresh"` // Bypass cache
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	// Check if extension is connected (tab is being tracked)
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "No tab being tracked", "Ensure the Gasoline extension is connected and a tab is being tracked")}
	}

	// Create a11y query with parameters
	queryParams := map[string]any{}
	if params.Scope != "" {
		queryParams["scope"] = params.Scope
	}
	if len(params.Tags) > 0 {
		queryParams["tags"] = params.Tags
	}

	paramsJSON, _ := json.Marshal(queryParams)

	// Create pending query for a11y audit
	queryID := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "a11y",
			Params: paramsJSON,
		},
		30*time.Second, // A11y audits can take time
		"",
	)

	// Wait for extension to respond
	result, err := h.capture.WaitForResult(queryID, 30*time.Second)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "A11y audit timeout: "+err.Error(), "Ensure the extension is connected and the page has axe-core loaded")}
	}

	// Parse and return the result
	var auditResult map[string]any
	if err := json.Unmarshal(result, &auditResult); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Failed to parse a11y result: "+err.Error(), "Check extension logs for errors")}
	}

	// Add summary if not present
	if _, ok := auditResult["summary"]; !ok {
		violations, _ := auditResult["violations"].([]any)
		passes, _ := auditResult["passes"].([]any)
		auditResult["summary"] = map[string]any{
			"violation_count": len(violations),
			"pass_count":      len(passes),
		}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("A11y audit", auditResult)}
}

func (h *ToolHandler) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit   int      `json:"limit"`
		Include []string `json:"include"` // "actions", "errors", "network", "websocket"
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}

	// If no include specified, include all
	includeActions := len(params.Include) == 0
	includeErrors := len(params.Include) == 0
	includeNetwork := len(params.Include) == 0
	includeWS := len(params.Include) == 0
	for _, inc := range params.Include {
		switch inc {
		case "actions":
			includeActions = true
		case "errors":
			includeErrors = true
		case "network":
			includeNetwork = true
		case "websocket":
			includeWS = true
		}
	}

	type timelineEntry struct {
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
		Summary   string `json:"summary"`
		Data      any    `json:"data,omitempty"`
	}

	var entries []timelineEntry

	// Add actions
	if includeActions {
		actions := h.capture.GetAllEnhancedActions()
		for _, a := range actions {
			// Timestamp is unix milliseconds
			ts := time.UnixMilli(a.Timestamp).Format(time.RFC3339Nano)
			// Get selector from Selectors map - prefer css
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
	}

	// Add errors from logs
	if includeErrors {
		h.server.mu.RLock()
		for _, entry := range h.server.entries {
			level, _ := entry["level"].(string)
			if level == "error" {
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
		}
		h.server.mu.RUnlock()
	}

	// Add network events
	if includeNetwork {
		networkEntries := h.capture.GetNetworkWaterfallEntries()
		for _, n := range networkEntries {
			// Use server-side Timestamp if available, otherwise derive from StartTime
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
	}

	// Add websocket events
	if includeWS {
		wsEvents := h.capture.GetAllWebSocketEvents()
		for _, ws := range wsEvents {
			summary := ws.Event
			if ws.Direction != "" {
				summary += " (" + ws.Direction + ")"
			}
			entries = append(entries, timelineEntry{
				Timestamp: ws.Timestamp, // Already a string
				Type:      "websocket",
				Summary:   summary,
			})
		}
	}

	// Sort by timestamp (newest first) — RFC3339 format is lexicographically sortable
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	if len(entries) > params.Limit {
		entries = entries[:params.Limit]
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Timeline", map[string]any{
		"entries": entries,
		"count":   len(entries),
	})}
}

func (h *ToolHandler) toolAnalyzeErrors(req JSONRPCRequest) JSONRPCResponse {
	// Cluster errors from browser logs by similar error messages
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	// Build clusters: message prefix -> occurrences
	type errorCluster struct {
		message    string
		level      string
		count      int
		firstSeen  string
		lastSeen   string
		urls       map[string]bool
		stackTrace string
	}
	clusters := make(map[string]*errorCluster)

	for _, entry := range h.server.entries {
		// Only process error-level logs
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}

		msg, _ := entry["message"].(string)
		if msg == "" {
			continue
		}

		// Normalize message by removing line numbers and dynamic content for clustering
		// Take first 100 chars as cluster key
		clusterKey := msg
		if len(clusterKey) > 100 {
			clusterKey = clusterKey[:100]
		}

		timestamp, _ := entry["timestamp"].(string)
		url, _ := entry["url"].(string)
		stack, _ := entry["stackTrace"].(string)

		if cluster, exists := clusters[clusterKey]; exists {
			cluster.count++
			cluster.lastSeen = timestamp
			if url != "" {
				cluster.urls[url] = true
			}
		} else {
			urls := make(map[string]bool)
			if url != "" {
				urls[url] = true
			}
			clusters[clusterKey] = &errorCluster{
				message:    msg,
				level:      level,
				count:      1,
				firstSeen:  timestamp,
				lastSeen:   timestamp,
				urls:       urls,
				stackTrace: stack,
			}
		}
	}

	// Convert to response format
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

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Error clusters", map[string]any{
		"clusters":    result,
		"total_count": len(result),
	})}
}

func (h *ToolHandler) toolAnalyzeHistory(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Track navigation history from actions (navigate events and URL changes)
	actions := h.capture.GetAllEnhancedActions()

	type historyEntry struct {
		Timestamp string `json:"timestamp"`
		FromURL   string `json:"from_url,omitempty"`
		ToURL     string `json:"to_url"`
		Type      string `json:"type"` // "navigate", "click", etc.
	}

	var entries []historyEntry
	seenURLs := make(map[string]bool)

	for _, a := range actions {
		// Track navigation events
		if a.Type == "navigate" && a.ToURL != "" {
			if !seenURLs[a.ToURL] {
				entries = append(entries, historyEntry{
					Timestamp: time.UnixMilli(a.Timestamp).Format(time.RFC3339),
					FromURL:   a.FromURL,
					ToURL:     a.ToURL,
					Type:      "navigate",
				})
				seenURLs[a.ToURL] = true
			}
		}
		// Also track when URL changes appear in regular actions
		if a.URL != "" && !seenURLs[a.URL] {
			entries = append(entries, historyEntry{
				Timestamp: time.UnixMilli(a.Timestamp).Format(time.RFC3339),
				ToURL:     a.URL,
				Type:      "page_visit",
			})
			seenURLs[a.URL] = true
		}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("History", map[string]any{
		"entries": entries,
		"count":   len(entries),
	})}
}

// ============================================
// Async Command Observation Tools
// ============================================

// toolObserveCommandResult retrieves the result of an async command by correlation_id.
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

	// Query command status by correlation ID
	cmd, found := h.capture.GetCommandResult(params.CorrelationID)
	if !found {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Command not found: "+params.CorrelationID, "The command may have already completed and been cleaned up (60s TTL), or the correlation_id is invalid")}
	}

	responseData := map[string]any{
		"correlation_id": cmd.CorrelationID,
		"status":         cmd.Status,
		"created_at":     cmd.CreatedAt.Format(time.RFC3339),
	}

	if cmd.Status == "complete" {
		responseData["result"] = cmd.Result
		responseData["completed_at"] = cmd.CompletedAt.Format(time.RFC3339)
		if cmd.Error != "" {
			responseData["error"] = cmd.Error
		}
	} else if cmd.Status == "expired" || cmd.Status == "timeout" {
		responseData["error"] = cmd.Error
	}

	summary := fmt.Sprintf("Command %s: %s", params.CorrelationID, cmd.Status)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
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
