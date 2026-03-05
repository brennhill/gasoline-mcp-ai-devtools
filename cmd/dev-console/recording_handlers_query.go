// Purpose: Recording query handlers for observe tool retrieval flows.
// Why: Keeps read-only recording metadata/action retrieval separate from mutation handlers.
// Docs: docs/features/feature/flow-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
)

// toolGetRecordings handles observe(what: "recordings", limit: 10)
func (h *ToolHandler) toolGetRecordings(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit int `json:"limit"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if params.Limit <= 0 {
		params.Limit = 10
	}

	// Load recordings from disk
	recordings, err := h.capture.ListRecordings(params.Limit)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to list recordings: %v", err),
			"Check that recordings directory exists")
	}

	responseData := map[string]any{
		"recordings": recordings,
		"count":      len(recordings),
		"limit":      params.Limit,
	}

	summary := fmt.Sprintf("%d recording(s) found", len(recordings))
	return succeed(req, summary, responseData)
}

// toolGetRecordingActions handles observe(what: "recording_actions", recording_id: "...")
func (h *ToolHandler) toolGetRecordingActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		RecordingID string `json:"recording_id"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	if params.RecordingID == "" {
		return fail(req, ErrMissingParam, "Required parameter 'recording_id' is missing", "Provide the recording_id from a previous event_recording_start call", withParam("recording_id"))
	}

	// Load recording
	recording, err := h.capture.GetRecording(params.RecordingID)
	if err != nil {
		return fail(req, ErrInternal,
			fmt.Sprintf("Failed to load recording: %v", err),
			"Ensure the recording_id is correct")
	}

	responseData := map[string]any{
		"recording_id": params.RecordingID,
		"name":         recording.Name,
		"created_at":   recording.CreatedAt,
		"start_url":    recording.StartURL,
		"duration_ms":  recording.Duration,
		"action_count": recording.ActionCount,
		"actions":      recording.Actions,
	}

	summary := fmt.Sprintf("%d action(s) from recording %s", len(recording.Actions), params.RecordingID)
	return succeed(req, summary, responseData)
}
