// streaming.go — Type aliases and MCP tool handler for context streaming.
// Pure logic lives in internal/streaming; this file owns the MCP dispatch.
package main

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/streaming"
)

// ============================================
// Type Aliases (backward compatibility)
// ============================================

type StreamConfig = streaming.StreamConfig
type StreamState = streaming.StreamState
type MCPNotification = streaming.MCPNotification
type NotificationParams = streaming.NotificationParams

// ============================================
// Constant Aliases
// ============================================

const (
	defaultThrottleSeconds    = streaming.DefaultThrottleSeconds
	defaultSeverityMin        = streaming.DefaultSeverityMin
	maxNotificationsPerMinute = streaming.MaxNotificationsPerMinute
)

// ============================================
// Function Aliases
// ============================================

var (
	NewStreamState         = streaming.NewStreamState
	categoryMatchesEvent   = streaming.CategoryMatchesEvent
	formatMCPNotification  = streaming.FormatMCPNotification
)

// ============================================
// Tool Handler: configure_streaming
// ============================================

// toolConfigureStreaming handles the configure_streaming MCP tool call.
func (h *ToolHandler) toolConfigureStreaming(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action          string   `json:"action"`
		Events          []string `json:"events"`
		ThrottleSeconds int      `json:"throttle_seconds"`
		URLFilter       string   `json:"url"`
		SeverityMin     string   `json:"severity_min"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"))}
	}

	result := h.alertBuffer.Stream.Configure(params.Action, params.Events, params.ThrottleSeconds, params.URLFilter, params.SeverityMin)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Streaming configuration", result)}
}

