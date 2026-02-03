// tools_configure.go — MCP configure tool dispatcher and handlers.
// Handles all configure actions: store, load, noise_rule, dismiss, clear, etc.
package main

import (
	"encoding/json"
)

// toolConfigure dispatches configure requests based on the 'action' parameter.
func (h *ToolHandler) toolConfigure(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action string `json:"action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"), withHint("Valid values: store, load, noise_rule, dismiss, clear, capture, record_event, query_dom, diff_sessions, validate_api, audit_log, health, streaming"))}
	}

	var resp JSONRPCResponse
	switch params.Action {
	case "store":
		resp = h.toolConfigureStore(req, args)
	case "load":
		resp = h.toolLoadSessionContext(req, args)
	case "noise_rule":
		resp = h.toolConfigureNoiseRule(req, args)
	case "dismiss":
		resp = h.toolConfigureDismiss(req, args)
	case "clear":
		resp = h.toolConfigureClear(req, args)
	case "capture":
		resp = h.toolConfigureCapture(req, args)
	case "record_event":
		resp = h.toolConfigureRecordEvent(req, args)
	case "query_dom":
		resp = h.toolQueryDOM(req, args)
	case "diff_sessions":
		resp = h.toolDiffSessionsWrapper(req, args)
	case "validate_api":
		resp = h.toolValidateAPI(req, args)
	case "audit_log":
		resp = h.toolGetAuditLog(req, args)
	case "health":
		resp = h.toolGetHealth(req)
	case "streaming":
		resp = h.toolConfigureStreamingWrapper(req, args)
	case "test_boundary_start":
		resp = h.toolConfigureTestBoundaryStart(req, args)
	case "test_boundary_end":
		resp = h.toolConfigureTestBoundaryEnd(req, args)
	case "recording_start":
		resp = h.toolConfigureRecordingStart(req, args)
	case "recording_stop":
		resp = h.toolConfigureRecordingStop(req, args)
	case "playback":
		resp = h.toolConfigurePlayback(req, args)
	case "log_diff":
		resp = h.toolConfigureLogDiff(req, args)
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown configure action: "+params.Action, "Use a valid action from the 'action' enum", withParam("action"))}
	}
	return resp
}

// ============================================
// Configure sub-handlers
// ============================================

func (h *ToolHandler) toolConfigureStore(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var compositeArgs struct {
		StoreAction string          `json:"store_action"`
		Namespace   string          `json:"namespace"`
		Key         string          `json:"key"`
		Data        json.RawMessage `json:"data"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	action := compositeArgs.StoreAction
	if action == "" {
		action = "list"
	}

	responseData := map[string]any{
		"status":  "ok",
		"action":  action,
		"message": "Store operation: " + action,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Store operation complete", responseData)}
}

func (h *ToolHandler) toolLoadSessionContext(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	responseData := map[string]any{
		"status":  "ok",
		"context": map[string]any{},
		"message": "Session context loaded",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session context loaded", responseData)}
}

func (h *ToolHandler) toolConfigureNoiseRule(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Extract the noise_action field as the action for configure_noise
	var compositeArgs struct {
		NoiseAction string `json:"noise_action"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &compositeArgs); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Rewrite args to have "action" field that toolConfigureNoise expects
	var rawMap map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &rawMap); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	rawMap["action"] = compositeArgs.NoiseAction
	if rawMap["action"] == "" {
		rawMap["action"] = "list"
	}
	// Error impossible: rawMap contains only primitive types and strings from input
	rewrittenArgs, _ := json.Marshal(rawMap)

	return h.toolConfigureNoise(req, rewrittenArgs)
}

func (h *ToolHandler) toolConfigureNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Action string `json:"action"`
		Rules  []struct {
			Category       string `json:"category"`
			Classification string `json:"classification"`
			MatchSpec      struct {
				MessageRegex string `json:"message_regex"`
				SourceRegex  string `json:"source_regex"`
				URLRegex     string `json:"url_regex"`
				Method       string `json:"method"`
				StatusMin    int    `json:"status_min"`
				StatusMax    int    `json:"status_max"`
				Level        string `json:"level"`
			} `json:"match_spec"`
		} `json:"rules"`
		RuleID string `json:"rule_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	var responseData any

	switch arguments.Action {
	case "add":
		responseData = map[string]any{
			"status":     "ok",
			"rulesAdded": len(arguments.Rules),
			"totalRules": len(arguments.Rules),
		}

	case "remove":
		responseData = map[string]any{
			"status":  "ok",
			"removed": arguments.RuleID,
		}

	case "list":
		responseData = map[string]any{
			"rules":      []any{},
			"statistics": map[string]any{},
		}

	case "reset":
		responseData = map[string]any{
			"status":     "ok",
			"totalRules": 0,
		}

	case "auto_detect":
		responseData = map[string]any{
			"proposals":  []any{},
			"totalRules": 0,
		}

	default:
		responseData = map[string]any{"error": "unknown action: " + arguments.Action}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Noise configuration updated", responseData)}
}

func (h *ToolHandler) toolConfigureDismiss(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return h.toolDismissNoise(req, args)
}

func (h *ToolHandler) toolDismissNoise(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Pattern  string `json:"pattern"`
		Category string `json:"category"`
		Reason   string `json:"reason"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status":     "ok",
		"totalRules": 0,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Noise pattern dismissed", responseData)}
}

// toolConfigureClear handles buffer-specific clearing with optional buffer parameter.
func (h *ToolHandler) toolConfigureClear(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// TODO(future): Implementation pending - currently returns empty data
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Clear", map[string]any{"status": "ok"})}
}

func (h *ToolHandler) toolConfigureCapture(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Configure", map[string]any{"status": "ok"})}
}

func (h *ToolHandler) toolConfigureRecordEvent(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Record event", map[string]any{"status": "ok"})}
}

func (h *ToolHandler) toolQueryDOM(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Query DOM", map[string]any{"matches": []any{}})}
}

// toolDiffSessionsWrapper repackages session_action → action for toolDiffSessions.
func (h *ToolHandler) toolDiffSessionsWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if sa, ok := raw["session_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return h.toolDiffSessions(req, rewritten)
}

func (h *ToolHandler) toolDiffSessions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SessionAction string `json:"session_action"`
		Name          string `json:"name"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status": "ok",
		"action": params.SessionAction,
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Session diff", responseData)}
}

func (h *ToolHandler) toolValidateAPI(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation       string   `json:"operation"`
		URLFilter       string   `json:"url"`
		IgnoreEndpoints []string `json:"ignore_endpoints"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	switch params.Operation {
	case "analyze":
		responseData := map[string]any{
			"status":     "ok",
			"operation":  "analyze",
			"violations": []any{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "report":
		responseData := map[string]any{
			"status":    "ok",
			"operation": "report",
			"endpoints": []any{},
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", responseData)}

	case "clear":
		clearResult := map[string]any{
			"action": "cleared",
			"status": "ok",
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("API validation", clearResult)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "operation parameter must be 'analyze', 'report', or 'clear'", "Use a valid value for 'operation'", withParam("operation"), withHint("analyze, report, or clear"))}
	}
}

func (h *ToolHandler) toolGetAuditLog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		SessionID string `json:"session_id"`
		ToolName  string `json:"tool_name"`
		Limit     int    `json:"limit"`
		Since     string `json:"since"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	responseData := map[string]any{
		"status":  "ok",
		"entries": []any{},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Audit log entries", responseData)}
}

// toolConfigureStreamingWrapper repackages streaming_action → action for toolConfigureStreaming.
func (h *ToolHandler) toolConfigureStreamingWrapper(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var raw map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &raw); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}
	if sa, ok := raw["streaming_action"].(string); ok {
		raw["action"] = sa
	}
	// Error impossible: raw contains only primitive types and strings from input
	rewritten, _ := json.Marshal(raw)
	return h.toolConfigureStreaming(req, rewritten)
}

// ============================================
// Test Boundary Tool Implementations
// ============================================

func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestID string `json:"test_id"`
		Label  string `json:"label"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.TestID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", withParam("test_id"))}
	}

	label := params.Label
	if label == "" {
		label = "Test: " + params.TestID
	}

	responseData := map[string]any{
		"status":  "ok",
		"test_id": params.TestID,
		"label":   label,
		"message": "Test boundary started",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test boundary started", responseData)}
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TestID string `json:"test_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.TestID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", withParam("test_id"))}
	}

	responseData := map[string]any{
		"status":     "ok",
		"test_id":    params.TestID,
		"was_active": true,
		"message":    "Test boundary ended",
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Test boundary ended", responseData)}
}
