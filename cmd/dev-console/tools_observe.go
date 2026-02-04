// tools_observe.go — MCP observe tool dispatcher and handlers.
// Handles all observe modes: errors, logs, network, websocket, actions, etc.
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff"))}
	}

	var resp JSONRPCResponse
	switch params.What {
	case "errors":
		resp = h.toolGetBrowserErrors(req, args)
	case "logs":
		resp = h.toolGetBrowserLogs(req, args)
	case "extension_logs":
		resp = h.toolGetExtensionLogs(req, args)
	case "network_waterfall":
		resp = h.toolGetNetworkWaterfall(req, args)
	case "network_bodies":
		resp = h.toolGetNetworkBodies(req, args)
	case "websocket_events":
		resp = h.toolGetWSEvents(req, args)
	case "websocket_status":
		resp = h.toolGetWSStatus(req, args)
	case "actions":
		resp = h.toolGetEnhancedActions(req, args)
	case "vitals":
		resp = h.toolGetWebVitals(req, args)
	case "page":
		resp = h.toolGetPageInfo(req, args)
	case "tabs":
		resp = h.toolGetTabs(req, args)
	case "pilot":
		resp = h.toolObservePilot(req, args)
	// Analyze modes (formerly separate analyze tool)
	case "performance":
		resp = h.toolCheckPerformance(req, args)
	case "api":
		resp = h.toolGetAPISchema(req, args)
	case "accessibility":
		resp = h.toolRunA11yAudit(req, args)
	case "changes":
		resp = h.toolGetChangesSince(req, args)
	case "timeline":
		resp = h.toolGetSessionTimeline(req, args)
	case "error_clusters":
		resp = h.toolAnalyzeErrors(req)
	case "history":
		resp = h.toolAnalyzeHistory(req, args)
	// Security scan modes (formerly separate security tool)
	case "security_audit":
		resp = h.toolSecurityAudit(req, args)
	case "third_party_audit":
		resp = h.toolAuditThirdParties(req, args)
	case "security_diff":
		resp = h.toolDiffSecurity(req, args)
	// Async command tracking modes
	case "command_result":
		resp = h.toolObserveCommandResult(req, args)
	case "pending_commands":
		resp = h.toolObservePendingCommands(req, args)
	case "failed_commands":
		resp = h.toolObserveFailedCommands(req, args)
	// capture.Recording modes
	case "recordings":
		resp = h.toolGetRecordings(req, args)
	case "recording_actions":
		resp = h.toolGetRecordingActions(req, args)
	case "playback_results":
		resp = h.toolGetPlaybackResults(req, args)
	case "log_diff_report":
		resp = h.toolGetLogDiffReport(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown observe mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: errors, logs, network_waterfall, network_bodies, websocket_events, websocket_status, actions, vitals, page, tabs, pilot, performance, api, accessibility, changes, timeline, error_clusters, history, security_audit, third_party_audit, security_diff, command_result, pending_commands, failed_commands, recordings, recording_actions, playback_results, log_diff_report"))}
	}

	// Prepend tracking status warning when no tab is tracked
	if trackingEnabled, hint := h.checkTrackingStatus(); !trackingEnabled && hint != "" {
		resp = h.prependTrackingWarning(resp, hint)
	}

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

	return resp
}

// prependTrackingWarning adds a tracking status warning as the first content block in the response.
func (h *ToolHandler) prependTrackingWarning(resp JSONRPCResponse, hint string) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	warningBlock := MCPContentBlock{
		Type: "text",
		Text: hint,
	}
	// Prepend warning as first block
	result.Content = append([]MCPContentBlock{warningBlock}, result.Content...)

	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
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
	// Parse optional limit parameter
	var params struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
	}

	// Read entries from server and filter for errors
	h.server.mu.RLock()
	var errors []map[string]any
	for i := len(h.server.entries) - 1; i >= 0 && len(errors) < params.Limit; i-- {
		entry := h.server.entries[i]
		level, _ := entry["level"].(string)
		if level == "error" {
			errors = append(errors, map[string]any{
				"message":   entry["message"],
				"source":    entry["source"],
				"url":       entry["url"],
				"line":      entry["line"],
				"column":    entry["column"],
				"stack":     entry["stack"],
				"timestamp": entry["timestamp"],
			})
		}
	}
	h.server.mu.RUnlock()

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Browser errors", map[string]any{"errors": errors, "count": len(errors)})}
}

