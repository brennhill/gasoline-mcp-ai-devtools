// tools_observe.go — MCP observe tool dispatcher and handlers.
// Handles all observe modes: errors, logs, network, websocket, actions, etc.
package main

import (
	"encoding/json"
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

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

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

// logLevelRank returns the severity rank of a log level (higher = more severe).
func logLevelRank(level string) int {
	switch level {
	case "debug":
		return 0
	case "log":
		return 1
	case "info":
		return 2
	case "warn":
		return 3
	case "error":
		return 4
	default:
		return -1
	}
}

func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit    int    `json:"limit"`
		Level    string `json:"level"`     // exact level match
		MinLevel string `json:"min_level"` // minimum level threshold
		Source   string `json:"source"`    // filter by source
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

		// Filter by exact level if specified
		level, _ := entry["level"].(string)
		if params.Level != "" && level != params.Level {
			continue
		}

		// Filter by minimum level if specified
		if params.MinLevel != "" && logLevelRank(level) < logLevelRank(params.MinLevel) {
			continue
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

// ============================================
// Simple delegator handlers
// ============================================

func (h *ToolHandler) toolGetNetworkBodies(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	bodies := h.capture.GetNetworkBodies()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network bodies", map[string]any{"entries": bodies})}
}

func (h *ToolHandler) toolGetWSEvents(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	events := h.capture.GetAllWebSocketEvents()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("WebSocket events", map[string]any{"entries": events})}
}

func (h *ToolHandler) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	actions := h.capture.GetAllEnhancedActions()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Enhanced actions", map[string]any{"entries": actions})}
}

func (h *ToolHandler) toolObservePilot(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	status := h.capture.GetPilotStatus()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Pilot status", status)}
}

func (h *ToolHandler) toolCheckPerformance(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	snapshots := h.capture.GetPerformanceSnapshots()
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Performance", map[string]any{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})}
}

func (h *ToolHandler) toolGetAPISchema(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API schema", map[string]any{
		"status":  "not_implemented",
		"message": "API schema inference not implemented. Planned for v6.0.",
	})}
}

func (h *ToolHandler) toolGetChangesSince(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Changes since checkpoint", map[string]any{
		"status":  "not_implemented",
		"message": "Change tracking not implemented. Planned for v6.0.",
	})}
}
