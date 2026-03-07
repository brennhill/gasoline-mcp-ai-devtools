// Purpose: Implements ingestion and buffering of enhanced action telemetry with navigation callback integration.
// Why: Preserves action history needed for replay/test-generation while enforcing capture memory constraints.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// AddEnhancedActions ingests action telemetry and optionally triggers navigation callback.
//
// Invariants:
// - Each enhancedActionEntry stores the action and its ingestion timestamp together.
// - actionTotalAdded is monotonic and never decremented outside explicit clear operations.
// - navigationCallback is captured under lock and invoked after unlock.
//
// Failure semantics:
// - Oversized action batches are accepted and oldest entries are evicted.
func (c *Capture) AddEnhancedActions(actions []EnhancedAction) {
	navCb := func() func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		now := time.Now()

		// Collect active test IDs for tagging
		activeTestIDs := make([]string, 0)
		for testID := range c.extensionState.activeTestIDs {
			activeTestIDs = append(activeTestIDs, testID)
		}

		for i := range actions {
			// Tag entry with active test IDs
			actions[i].TestIDs = activeTestIDs
		}

		hasNavigation := c.buffers.appendEnhancedActions(actions, now)

		if hasNavigation {
			return c.navigationCallback
		}
		return nil
	}()

	// Fire navigation callback outside lock to prevent deadlocks
	if navCb != nil {
		util.SafeGo(navCb)
	}
}

// GetEnhancedActionCount returns the current number of enhanced actions in the buffer.
func (c *Capture) GetEnhancedActionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.actionCount()
}
