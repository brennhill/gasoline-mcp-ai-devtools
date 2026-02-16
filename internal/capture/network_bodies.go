// network_bodies.go — Network body (HTTP request/response) buffering.
// Captures HTTP request/response bodies with binary format detection.
// Design: Ring buffer with memory-based eviction.
package capture

import (
	"fmt"
	"os"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// repairNBParallelArrays truncates network body parallel arrays to equal length if mismatched.
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

// detectAndSetBinaryFormat detects binary format from request or response body.
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

// evictNBByCount trims network bodies to MaxNetworkBodies, updating memory accounting.
func (c *Capture) evictNBByCount() {
	if len(c.networkBodies) <= MaxNetworkBodies {
		return
	}
	keep := len(c.networkBodies) - MaxNetworkBodies
	for j := 0; j < keep; j++ {
		c.nbMemoryTotal -= nbEntryMemory(&c.networkBodies[j])
	}
	newBodies := make([]NetworkBody, MaxNetworkBodies)
	copy(newBodies, c.networkBodies[keep:])
	c.networkBodies = newBodies
	newAddedAt := make([]time.Time, MaxNetworkBodies)
	copy(newAddedAt, c.networkAddedAt[keep:])
	c.networkAddedAt = newAddedAt
}

// AddNetworkBodies adds network bodies to the buffer.
// Enforces memory limits and updates running totals.
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
	for testID := range c.ext.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	for i := range bodies {
		bodies[i].TestIDs = activeTestIDs
		detectAndSetBinaryFormat(&bodies[i])
		c.networkBodies = append(c.networkBodies, bodies[i])
		c.networkAddedAt = append(c.networkAddedAt, now)
		c.nbMemoryTotal += nbEntryMemory(&bodies[i])
	}

	c.evictNBByCount()
	c.evictNBForMemory()
}

// evictNBForMemory removes oldest bodies if memory exceeds limit.
// Calculates how many entries to drop in a single pass.
func (c *Capture) evictNBForMemory() {
	excess := c.nbMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(c.networkBodies) && excess > 0 {
		entryMem := nbEntryMemory(&c.networkBodies[drop])
		excess -= entryMem
		c.nbMemoryTotal -= entryMem
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
