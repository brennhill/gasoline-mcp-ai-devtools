// Purpose: Re-exports streaming types/functions and implements the configure_streaming MCP handler for push notifications.
// Why: Bridges internal/streaming into the cmd package while keeping the configure tool dispatch surface unified.
// Docs: docs/features/feature/push-alerts/index.md

package main

import (
	"encoding/json"

	cfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/configure"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/streaming"
)

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
// Accepts both action and streaming_action (legacy alias rewritten for backward compatibility).
func (h *ToolHandler) toolConfigureStreaming(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteStreamingArgs(args)
	if err != nil {
		return fail(req, ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")
	}

	var params struct {
		Action          string   `json:"action"`
		Events          []string `json:"events"`
		ThrottleSeconds int      `json:"throttle_seconds"`
		URLFilter       string   `json:"url"`
		SeverityMin     string   `json:"severity_min"`
	}
	if resp, stop := parseArgs(req, rewritten, &params); stop {
		return resp
	}

	if resp, blocked := requireString(req, params.Action, "action", "Add the 'action' parameter and call again"); blocked {
		return resp
	}

	result := h.alertBuffer.Stream.Configure(params.Action, params.Events, params.ThrottleSeconds, params.URLFilter, params.SeverityMin)
	return succeed(req, "Streaming configuration", result)
}
