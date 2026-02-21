// telemetry_passive.go — Passive telemetry collection and integration logic.
// Docs: docs/features/feature/observe/index.md
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const defaultTelemetryClientKey = "_default"

const (
	telemetryModeOff  = "off"
	telemetryModeAuto = "auto"
	telemetryModeFull = "full"
)

type passiveTelemetryCursor struct {
	errorTotal        int64
	networkTotal      int64
	networkErrorTotal int64
	wsTotal           int64
	actionTotal       int64
}

func (h *MCPHandler) maybeAddTelemetrySummary(resp JSONRPCResponse, clientID, toolName, modeOverride string) JSONRPCResponse {
	if h.toolHandler == nil || resp.Result == nil {
		return resp
	}

	summary, changed := h.buildTelemetrySummary(clientID, toolName)
	mode := h.resolveTelemetryMode(modeOverride)
	if mode == telemetryModeOff {
		return resp
	}

	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}
	// Only mutate canonical MCP tool result payloads.
	if len(result.Content) == 0 {
		return resp
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["telemetry_changed"] = changed
	if mode == telemetryModeFull || (mode == telemetryModeAuto && changed) {
		result.Metadata["telemetry_summary"] = summary
	}
	h.addInteractDiagnosticWarning(toolName, &result)

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return resp
	}
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

func (h *MCPHandler) buildTelemetrySummary(clientID, toolName string) (map[string]any, bool) {
	current := h.currentTelemetryCursor()
	deltas := h.telemetryDeltasForClient(clientID, current)
	changed := deltas.errorTotal > 0 ||
		deltas.networkTotal > 0 ||
		deltas.networkErrorTotal > 0 ||
		deltas.wsTotal > 0 ||
		deltas.actionTotal > 0

	summary := map[string]any{
		"new_errors_since_last_call":           deltas.errorTotal,
		"new_network_requests_since_last_call": deltas.networkTotal,
		"new_network_errors_since_last_call":   deltas.networkErrorTotal,
		"new_websocket_events_since_last_call": deltas.wsTotal,
		"new_actions_since_last_call":          deltas.actionTotal,
		"trigger_tool":                         toolName,
		"retrieved_at":                         time.Now().UTC().Format(time.RFC3339),
		"ready_for_interaction":                false,
	}

	cap := h.toolHandler.GetCapture()
	if cap != nil {
		summary["extension_connected"] = cap.IsExtensionConnected()
		enabled, tabID, tabURL := cap.GetTrackingStatus()
		summary["tracking_enabled"] = enabled
		if tabID > 0 {
			summary["tracked_tab_id"] = tabID
		}
		if tabURL != "" {
			summary["tracked_tab_url"] = tabURL
		}
		commandExec := buildCommandExecutionInfo(cap)
		summary["ready_for_interaction"] = commandExec.Ready
		summary["command_execution_status"] = commandExec.Status
	}
	if clientID != "" {
		summary["client_id"] = clientID
	}

	return summary, changed
}

func (h *MCPHandler) currentTelemetryCursor() passiveTelemetryCursor {
	current := passiveTelemetryCursor{}

	if h.server != nil {
		current.errorTotal = h.server.getErrorTotalAdded()
	}

	cap := h.toolHandler.GetCapture()
	if cap == nil {
		return current
	}
	current.networkTotal = cap.GetNetworkTotalAdded()
	current.networkErrorTotal = cap.GetNetworkErrorTotalAdded()
	current.wsTotal = cap.GetWebSocketTotalAdded()
	current.actionTotal = cap.GetActionTotalAdded()
	return current
}

