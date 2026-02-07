// enhanced_actions.go — User action (click, input, navigation) buffering.
// Captures browser user actions with multi-strategy selectors.
// Design: Ring buffer with memory-based eviction.
package capture

import (
	"fmt"
	"os"
	"time"
)

// AddEnhancedActions adds enhanced actions to the buffer.
// Enforces memory limits and updates running totals.
func (c *Capture) AddEnhancedActions(actions []EnhancedAction) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive: verify parallel arrays are in sync
	if len(c.enhancedActions) != len(c.actionAddedAt) {
		fmt.Fprintf(os.Stderr, "[gasoline] WARNING: enhancedActions/actionAddedAt length mismatch: %d != %d (recovering by truncating)\n",
			len(c.enhancedActions), len(c.actionAddedAt))
		minLen := min(len(c.enhancedActions), len(c.actionAddedAt))
		c.enhancedActions = c.enhancedActions[:minLen]
		c.actionAddedAt = c.actionAddedAt[:minLen]
	}

	// Enforce memory limits before adding
	c.enforceMemory()

	c.actionTotalAdded += int64(len(actions))
	now := time.Now()

	// Collect active test IDs for tagging
	activeTestIDs := make([]string, 0)
	for testID := range c.ext.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	for i := range actions {
		// Tag entry with active test IDs
		actions[i].TestIDs = activeTestIDs

		// Add to ring buffer
		c.enhancedActions = append(c.enhancedActions, actions[i])
		c.actionAddedAt = append(c.actionAddedAt, now)
	}

	// Enforce max count (respecting minimal mode)
	capacity := c.effectiveActionCapacity()
	if len(c.enhancedActions) > capacity {
		keep := len(c.enhancedActions) - capacity
		newActions := make([]EnhancedAction, capacity)
		copy(newActions, c.enhancedActions[keep:])
		c.enhancedActions = newActions
		newAddedAt := make([]time.Time, capacity)
		copy(newAddedAt, c.actionAddedAt[keep:])
		c.actionAddedAt = newAddedAt
	}
}

// GetEnhancedActionCount returns the current number of enhanced actions in the buffer.
func (c *Capture) GetEnhancedActionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.enhancedActions)
}
