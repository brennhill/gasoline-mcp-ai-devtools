// Purpose: Defines the dedicated interact action dispatch sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating interact routing/jitter orchestration.
// Docs: docs/features/feature/interact-explore/index.md

package main

import "sync"

type interactActionHandler struct {
	parent *ToolHandler

	// Cached interact dispatch map (initialized once).
	once     sync.Once
	handlers map[string]interactHandler
}

func newInteractActionHandler(parent *ToolHandler) *interactActionHandler {
	return &interactActionHandler{parent: parent}
}

func (h *ToolHandler) interactAction() *interactActionHandler {
	if h.interactActionHandler == nil {
		h.interactActionHandler = newInteractActionHandler(h)
	}
	return h.interactActionHandler
}
