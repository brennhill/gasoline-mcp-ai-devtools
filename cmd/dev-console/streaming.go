// Purpose: Re-exports streaming types/functions and implements the configure_streaming MCP handler for push notifications.
// Why: Bridges internal/streaming into the cmd package while keeping the configure tool dispatch surface unified.
// Docs: docs/features/feature/push-alerts/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/streaming"
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
	NewStreamState        = streaming.NewStreamState
	categoryMatchesEvent  = streaming.CategoryMatchesEvent
	formatMCPNotification = streaming.FormatMCPNotification
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
		if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if params.Action == "" {
		return fail(req, ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"))
	}

	result := h.alertBuffer.Stream.Configure(params.Action, params.Events, params.ThrottleSeconds, params.URLFilter, params.SeverityMin)
	return succeed(req, "Streaming configuration", result)
}
