// Purpose: Defines the dedicated configure session/store sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating session-oriented configure flows.
// Docs: docs/features/feature/config-profiles/index.md

package main

type configureSessionHandler struct {
	parent *ToolHandler
}

func newConfigureSessionHandler(parent *ToolHandler) *configureSessionHandler {
	return &configureSessionHandler{parent: parent}
}

func (h *ToolHandler) configureSession() *configureSessionHandler {
	if h.configureSessionHandler == nil {
		h.configureSessionHandler = newConfigureSessionHandler(h)
	}
	return h.configureSessionHandler
}