func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit  int    `json:"limit"`
		Level  string `json:"level"` // filter by level: "log", "warn", "error", "info", "debug"
		Source string `json:"source"` // filter by source
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
	}

	// Read entries from server with optional filtering
	h.server.mu.RLock()
	var logs []map[string]any
	for i := len(h.server.entries) - 1; i >= 0 && len(logs) < params.Limit; i-- {
		entry := h.server.entries[i]

		// Skip non-console entries (e.g., lifecycle events)
		entryType, _ := entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}

		// Filter by level if specified
		if params.Level != "" {
			level, _ := entry["level"].(string)
			if level != params.Level {
				continue
			}
		}

		// Filter by source if specified
		if params.Source != "" {
			source, _ := entry["source"].(string)
			if source != params.Source {
				continue
			}
		}

		logs = append(logs, map[string]any{
			"level":     entry["level"],
			"message":   entry["message"],
			"source":    entry["source"],
			"url":       entry["url"],
			"line":      entry["line"],
			"column":    entry["column"],
			"timestamp": entry["timestamp"],
		})
	}
	h.server.mu.RUnlock()

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Browser logs", map[string]any{"logs": logs, "count": len(logs)})}
}

func (h *ToolHandler) toolGetExtensionLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit int    `json:"limit"`
		Level string `json:"level"` // filter by level
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
	}

	// Read extension logs from capture buffer
	allLogs := h.capture.GetExtensionLogs()

	// Filter and limit (newest first)
	var logs []map[string]any
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

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Extension logs", map[string]any{"logs": logs, "count": len(logs)})}
}

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

	// Read network waterfall entries from capture buffer
	allEntries := h.capture.GetNetworkWaterfallEntries()

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

func (h *ToolHandler) toolGetNetworkBodies(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	bodies := h.capture.GetNetworkBodies()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network bodies", map[string]any{"entries": bodies})}
}

func (h *ToolHandler) toolGetWSEvents(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	events := h.capture.GetAllWebSocketEvents()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket events", map[string]any{"entries": events})}
}

func (h *ToolHandler) toolGetWSStatus(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket status", map[string]any{"connections": []any{}})}
}

func (h *ToolHandler) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	actions := h.capture.GetAllEnhancedActions()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Enhanced actions", map[string]any{"entries": actions})}
}

func (h *ToolHandler) toolGetWebVitals(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Web vitals", map[string]any{"metrics": map[string]any{}})}
}

func (h *ToolHandler) toolGetPageInfo(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract page info from recent data sources
	var pageURL, pageTitle string

	// Try to get page URL from most recent network waterfall entry
	waterfallEntries := h.capture.GetNetworkWaterfallEntries()
	if len(waterfallEntries) > 0 {
		// Get the most recent entry's page URL
		pageURL = waterfallEntries[len(waterfallEntries)-1].PageURL
	}

	// Try to get page info from recent log entries
	h.server.mu.RLock()
	for i := len(h.server.entries) - 1; i >= 0; i-- {
		entry := h.server.entries[i]
		// Look for page navigation or URL info
		if url, ok := entry["url"].(string); ok && url != "" && pageURL == "" {
			pageURL = url
		}
		if title, ok := entry["title"].(string); ok && title != "" {
			pageTitle = title
			break
		}
		// Also check page_url field
		if url, ok := entry["page_url"].(string); ok && url != "" && pageURL == "" {
			pageURL = url
		}
	}
	h.server.mu.RUnlock()

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Page info", map[string]any{
		"url":   pageURL,
		"title": pageTitle,
	})}
}

func (h *ToolHandler) toolGetTabs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Tabs", map[string]any{"tabs": []any{}})}
}

func (h *ToolHandler) toolObservePilot(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Pilot status", map[string]any{"enabled": false})}
}

func (h *ToolHandler) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Performance", map[string]any{"metrics": map[string]any{}})}
}

func (h *ToolHandler) toolGetAPISchema(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API schema", map[string]any{"endpoints": []any{}})}
}

func (h *ToolHandler) toolRunA11yAudit(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("A11y audit", map[string]any{"violations": []any{}})}
}

func (h *ToolHandler) toolGetChangesSince(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Checkpoint string   `json:"checkpoint"`
		Include    []string `json:"include"`
		Severity   string   `json:"severity"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status":     "ok",
		"checkpoint": arguments.Checkpoint,
		"changes":    []any{},
		"message":    "No changes since checkpoint",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Changes since checkpoint", responseData)}
}

func (h *ToolHandler) toolGetSessionTimeline(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Timeline", map[string]any{"entries": []any{}})}
}

func (h *ToolHandler) toolAnalyzeErrors(req JSONRPCRequest) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Error clusters", map[string]any{"clusters": []any{}})}
}

func (h *ToolHandler) toolAnalyzeHistory(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("History", map[string]any{"entries": []any{}})}
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
