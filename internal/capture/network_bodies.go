// Purpose: Implements network body ingestion, repair, enrichment, and buffer filtering operations.
// Why: Preserves request/response payload evidence while maintaining bounded memory and schema consistency.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"fmt"
	"os"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// repairNBParallelArrays repairs ring-buffer parallel-slice corruption in-place.
//
// Invariants:
// - networkBodies and networkAddedAt must have identical length.
//
// Failure semantics:
// - On mismatch, keeps the common prefix and drops tail data to restore index alignment.
// - Emits warning to stderr because truncated entries imply prior mutation bug.
func (c *Capture) repairNBParallelArrays() {
	if len(c.buffers.networkBodies) == len(c.buffers.networkAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: networkBodies/networkAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(c.buffers.networkBodies), len(c.buffers.networkAddedAt))
	minLen := min(len(c.buffers.networkBodies), len(c.buffers.networkAddedAt))
	c.buffers.networkBodies = c.buffers.networkBodies[:minLen]
	c.buffers.networkAddedAt = c.buffers.networkAddedAt[:minLen]
}

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

// evictNBByCount enforces count-based cap while preserving newest entries.
//
// Invariants:
// - c.buffers.networkBodyMemoryTotal remains consistent with surviving entries after eviction.
//
// Failure semantics:
// - Oldest entries are dropped first (FIFO eviction).
func (c *Capture) evictNBByCount() {
	if len(c.buffers.networkBodies) <= MaxNetworkBodies {
		return
	}
	keep := len(c.buffers.networkBodies) - MaxNetworkBodies
	for j := 0; j < keep; j++ {
		c.buffers.networkBodyMemoryTotal -= nbEntryMemory(&c.buffers.networkBodies[j])
	}
	newBodies := make([]NetworkBody, MaxNetworkBodies)
	copy(newBodies, c.buffers.networkBodies[keep:])
	c.buffers.networkBodies = newBodies
	newAddedAt := make([]time.Time, MaxNetworkBodies)
	copy(newAddedAt, c.buffers.networkAddedAt[keep:])
	c.buffers.networkAddedAt = newAddedAt
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

	c.repairNBParallelArrays()
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

	c.evictNBByCount()
	c.evictNBForMemory()
}

// evictNBForMemory enforces memory cap using oldest-first eviction.
//
// Invariants:
// - c.buffers.networkBodyMemoryTotal is decremented by exact removed-entry estimates.
//
// Failure semantics:
// - Drops enough leading entries to get under cap in one pass; may remove multiple recent appends.
func (c *Capture) evictNBForMemory() {
	excess := c.buffers.networkBodyMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.buffers.networkBodies) && excess > 0 {
		entryMem := nbEntryMemory(&c.buffers.networkBodies[drop])
		excess -= entryMem
		c.buffers.networkBodyMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]NetworkBody, len(c.buffers.networkBodies)-drop)
	copy(surviving, c.buffers.networkBodies[drop:])
	c.buffers.networkBodies = surviving
	if len(c.buffers.networkAddedAt) >= drop {
		survivingAt := make([]time.Time, len(c.buffers.networkAddedAt)-drop)
		copy(survivingAt, c.buffers.networkAddedAt[drop:])
		c.buffers.networkAddedAt = survivingAt
	}
}

// GetNetworkBodyCount returns the current number of network bodies in the buffer.
func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buffers.networkBodies)
}
