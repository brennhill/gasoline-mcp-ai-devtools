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
	if len(c.networkBodies) == len(c.networkAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: networkBodies/networkAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(c.networkBodies), len(c.networkAddedAt))
	minLen := min(len(c.networkBodies), len(c.networkAddedAt))
	c.networkBodies = c.networkBodies[:minLen]
	c.networkAddedAt = c.networkAddedAt[:minLen]
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
// - c.networkBodyMemoryTotal remains consistent with surviving entries after eviction.
//
// Failure semantics:
// - Oldest entries are dropped first (FIFO eviction).
func (c *Capture) evictNBByCount() {
	if len(c.networkBodies) <= MaxNetworkBodies {
		return
	}
	keep := len(c.networkBodies) - MaxNetworkBodies
	for j := 0; j < keep; j++ {
		c.networkBodyMemoryTotal -= nbEntryMemory(&c.networkBodies[j])
	}
	newBodies := make([]NetworkBody, MaxNetworkBodies)
	copy(newBodies, c.networkBodies[keep:])
	c.networkBodies = newBodies
	newAddedAt := make([]time.Time, MaxNetworkBodies)
	copy(newAddedAt, c.networkAddedAt[keep:])
	c.networkAddedAt = newAddedAt
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
	c.networkTotalAdded += int64(len(bodies))
	for i := range bodies {
		if bodies[i].Status >= 400 {
			c.networkErrorTotalAdded++
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
		c.networkBodies = append(c.networkBodies, bodies[i])
		c.networkAddedAt = append(c.networkAddedAt, now)
		c.networkBodyMemoryTotal += nbEntryMemory(&bodies[i])
	}

	c.evictNBByCount()
	c.evictNBForMemory()
}

// evictNBForMemory enforces memory cap using oldest-first eviction.
//
// Invariants:
// - c.networkBodyMemoryTotal is decremented by exact removed-entry estimates.
//
// Failure semantics:
// - Drops enough leading entries to get under cap in one pass; may remove multiple recent appends.
func (c *Capture) evictNBForMemory() {
	excess := c.networkBodyMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.networkBodies) && excess > 0 {
		entryMem := nbEntryMemory(&c.networkBodies[drop])
		excess -= entryMem
		c.networkBodyMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]NetworkBody, len(c.networkBodies)-drop)
	copy(surviving, c.networkBodies[drop:])
	c.networkBodies = surviving
	if len(c.networkAddedAt) >= drop {
		survivingAt := make([]time.Time, len(c.networkAddedAt)-drop)
		copy(survivingAt, c.networkAddedAt[drop:])
		c.networkAddedAt = survivingAt
	}
}

// GetNetworkBodyCount returns the current number of network bodies in the buffer.
func (c *Capture) GetNetworkBodyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.networkBodies)
}
