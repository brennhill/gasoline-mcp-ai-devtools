// Purpose: Defines the dedicated interact state snapshot sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating state save/load/capture behavior.
// Docs: docs/features/feature/state-time-travel/index.md

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// stateInteractDeps defines the narrow interface that stateInteractHandler needs.
type stateInteractDeps interface {
	requireSessionStore(req JSONRPCRequest) (JSONRPCResponse, bool)
	enqueuePendingQuery(req JSONRPCRequest, query queries.PendingQuery, timeout time.Duration) (JSONRPCResponse, bool)
	recordAIAction(action, url string, extra map[string]any)
	diagnosticHint() func(*StructuredError)
	GetRedactionEngine() RedactionEngine
	GetCapture() *capture.Store
}

type stateInteractHandler struct {
	deps stateInteractDeps

	// Concrete session store injected at construction.
	sessionStoreImpl *persistence.SessionStore
}

func newStateInteractHandler(deps stateInteractDeps, store *persistence.SessionStore) *stateInteractHandler {
	return &stateInteractHandler{
		deps:             deps,
		sessionStoreImpl: store,
	}
}

func (h *ToolHandler) stateInteract() *stateInteractHandler {
	return h.stateInteractHandler
}
