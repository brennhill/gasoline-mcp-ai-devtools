// tools_observe.go — MCP observe tool dispatcher and handlers.
// Handles all observe modes: errors, logs, network, websocket, actions, etc.
package main

import (
	"encoding/json"
	"sort"
	"strings"
)

// ObserveHandler is the function signature for observe tool handlers.
type ObserveHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// observeHandlers maps observe mode names to their handler functions.
var observeHandlers = map[string]ObserveHandler{
	"errors":            func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetBrowserErrors(req, args) },
	"logs":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetBrowserLogs(req, args) },
	"extension_logs":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetExtensionLogs(req, args) },
	"network_waterfall": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetNetworkWaterfall(req, args) },
	"network_bodies":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetNetworkBodies(req, args) },
	"websocket_events":  func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetWSEvents(req, args) },
	"websocket_status":  func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetWSStatus(req, args) },
	"actions":           func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetEnhancedActions(req, args) },
	"vitals":            func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetWebVitals(req, args) },
	"page":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetPageInfo(req, args) },
	"tabs":              func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetTabs(req, args) },
	"pilot":             func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolObservePilot(req, args) },
	"performance":       func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolCheckPerformance(req, args) },
	"api":               func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetAPISchema(req, args) },
	"accessibility":     func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolRunA11yAudit(req, args) },
	"changes":           func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetChangesSince(req, args) },
	"timeline":          func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetSessionTimeline(req, args) },
	"error_clusters":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeErrors(req) },
	"error_bundles":     func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetErrorBundles(req, args) },
	"history":           func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAnalyzeHistory(req, args) },
	"security_audit":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolSecurityAudit(req, args) },
	"third_party_audit": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolAuditThirdParties(req, args) },
	"security_diff":     func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolDiffSecurity(req, args) },
	"screenshot":        func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetScreenshot(req, args) },
	"command_result":    func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolObserveCommandResult(req, args) },
	"pending_commands":  func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolObservePendingCommands(req, args) },
	"failed_commands":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolObserveFailedCommands(req, args) },
	"recordings":        func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetRecordings(req, args) },
	"recording_actions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetRecordingActions(req, args) },
	"playback_results":  func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetPlaybackResults(req, args) },
	"log_diff_report":   func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse { return h.toolGetLogDiffReport(req, args) },
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
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
	}

	// Read entries from server and filter for errors
	h.server.mu.RLock()
	errors := make([]map[string]any, 0)
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

// Note: logLevelRank has been moved to observe_filtering.go

func (h *ToolHandler) toolGetBrowserLogs(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Parse optional parameters
	var params struct {
		Limit    int    `json:"limit"`
		Level    string `json:"level"`     // exact level match
		MinLevel string `json:"min_level"` // minimum level threshold
		Source   string `json:"source"`    // filter by source
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
	}

	// Read entries from server with optional filtering
	h.server.mu.RLock()
	logs := make([]map[string]any, 0)
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
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 100 // default limit
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
