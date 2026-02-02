// test_boundary.go — Test boundary tracking for CI/CD correlation.
// Tracks active test IDs for correlating entries captured during tests.
package capture

// GetActiveTestIDs returns the list of currently active test IDs.
func (c *Capture) GetActiveTestIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.activeTestIDs))
	for testID := range c.activeTestIDs {
		result = append(result, testID)
	}
	return result
}

// SetTestBoundaryStart adds a test ID to the active set for correlating entries.
// Deprecated: Use test_boundary_start action via configure tool instead.
func (c *Capture) SetTestBoundaryStart(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeTestIDs[id] = true
}

// SetTestBoundaryEnd removes a test ID from the active set.
// Deprecated: Use test_boundary_end action via configure tool instead.
func (c *Capture) SetTestBoundaryEnd(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.activeTestIDs, id)
}
