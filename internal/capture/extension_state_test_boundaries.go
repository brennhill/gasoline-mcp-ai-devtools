// Purpose: Manages active test boundary IDs for event tagging during test runs.
// Why: Separates test-boundary lifecycle from other extension state to keep CI concerns isolated.
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
