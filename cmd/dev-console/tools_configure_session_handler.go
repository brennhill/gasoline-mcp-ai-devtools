// Purpose: Defines the dedicated configure session/store sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating session-oriented configure flows.
// Docs: docs/features/feature/config-profiles/index.md

package main

import (
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/persistence"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/session"
)

// configureSessionDeps defines the narrow interface that configureSessionHandler needs.
type configureSessionDeps interface {
	requireSessionStore(req JSONRPCRequest) (JSONRPCResponse, bool)
	invalidateSummaryPref()
}

type configureSessionHandler struct {
	deps configureSessionDeps

	// Concrete implementations injected at construction.
	sessionStoreImpl *persistence.SessionStore
	sessionManager   *session.Manager
	server           *Server
}

func newConfigureSessionHandler(deps configureSessionDeps, store *persistence.SessionStore, mgr *session.Manager, srv *Server) *configureSessionHandler {
	return &configureSessionHandler{
		deps:             deps,
		sessionStoreImpl: store,
		sessionManager:   mgr,
		server:           srv,
	}
}

func (h *ToolHandler) configureSession() *configureSessionHandler {
	if h.configureSessionHandler == nil {
		h.configureSessionHandler = newConfigureSessionHandler(h, h.sessionStoreImpl, h.sessionManager, h.server)
	}
	return h.configureSessionHandler
}
