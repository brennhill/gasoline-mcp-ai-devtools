// Purpose: Defines the dedicated interact upload sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating upload-specific validation/queueing.
// Docs: docs/features/feature/file-upload/index.md

package main

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// uploadDeps defines the narrow interface that uploadInteractHandler needs from its parent.
type uploadDeps interface {
	enqueuePendingQuery(req JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool)
	requirePilot(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool)
	requireExtension(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool)
	requireTabTracking(req JSONRPCRequest, opts ...func(*StructuredError)) (JSONRPCResponse, bool)
	recordAIAction(action, url string, extra map[string]any)
	armEvidenceForCommand(correlationID, action string, args json.RawMessage, clientID string)
}

type uploadInteractHandler struct {
	deps uploadDeps
}

func newUploadInteractHandler(deps uploadDeps) *uploadInteractHandler {
	return &uploadInteractHandler{deps: deps}
}
