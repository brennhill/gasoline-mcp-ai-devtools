// Purpose: Defines the dedicated interact recording sub-handler used for record_start/record_stop actions.
// Why: Reduces ToolHandler god-object surface by isolating recording-specific orchestration/state transitions.
// Docs: docs/features/feature/tab-recording/index.md

package main

type recordingInteractHandler struct {
	parent *ToolHandler
}

func newRecordingInteractHandler(parent *ToolHandler) *recordingInteractHandler {
	return &recordingInteractHandler{parent: parent}
}
