// Purpose: Defines the dedicated interact action dispatch sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating interact routing/jitter orchestration.
// Docs: docs/features/feature/interact-explore/index.md

package main

import "sync"

type interactActionHandler struct {
	parent *ToolHandler

	// Action jitter: randomized micro-delays before interact actions.
	// Relocated from ToolHandler — exclusively owned by interactActionHandler.
	jitterMu          sync.RWMutex
	actionJitterMaxMs int // max jitter before each interact action (0 = disabled)

	// Optional evidence capture state keyed by correlation_id.
	// Tracks before/after screenshots for interact actions when evidence mode is enabled.
	// Relocated from ToolHandler — exclusively owned by interactActionHandler.
	evidenceMu        sync.Mutex
	evidenceByCommand map[string]*commandEvidenceState

	// Deterministic retry contract metadata keyed by correlation_id.
	// Relocated from ToolHandler — exclusively owned by interactActionHandler.
	retryContractMu sync.Mutex
	retryByCommand  map[string]*commandRetryState

	// Scoped element index registry used by list_interactive/index follow-up actions.
	// Relocated from ToolHandler — exclusively owned by interactActionHandler.
	elementIndexRegistry *elementIndexRegistry
}

func newInteractActionHandler(parent *ToolHandler) *interactActionHandler {
	return &interactActionHandler{
		parent:               parent,
		evidenceByCommand:    make(map[string]*commandEvidenceState),
		retryByCommand:       make(map[string]*commandRetryState),
		elementIndexRegistry: newElementIndexRegistry(),
	}
}

// SetJitter sets the maximum jitter delay in milliseconds.
func (h *interactActionHandler) SetJitter(maxMs int) {
	h.jitterMu.Lock()
	defer h.jitterMu.Unlock()
	h.actionJitterMaxMs = maxMs
}

// GetJitter returns the current maximum jitter delay in milliseconds.
func (h *interactActionHandler) GetJitter() int {
	h.jitterMu.RLock()
	defer h.jitterMu.RUnlock()
	return h.actionJitterMaxMs
}

func (h *ToolHandler) interactAction() *interactActionHandler {
	return h.interactActionHandler
}
