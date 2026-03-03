// Purpose: Defines the dedicated interact state snapshot sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating state save/load/capture behavior.
// Docs: docs/features/feature/state-time-travel/index.md

package main

type stateInteractHandler struct {
	parent *ToolHandler
}

func newStateInteractHandler(parent *ToolHandler) *stateInteractHandler {
	return &stateInteractHandler{parent: parent}
}

func (h *ToolHandler) stateInteract() *stateInteractHandler {
	if h.stateInteractHandler == nil {
		h.stateInteractHandler = newStateInteractHandler(h)
	}
	return h.stateInteractHandler
}
