// Purpose: Interact handler implementations for screen_recording_start and screen_recording_stop actions.
// Why: Separates request validation/queueing from path helpers and state-machine logic.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// queueRecordStart creates the pending query and returns the response for a screen_recording_start action.
func (r *recordingInteractHandler) queueRecordStart(req JSONRPCRequest, fullName, audio, videoPath string, fps, tabID int) JSONRPCResponse {
	correlationID := newCorrelationID("rec")

	extParams := map[string]any{"action": "screen_recording_start", "name": fullName, "fps": fps, "audio": audio}
	// Error impossible: map contains only primitive types from input
	extJSON, _ := json.Marshal(extParams)

	query := queries.PendingQuery{
		Type:          "screen_recording_start",
		Params:        json.RawMessage(extJSON),
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := r.deps.enqueuePendingQuery(req, query, recordStartCommandTimeout); blocked {
		return enqueueResp
	}
	r.setInteractRecordingStart(correlationID)

	r.deps.recordAIAction("screen_recording_start", "", map[string]any{"name": fullName, "fps": fps, "audio": audio})

	return succeed(req, "Recording queued", map[string]any{
		"status":                "queued",
		"recording_state":       recordingStateAwaitingGesture,
		"correlation_id":        correlationID,
		"name":                  fullName,
		"fps":                   fps,
		"audio":                 audio,
		"path":                  videoPath,
		"requires_user_gesture": true,
		"user_prompt":           "Prompt the user to open the Gasoline popup and click Approve to grant recording permission for the target tab.",
		"message":               "Recording start queued. Prompt user to open the Gasoline popup and click Approve, then use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to confirm.",
	})
}

// handleRecordStart processes interact({action: "screen_recording_start"}).
// Generates the filename, forwards to extension via PendingQuery.
// Recording targets the browser, not a specific tab -- no requireTabTracking gate needed.
func (r *recordingInteractHandler) handleRecordStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name  string `json:"name"`
		FPS   int    `json:"fps,omitempty"`
		TabID int    `json:"tab_id,omitempty"`
		Audio string `json:"audio,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := r.deps.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := r.deps.requireExtension(req); blocked {
		return resp
	}

	fps := clampFPS(params.FPS)

	if !validAudioModes[params.Audio] {
		return fail(req, ErrInvalidJSON, "Invalid audio mode: must be 'tab', 'mic', 'both', or omitted", "Use audio: 'tab', 'mic', 'both', or omit for no audio")
	}

	name := params.Name
	if name == "" {
		name = "recording"
	}

	dir, err := recordingsDir()
	if err != nil {
		return fail(req, ErrInternal, err.Error(), "Check disk permissions")
	}

	fullName, videoPath := resolveRecordingPath(dir, name)
	return r.queueRecordStart(req, fullName, params.Audio, videoPath, fps, params.TabID)
}

// handleRecordStop processes interact({action: "screen_recording_stop"}).
// Recording targets the browser, not a specific tab -- no requireTabTracking gate needed.
func (r *recordingInteractHandler) handleRecordStop(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if resp, stop := parseArgs(req, args, &params); stop {
		return resp
	}

	if resp, blocked := r.deps.requirePilot(req); blocked {
		return resp
	}
	if resp, blocked := r.deps.requireExtension(req); blocked {
		return resp
	}

	recordingState := r.resolveInteractRecordingState()
	if recordingState.State != recordingStateRecording {
		retry := "Run interact(action:'screen_recording_start') and wait for observe(what:'command_result') to report status 'recording' before stopping."
		if recordingState.State == recordingStateAwaitingGesture {
			retry = "Recording start is still awaiting user gesture. Ask the user to open the Gasoline popup and click Approve, then retry stop after start reports status 'recording'."
		}
		if recordingState.State == recordingStateStopping {
			retry = "A previous screen_recording_stop is still in progress. Poll observe(what:'command_result') for the stop correlation_id and wait for a terminal status."
		}
		msg := fmt.Sprintf("Cannot stop recording while state is %q", recordingState.State)
		if recordingState.StartCorrelationID == "" {
			msg = "Cannot stop recording: no active interact(screen_recording_start) session found"
		}
		return fail(req, ErrNoData, msg, retry, r.deps.diagnosticHint())
	}

	correlationID := newCorrelationID("recstop")

	extParams := map[string]any{
		"action": "screen_recording_stop",
	}
	// Error impossible: map contains only string values
	extJSON, _ := json.Marshal(extParams)

	query := queries.PendingQuery{
		Type:          "screen_recording_stop",
		Params:        json.RawMessage(extJSON),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	if enqueueResp, blocked := r.deps.enqueuePendingQuery(req, query, recordStopCommandTimeout); blocked {
		return enqueueResp
	}
	r.setInteractRecordingStopping(correlationID)

	r.deps.recordAIAction("screen_recording_stop", "", nil)

	return succeed(req, "Recording stop queued", map[string]any{
		"status":          "queued",
		"recording_state": recordingStateStopping,
		"correlation_id":  correlationID,
		"message":         "Recording stop queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})
}
