// Purpose: Implements network body ingestion, repair, enrichment, and buffer filtering operations.
// Why: Preserves request/response payload evidence while maintaining bounded memory and schema consistency.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
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
// - networkBodies/networkAddedAt are updated in lockstep.
// - Totals are monotonic (`networkTotalAdded`, `networkErrorTotalAdded`) and never decremented.
// - Active test IDs are snapshotted once per batch for consistent event tagging.
//
// Failure semantics:
// - Batch ingestion never partially fails; over-capacity data is deterministically evicted.
func (c *Capture) AddNetworkBodies(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffers.repairNetworkParallelArrays()
	c.buffers.networkTotalAdded += int64(len(bodies))
	for i := range bodies {
		if bodies[i].Status >= 400 {
			c.buffers.networkErrorTotalAdded++
		}
	}
	now := time.Now()

	activeTestIDs := make([]string, 0)
	for testID := range c.extensionState.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	for i := range bodies {
		bodies[i].TestIDs = activeTestIDs
		detectAndSetBinaryFormat(&bodies[i])
		c.buffers.networkBodies = append(c.buffers.networkBodies, bodies[i])
		c.buffers.networkAddedAt = append(c.buffers.networkAddedAt, now)
		c.buffers.networkBodyMemoryTotal += nbEntryMemory(&bodies[i])
	}

	c.buffers.evictNetworkByCount()
	c.buffers.evictNetworkForMemory()
}

// GetNetworkBodyCount returns the current number of network bodies in the buffer.
func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkCount()
}