func (h *MCPHandler) telemetryDeltasForClient(clientID string, current passiveTelemetryCursor) passiveTelemetryCursor {
	key := clientID
	if key == "" {
		key = defaultTelemetryClientKey
	}

	h.telemetryMu.Lock()
	defer h.telemetryMu.Unlock()

	if h.telemetryCursors == nil {
		h.telemetryCursors = make(map[string]passiveTelemetryCursor)
	}

	previous, ok := h.telemetryCursors[key]
	h.telemetryCursors[key] = current
	if !ok {
		return passiveTelemetryCursor{}
	}

	return passiveTelemetryCursor{
		errorTotal:        clampDelta(current.errorTotal, previous.errorTotal),
		networkTotal:      clampDelta(current.networkTotal, previous.networkTotal),
		networkErrorTotal: clampDelta(current.networkErrorTotal, previous.networkErrorTotal),
		wsTotal:           clampDelta(current.wsTotal, previous.wsTotal),
		actionTotal:       clampDelta(current.actionTotal, previous.actionTotal),
	}
}

func clampDelta(current, previous int64) int64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

func (h *MCPHandler) addInteractDiagnosticWarning(toolName string, result *MCPToolResult) {
	if toolName != "interact" || result == nil || len(result.Content) == 0 {
		return
	}

	payload, ok := parseJSONPayloadFromContent(result.Content[0].Text)
	if !ok {
		return
	}

	correlationID := strings.TrimSpace(toString(payload["correlation_id"]))
	if correlationID == "" {
		return
	}

	status := strings.ToLower(strings.TrimSpace(toString(payload["status"])))
	final, _ := payload["final"].(bool)
	elapsedMs := toInt64(payload["elapsed_ms"])

	suspiciousFast := final && status == "complete" && elapsedMs >= 0 && elapsedMs < 10 && isSuspiciousInteractCorrelationID(correlationID)
	failed := isInteractFailureResult(result, status)
	if !suspiciousFast && !failed {
		return
	}

	commandExec := buildCommandExecutionInfo(h.toolHandler.GetCapture())
	if suspiciousFast {
		result.Metadata["diagnostic_warning"] = fmt.Sprintf(
			"Command completed in %dms (unusually fast). Doctor status: ready_for_interaction=%t (%s). Run configure(what:'doctor') for details.",
			elapsedMs,
			commandExec.Ready,
			commandExec.Status,
		)
		return
	}

	if status == "" {
		status = "failed"
	}
	result.Metadata["diagnostic_warning"] = fmt.Sprintf(
		"Interact command failed (status=%s). Doctor status: ready_for_interaction=%t (%s). Run configure(what:'doctor') for details.",
		status,
		commandExec.Ready,
		commandExec.Status,
	)
}

func parseJSONPayloadFromContent(text string) (map[string]any, bool) {
	start := strings.Index(text, "{")
	if start < 0 {
		return nil, false
	}
	raw := text[start:]

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func isSuspiciousInteractCorrelationID(correlationID string) bool {
	return strings.HasPrefix(correlationID, "nav_") ||
		strings.HasPrefix(correlationID, "dom_click_") ||
		strings.HasPrefix(correlationID, "dom_type_")
}

func isInteractFailureResult(result *MCPToolResult, status string) bool {
	switch status {
	case "error", "timeout", "expired", "cancelled":
		return true
	}
	return result != nil && result.IsError
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	default:
		return -1
	}
}

func parseTelemetryModeOverride(args json.RawMessage) string {
	if len(args) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(args, &payload); err != nil {
		return ""
	}
	raw, ok := payload["telemetry_mode"].(string)
	if !ok {
		return ""
	}
	mode, ok := normalizeTelemetryMode(raw)
	if !ok {
		return ""
	}
	return mode
}

func normalizeTelemetryMode(mode string) (string, bool) {
	switch mode {
	case telemetryModeOff, telemetryModeAuto, telemetryModeFull:
		return mode, true
	default:
		return "", false
	}
}

func (h *MCPHandler) resolveTelemetryMode(modeOverride string) string {
	if mode, ok := normalizeTelemetryMode(modeOverride); ok {
		return mode
	}
	if h.server != nil {
		mode, ok := normalizeTelemetryMode(h.server.getTelemetryMode())
		if ok {
			return mode
		}
	}
	return telemetryModeAuto
}
