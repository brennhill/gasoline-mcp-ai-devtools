// enhanced_actions.go — User action (click, input, navigation) buffering.
// Captures browser user actions with multi-strategy selectors.
// Design: Ring buffer with memory-based eviction.
package capture

import (
	"fmt"
	"os"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// AddEnhancedActions adds enhanced actions to the buffer.
// Enforces memory limits and updates running totals.
// If any action is a navigation, fires the navigation callback (outside lock).
func (c *Capture) AddEnhancedActions(actions []EnhancedAction) {
	var navCb func()

	c.mu.Lock()

	// Defensive: verify parallel arrays are in sync
	if len(c.enhancedActions) != len(c.actionAddedAt) {
		fmt.Fprintf(os.Stderr, "[gasoline] WARNING: enhancedActions/actionAddedAt length mismatch: %d != %d (recovering by truncating)\n",
			len(c.enhancedActions), len(c.actionAddedAt))
		minLen := min(len(c.enhancedActions), len(c.actionAddedAt))
		c.enhancedActions = c.enhancedActions[:minLen]
		c.actionAddedAt = c.actionAddedAt[:minLen]
	}

	c.actionTotalAdded += int64(len(actions))
	now := time.Now()

	// Collect active test IDs for tagging
	activeTestIDs := make([]string, 0)
	for testID := range c.ext.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	hasNavigation := false
	for i := range actions {
		// Tag entry with active test IDs
		actions[i].TestIDs = activeTestIDs

		// Add to ring buffer
		c.enhancedActions = append(c.enhancedActions, actions[i])
		c.actionAddedAt = append(c.actionAddedAt, now)

		// Detect navigation actions
		if actions[i].Type == "navigation" {
			hasNavigation = true
		}
	}

	// Enforce max count
	if len(c.enhancedActions) > MaxEnhancedActions {
		keep := len(c.enhancedActions) - MaxEnhancedActions
		newActions := make([]EnhancedAction, MaxEnhancedActions)
		copy(newActions, c.enhancedActions[keep:])
		c.enhancedActions = newActions
		newAddedAt := make([]time.Time, MaxEnhancedActions)
		copy(newAddedAt, c.actionAddedAt[keep:])
		c.actionAddedAt = newAddedAt
	}

	// Capture callback reference before releasing lock
	if hasNavigation && c.navigationCallback != nil {
		navCb = c.navigationCallback
	}

	c.mu.Unlock()

	// Fire navigation callback outside lock to prevent deadlocks
	if navCb != nil {
		util.SafeGo(navCb)
	}
}

// GetEnhancedActionCount returns the current number of enhanced actions in the buffer.
func (c *Capture) GetEnhancedActionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.enhancedActions)
}
