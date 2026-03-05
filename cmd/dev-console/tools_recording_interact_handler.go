// Purpose: Defines the dedicated interact recording sub-handler used for record_start/record_stop actions.
// Why: Reduces ToolHandler god-object surface by isolating recording-specific orchestration/state transitions.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// recordingDeps defines the narrow interface that recordingInteractHandler needs from its parent.
type recordingDeps interface {
	enqueuePendingQuery(req JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool)
	requirePilot(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool)
	requireExtension(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool)
	recordAIAction(action, url string, extra map[string]any)
	diagnosticHint() func(*StructuredError)
	getCommandResult(correlationID string) (*queries.CommandResult, bool)
}

type recordingInteractHandler struct {
	deps recordingDeps

	// Interact recording state gate (record_start/record_stop sequencing).
	// Relocated from ToolHandler — exclusively owned by recordingInteractHandler.
	recordInteractMu sync.Mutex
	recordInteract   interactRecordingState
}

func newRecordingInteractHandler(deps recordingDeps) *recordingInteractHandler {
	return &recordingInteractHandler{deps: deps}
}
