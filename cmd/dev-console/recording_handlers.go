// recording_handlers.go — MCP tool handlers for Flow Recording & Playback feature.
// Handles recording_start, recording_stop, recording_get, and playback actions.
package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// ============================================================================
// Recording Control Handlers
// ============================================================================

// toolConfigureRecordingStart handles configure(action: "recording_start", name: "...", url: "...")
func (h *ToolHandler) toolConfigureRecordingStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name                 string `json:"name"`
		URL                  string `json:"url"`
		SensitiveDataEnabled bool   `json:"sensitive_data_enabled"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	// Validate URL if provided
	if params.URL == "" {
		params.URL = "about:blank"
	}

	// Call capture to start recording
	recordingID, err := h.capture.StartRecording(params.Name, params.URL, params.SensitiveDataEnabled)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to start recording: %v", err),
			"Check storage quota and try again",
		)}
	}

	// Emit log marker
	now := time.Now()
	logEntry := LogEntry{
		"timestamp": now.Format(time.RFC3339Nano),
		"level":     "info",
		"message":   fmt.Sprintf("[RECORDING_START] Recording started: %s", recordingID),
		"category":  "RECORDING",
		"recording_id": recordingID,
		"url":        params.URL,
	}

	// Add to server log entries
	h.MCPHandler.server.mu.Lock()
	h.MCPHandler.server.entries = append(h.MCPHandler.server.entries, logEntry)
	if len(h.MCPHandler.server.entries) > h.MCPHandler.server.maxEntries {
		h.MCPHandler.server.entries = h.MCPHandler.server.entries[1:]
	}
	h.MCPHandler.server.mu.Unlock()

	responseData := map[string]interface{}{
		"status":       "ok",
		"recording_id": recordingID,
		"name":         params.Name,
		"url":          params.URL,
		"message":      fmt.Sprintf("Recording started: %s", recordingID),
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording started", responseData)}
}

// toolConfigureRecordingStop handles configure(action: "recording_stop", recording_id: "...")
func (h *ToolHandler) toolConfigureRecordingStop(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.RecordingID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'recording_id' is missing", "Provide the recording_id from recording_start", withParam("recording_id"))}
	}

	// Call capture to stop recording
	actionCount, duration, err := h.capture.StopRecording(params.RecordingID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to stop recording: %v", err),
			"Ensure the recording_id is valid and the recording is active",
		)}
	}

	// Emit log marker
	now := time.Now()
	logEntry := LogEntry{
		"timestamp": now.Format(time.RFC3339Nano),
		"level":     "info",
		"message":   fmt.Sprintf("[RECORDING_STOP] Recording stopped: %s (%d actions, %dms)", params.RecordingID, actionCount, duration),
		"category":  "RECORDING",
		"recording_id": params.RecordingID,
		"action_count": actionCount,
		"duration_ms":  duration,
	}

	// Add to server log entries
	h.MCPHandler.server.mu.Lock()
	h.MCPHandler.server.entries = append(h.MCPHandler.server.entries, logEntry)
	if len(h.MCPHandler.server.entries) > h.MCPHandler.server.maxEntries {
		h.MCPHandler.server.entries = h.MCPHandler.server.entries[1:]
	}
	h.MCPHandler.server.mu.Unlock()

	responseData := map[string]interface{}{
		"status":        "ok",
		"recording_id":  params.RecordingID,
		"action_count":  actionCount,
		"duration_ms":   duration,
		"message":       fmt.Sprintf("Recording stopped: %d actions captured in %dms", actionCount, duration),
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording stopped", responseData)}
}

// ============================================================================
// Recording Query Handlers (for observe tool)
// ============================================================================

// toolGetRecordings handles observe(what: "recordings", limit: 10)
func (h *ToolHandler) toolGetRecordings(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Load recordings from disk
	recordings, err := h.capture.ListRecordings(params.Limit)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to list recordings: %v", err),
			"Check that recordings directory exists",
		)}
	}

	responseData := map[string]interface{}{
		"recordings": recordings,
		"count":      len(recordings),
		"limit":      params.Limit,
	}

	summary := fmt.Sprintf("%d recording(s) found", len(recordings))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// toolGetRecordingActions handles observe(what: "recording_actions", recording_id: "...")
func (h *ToolHandler) toolGetRecordingActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.RecordingID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'recording_id' is missing", "Provide the recording_id from a previous recording_start call", withParam("recording_id"))}
	}

	// Load recording
	recording, err := h.capture.GetRecording(params.RecordingID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to load recording: %v", err),
			"Ensure the recording_id is correct",
		)}
	}

	responseData := map[string]interface{}{
		"recording_id": params.RecordingID,
		"name":         recording.Name,
		"created_at":   recording.CreatedAt,
		"start_url":    recording.StartURL,
		"duration_ms":  recording.Duration,
		"action_count": recording.ActionCount,
		"actions":      recording.Actions,
	}

	summary := fmt.Sprintf("%d action(s) from recording %s", len(recording.Actions), params.RecordingID)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, responseData)}
}

// ============================================================================
// Playback Handlers
// ============================================================================

// toolConfigurePlayback handles configure(action: "playback", recording_id: "...")
func (h *ToolHandler) toolConfigurePlayback(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.RecordingID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'recording_id' is missing", "Provide a recording_id from a previous recording", withParam("recording_id"))}
	}

	// Execute playback
	session, err := h.capture.ExecutePlayback(params.RecordingID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to execute playback: %v", err),
			"Ensure the recording_id is valid",
		)}
	}

	// Build response
	status := "ok"
	if session.ActionsFailed > 0 {
		status = "partial"
	}

	// Emit log marker
	now := time.Now()
	logEntry := LogEntry{
		"timestamp": now.Format(time.RFC3339Nano),
		"level":     "info",
		"message":   fmt.Sprintf("[PLAYBACK_COMPLETE] Recording replayed: %d/%d actions succeeded", session.ActionsExecuted, session.ActionsExecuted+session.ActionsFailed),
		"category":  "PLAYBACK",
		"recording_id": params.RecordingID,
		"actions_executed": session.ActionsExecuted,
		"actions_failed":   session.ActionsFailed,
	}

	// Add to server log entries
	h.MCPHandler.server.mu.Lock()
	h.MCPHandler.server.entries = append(h.MCPHandler.server.entries, logEntry)
	if len(h.MCPHandler.server.entries) > h.MCPHandler.server.maxEntries {
		h.MCPHandler.server.entries = h.MCPHandler.server.entries[1:]
	}
	h.MCPHandler.server.mu.Unlock()

	responseData := map[string]interface{}{
		"status":            status,
		"recording_id":      params.RecordingID,
		"actions_executed":  session.ActionsExecuted,
		"actions_failed":    session.ActionsFailed,
		"actions_total":     session.ActionsExecuted + session.ActionsFailed,
		"duration_ms":       time.Since(session.StartedAt).Milliseconds(),
		"results_count":     len(session.Results),
		"selector_failures": session.SelectorFailures,
	}

	message := fmt.Sprintf("Playback complete: %d/%d actions executed", session.ActionsExecuted, session.ActionsExecuted+session.ActionsFailed)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(message, responseData)}
}

// toolGetPlaybackResults handles observe(what: "playback_results", recording_id: "...")
func (h *ToolHandler) toolGetPlaybackResults(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.RecordingID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'recording_id' is missing", "Provide the recording_id from playback", withParam("recording_id"))}
	}

	// For now, return a placeholder (would need to store playback sessions)
	responseData := map[string]interface{}{
		"recording_id": params.RecordingID,
		"message":      "Playback results would be stored here for later retrieval",
		"results":      []interface{}{},
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Playback results", responseData)}
}

// ============================================================================
// Log Diffing Handlers
// ============================================================================

// toolConfigureLogDiff handles configure(action: "log_diff", original_id: "...", replay_id: "...")
func (h *ToolHandler) toolConfigureLogDiff(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.OriginalID == "" || params.ReplayID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameters 'original_id' and 'replay_id' are missing", "Provide both recording IDs", withParam("original_id"), withParam("replay_id"))}
	}

	// Compare recordings
	result, err := h.capture.DiffRecordings(params.OriginalID, params.ReplayID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to diff recordings: %v", err),
			"Ensure both recording IDs are valid",
		)}
	}

	// Emit log marker
	now := time.Now()
	logEntry := LogEntry{
		"timestamp": now.Format(time.RFC3339Nano),
		"level":     "info",
		"message":   fmt.Sprintf("[LOG_DIFF] Comparison complete: %s", result.Summary),
		"category":  "LOG_DIFF",
		"original_id": params.OriginalID,
		"replay_id":   params.ReplayID,
		"status":      result.Status,
	}

	// Add to server log entries
	h.MCPHandler.server.mu.Lock()
	h.MCPHandler.server.entries = append(h.MCPHandler.server.entries, logEntry)
	if len(h.MCPHandler.server.entries) > h.MCPHandler.server.maxEntries {
		h.MCPHandler.server.entries = h.MCPHandler.server.entries[1:]
	}
	h.MCPHandler.server.mu.Unlock()

	responseData := map[string]interface{}{
		"status":          result.Status,
		"summary":         result.Summary,
		"original_id":     params.OriginalID,
		"replay_id":       params.ReplayID,
		"new_errors":      len(result.NewErrors),
		"missing_events":  len(result.MissingEvents),
		"changed_values":  len(result.ChangedValues),
		"action_stats":    result.ActionStats,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(result.Summary, responseData)}
}

// toolGetLogDiffReport handles observe(what: "log_diff_report", original_id: "...", replay_id: "...")
func (h *ToolHandler) toolGetLogDiffReport(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		OriginalID string `json:"original_id"`
		ReplayID   string `json:"replay_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.OriginalID == "" || params.ReplayID == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameters 'original_id' and 'replay_id' are missing", "Provide both recording IDs", withParam("original_id"), withParam("replay_id"))}
	}

	// Compare recordings
	result, err := h.capture.DiffRecordings(params.OriginalID, params.ReplayID)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInternal,
			fmt.Sprintf("Failed to generate report: %v", err),
			"Ensure both recording IDs are valid",
		)}
	}

	// Generate report text
	report := result.GetRegressionReport()

	responseData := map[string]interface{}{
		"status":  result.Status,
		"report":  report,
		"summary": result.Summary,
		"stats":   result.ActionStats,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Log diff report", responseData)}
}
