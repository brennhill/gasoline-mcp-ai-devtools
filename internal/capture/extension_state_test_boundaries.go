package capture

// GetActiveTestIDs returns the list of currently active test IDs.
func (c *Capture) GetActiveTestIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.extensionState.activeTestIDs))
	for testID := range c.extensionState.activeTestIDs {
		result = append(result, testID)
	}
	return result
}

// SetTestBoundaryStart marks a test boundary as active for future event tagging.
//
// Invariants:
// - activeTestIDs behaves as a set (idempotent insert).
func (c *Capture) SetTestBoundaryStart(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.activeTestIDs[id] = true
}

// SetTestBoundaryEnd clears a test boundary marker.
//
// Failure semantics:
// - Deleting unknown IDs is a no-op.
func (c *Capture) SetTestBoundaryEnd(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.extensionState.activeTestIDs, id)
}

// getExtensionSnapshot returns a snapshot of extension state.
// MUST be called with c.mu held (RLock or Lock).
func (c *Capture) getExtensionSnapshot() ExtensionSnapshot {
	return ExtensionSnapshot{
		LastPollAt:          c.extensionState.lastPollAt,
		ExtSessionID:        c.extensionState.extSessionID,
		ExtSessionChangedAt: c.extensionState.extSessionChangedAt,
		PilotEnabled:        c.extensionState.pilotEnabled,
		ActiveTestIDCount:   len(c.extensionState.activeTestIDs),
	}
}
