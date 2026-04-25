// Purpose: Validates and executes MCP tools/call, then applies response guards.
// Why: Isolates tool-call lifecycle concerns from transport and generic method dispatch.
//
// Metrics emitted from this file:
//   - telemetry.AppError("tool_rate_limited", …) — fires when a tool call
//     is rejected by the per-tool rate limiter. Classified
//     integration/warning so dashboards can quantify abusive callers
//     separately from organic errors. Lands as `event=app_error,
//     error_code=TOOL_RATE_LIMITED`.
//
// Per-call usage telemetry (tool_call/session_start/first_tool_call) is
// fired downstream in tools_core.go via UsageTracker.RecordToolCall — not
// from this file.
//
// Wire contract: docs/core/app-metrics.md.

package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// handleToolsCall validates tool call payload, executes tool, then applies response guards.
//
// Failure semantics:
// - Invalid JSON args, missing tool handler, unknown tool, and rate-limit breaches are explicit errors.
// - Tool post-processing (redaction/warnings/telemetry) is best-effort and never blocks success path.
func (h *MCPHandler) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: req.ID,
			Error: &JSONRPCError{Code: -32602, Message: "Invalid params: " + err.Error()},
		}
	}

	if h.toolHandler == nil {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}

	h.warnUnknownToolArguments(params.Name, params.Arguments)

	if err := h.checkToolRateLimit(); err != nil {
		telemetry.AppError("tool_rate_limited", nil)
		return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: req.ID, Error: err}
	}

	resp, handled := h.toolHandler.HandleToolCall(req, params.Name, params.Arguments)
	if !handled {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: req.ID,
			Error: &JSONRPCError{Code: -32601, Message: "Unknown tool: " + params.Name},
		}
	}

	telemetryModeOverride := parseTelemetryModeOverride(params.Arguments)
	resp = h.applyToolResponsePostProcessing(resp, req.ClientID, params.Name, telemetryModeOverride)
	return resp
}

// checkToolRateLimit enforces per-process tool call throttling.
//
// Failure semantics:
// - Nil limiter means unlimited mode.
func (h *MCPHandler) checkToolRateLimit() *JSONRPCError {
	limiter := h.toolHandler.GetToolCallLimiter()
	if limiter != nil && !limiter.Allow() {
		return &JSONRPCError{
			Code:    -32603,
			Message: "Tool call rate limit exceeded (500 calls/minute). Please wait before retrying.",
		}
	}
	return nil
}

func (h *MCPHandler) warnUnknownToolArguments(toolName string, args json.RawMessage) {
	if h.server == nil || h.toolHandler == nil || len(args) == 0 {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return
	}
	if len(raw) == 0 {
		return
	}

	allowed := h.allowedToolArgumentKeys(toolName, raw)
	if len(allowed) == 0 {
		return
	}

	unknown := make([]string, 0)
	for k := range raw {
		if _, ok := allowed[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		h.server.AddWarning(fmt.Sprintf("unknown parameter '%s' for tool '%s' (ignored)", k, toolName))
	}
}

func (h *MCPHandler) allowedToolArgumentKeys(toolName string, rawArgs map[string]json.RawMessage) map[string]struct{} {
	tools := h.toolHandler.ToolsList()
	for _, tool := range tools {
		if tool.Name != toolName {
			continue
		}

		keys := make(map[string]struct{})
		props, ok := tool.InputSchema["properties"].(map[string]any)
		if !ok {
			return keys
		}
		for k := range props {
			keys[k] = struct{}{}
		}
		return keys
	}
	return nil
}
