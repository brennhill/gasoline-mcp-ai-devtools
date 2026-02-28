// Purpose: Implements ingestion and buffering of enhanced action telemetry with navigation callback integration.
// Why: Preserves action history needed for replay/test-generation while enforcing capture memory constraints.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"fmt"
	"os"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// AddEnhancedActions ingests action telemetry and optionally triggers navigation callback.
//
// Invariants:
// - enhancedActions/actionAddedAt remain index-aligned.
// - actionTotalAdded is monotonic and never decremented outside explicit clear operations.
// - navigationCallback is captured under lock and invoked after unlock.
//
// Failure semantics:
// - Parallel-array mismatch is repaired by truncation to common prefix.
// - Oversized action batches are accepted and oldest entries are evicted.
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
	for testID := range c.extensionState.activeTestIDs {
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
