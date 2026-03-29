// Purpose: Implements network body ingestion, repair, enrichment, and buffer filtering operations.
// Why: Preserves request/response payload evidence while maintaining bounded memory and schema consistency.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// detectAndSetBinaryFormat infers payload format only when not already set.
//
// Failure semantics:
// - Detection is best-effort; unknown formats leave fields empty without erroring ingestion.
func detectAndSetBinaryFormat(body *NetworkBody) {
	if body.BinaryFormat != "" {
		return
	}
	if len(body.RequestBody) > 0 {
		if format := util.DetectBinaryFormat([]byte(body.RequestBody)); format != nil {
			body.BinaryFormat = format.Name
			body.FormatConfidence = format.Confidence
			return
		}
	}
	if len(body.ResponseBody) > 0 {
		if format := util.DetectBinaryFormat([]byte(body.ResponseBody)); format != nil {
			body.BinaryFormat = format.Name
			body.FormatConfidence = format.Confidence
		}
	}
}

// AddNetworkBodies ingests a batch into the network evidence ring buffer.
//
// Invariants:
// - Each networkBodyEntry stores the body and its ingestion timestamp together.
// - Totals are monotonic (`networkTotalAdded`, `networkErrorTotalAdded`) and never decremented.
// - Active test IDs are snapshotted once per batch for consistent event tagging.
//
// Failure semantics:
// - Batch ingestion never partially fails; over-capacity data is deterministically evicted.
func (c *Capture) AddNetworkBodies(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	activeTestIDs := make([]string, 0)
	for testID := range c.extensionState.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	c.buffers.appendNetworkBodies(bodies, activeTestIDs, now)
}

// GetNetworkBodyCount returns the current number of network bodies in the buffer.
func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkCount()
}
