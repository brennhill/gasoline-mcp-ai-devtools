// Purpose: Interact record_start/record_stop state-machine helpers.
// Why: Keeps command-result interpretation and state transitions isolated from request handlers.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"
	"strings"
	"time"
)

// extractRecordingLifecycleStatus pulls the extension-reported lifecycle status
// from command result payloads ("recording", "saved", "error", etc.).
func extractRecordingLifecycleStatus(result json.RawMessage) string {
	if len(result) == 0 {
		return ""
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payload.Status))
}

// resolveInteractRecordingState refreshes state using latest command results.
func (h *ToolHandler) resolveInteractRecordingState() interactRecordingState {
	h.recordInteractMu.Lock()
	defer h.recordInteractMu.Unlock()

	state := h.recordInteract
	if state.State == "" {
		state.State = recordingStateIdle
	}

	if state.StopCorrelationID != "" {
		if stopCmd, found := h.capture.GetCommandResult(state.StopCorrelationID); found {
			if stopCmd.Status == "pending" {
				state.State = recordingStateStopping
				state.UpdatedAt = time.Now()
				h.recordInteract = state
				return state
			}
			// Any terminal stop result returns the state machine to idle.
			state = interactRecordingState{State: recordingStateIdle, UpdatedAt: time.Now()}
			h.recordInteract = state
			return state
		}
	}

	if state.StartCorrelationID == "" {
		state.State = recordingStateIdle
		state.UpdatedAt = time.Now()
		h.recordInteract = state
		return state
	}

	startCmd, found := h.capture.GetCommandResult(state.StartCorrelationID)
	if !found {
		// Keep queued state until command result appears.
		if state.State == "" {
			state.State = recordingStateAwaitingGesture
		}
		state.UpdatedAt = time.Now()
		h.recordInteract = state
		return state
	}

	switch startCmd.Status {
	case "pending":
		state.State = recordingStateAwaitingGesture
	case "complete":
		switch extractRecordingLifecycleStatus(startCmd.Result) {
		case recordingStateRecording:
			state.State = recordingStateRecording
		case recordingStateAwaitingGesture:
			state.State = recordingStateAwaitingGesture
		default:
			state = interactRecordingState{State: recordingStateIdle}
		}
	default:
		// error/timeout/expired/cancelled and unknown statuses are terminal.
		state = interactRecordingState{State: recordingStateIdle}
	}

	state.UpdatedAt = time.Now()
	h.recordInteract = state
	return state
}

func (h *ToolHandler) setInteractRecordingStart(correlationID string) {
	h.recordInteractMu.Lock()
	defer h.recordInteractMu.Unlock()
	h.recordInteract = interactRecordingState{
		State:              recordingStateAwaitingGesture,
		StartCorrelationID: correlationID,
		UpdatedAt:          time.Now(),
	}
}

func (h *ToolHandler) setInteractRecordingStopping(correlationID string) {
	h.recordInteractMu.Lock()
	defer h.recordInteractMu.Unlock()
	state := h.recordInteract
	if state.State == "" {
		state.State = recordingStateIdle
	}
	state.State = recordingStateStopping
	state.StopCorrelationID = correlationID
	state.UpdatedAt = time.Now()
	h.recordInteract = state
}
