// Purpose: Defines the dedicated configure session/store sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating session-oriented configure flows.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/persistence"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/session"
)

// configureSessionDeps defines the narrow interface that configureSessionHandler needs.
type configureSessionDeps interface {
	requireSessionStore(req JSONRPCRequest) (JSONRPCResponse, bool)
	invalidateSummaryPref()
	SetActiveCodebase(path string)
	ClearLogEntries()
	LogEntryCount() int
}

type configureSessionHandler struct {
	deps configureSessionDeps

	// Concrete implementations injected at construction.
	sessionStoreImpl *persistence.SessionStore
	sessionManager   *session.Manager
}

func newConfigureSessionHandler(deps configureSessionDeps, store *persistence.SessionStore, mgr *session.Manager) *configureSessionHandler {
	return &configureSessionHandler{
		deps:             deps,
		sessionStoreImpl: store,
		sessionManager:   mgr,
	}
}

func (h *ToolHandler) configureSession() *configureSessionHandler {
	return h.configureSessionHandler
}

// SetActiveCodebase delegates to the server's active codebase setter.
// Satisfies configureSessionDeps so configureSessionHandler does not need *Server.
func (h *ToolHandler) SetActiveCodebase(path string) {
	if h.MCPHandler != nil && h.MCPHandler.server != nil {
		h.MCPHandler.server.SetActiveCodebase(path)
	}
}

// ClearLogEntries delegates to the server log store.
// Satisfies configureSessionDeps so configureSessionHandler does not need *Server.
func (h *ToolHandler) ClearLogEntries() {
	if h.MCPHandler != nil && h.MCPHandler.server != nil {
		h.MCPHandler.server.logs.clearEntries()
	}
}

// LogEntryCount delegates to the server log store.
// Satisfies configureSessionDeps so configureSessionHandler does not need *Server.
func (h *ToolHandler) LogEntryCount() int {
	if h.MCPHandler != nil && h.MCPHandler.server != nil {
		return h.MCPHandler.server.logs.getEntryCount()
	}
	return 0
}
