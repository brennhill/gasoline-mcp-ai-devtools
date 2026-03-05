// Purpose: Recording lifecycle control handlers (start/stop).
// Why: Isolates recording-state transitions from query/playback/reporting handlers.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// toolConfigureEventRecordingStart handles configure(action: "event_recording_start", name: "...", url: "...")
func (h *ToolHandler) toolConfigureEventRecordingStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name                 string `json:"name"`
		URL                  string `json:"url"`
		SensitiveDataEnabled bool   `json:"sensitive_data_enabled"`
	}
	if len(args) > 0 {
				if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Validate URL if provided
	if params.URL == "" {
		params.URL = "about:blank"
	}

	// Call capture to start recording
	recordingID, err := h.capture.StartRecording(params.Name, params.URL, params.SensitiveDataEnabled)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to start recording: %v", err),
			"Check storage quota and try again")
	}

	h.appendServerLog(LogEntry{
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"level":        "info",
		"message":      fmt.Sprintf("[RECORDING_START] Recording started: %s", recordingID),
		"category":     "RECORDING",
		"recording_id": recordingID,
		"url":          params.URL,
	})

	responseData := map[string]any{
		"status":       "ok",
		"recording_id": recordingID,
		"name":         params.Name,
		"url":          params.URL,
		"message":      fmt.Sprintf("Recording started: %s", recordingID),
	}

	return succeed(req, "Recording started", responseData)
}

// toolConfigureEventRecordingStop handles configure(action: "event_recording_stop", recording_id: "...")
func (h *ToolHandler) toolConfigureEventRecordingStop(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
				if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if params.RecordingID == "" {
		return fail(req, ErrMissingParam, "Required parameter 'recording_id' is missing", "Provide the recording_id from event_recording_start", withParam("recording_id"))
	}

	// Call capture to stop recording
	actionCount, duration, err := h.capture.StopRecording(params.RecordingID)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to stop recording: %v", err),
			"No active recording with this ID. Start one first: configure({what: 'event_recording_start', name: 'my-recording'})")
	}

	h.appendServerLog(LogEntry{
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"level":        "info",
		"message":      fmt.Sprintf("[RECORDING_STOP] Recording stopped: %s (%d actions, %dms)", params.RecordingID, actionCount, duration),
		"category":     "RECORDING",
		"recording_id": params.RecordingID,
		"action_count": actionCount,
		"duration_ms":  duration,
	})

	responseData := map[string]any{
		"status":       "ok",
		"recording_id": params.RecordingID,
		"action_count": actionCount,
		"duration_ms":  duration,
		"message":      fmt.Sprintf("Recording stopped: %d actions captured in %dms", actionCount, duration),
	}

	return succeed(req, "Recording stopped", responseData)
}
