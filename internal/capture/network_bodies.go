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

// AddNetworkBodies adds network bodies to the buffer.
// Enforces memory limits and updates running totals.
func (c *Capture) AddNetworkBodies(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive: verify parallel arrays are in sync
	if len(c.networkBodies) != len(c.networkAddedAt) {
		fmt.Fprintf(os.Stderr, "[gasoline] WARNING: networkBodies/networkAddedAt length mismatch: %d != %d (recovering by truncating)\n",
			len(c.networkBodies), len(c.networkAddedAt))
		minLen := min(len(c.networkBodies), len(c.networkAddedAt))
		c.networkBodies = c.networkBodies[:minLen]
		c.networkAddedAt = c.networkAddedAt[:minLen]
	}

	c.networkTotalAdded += int64(len(bodies))
	now := time.Now()

	// Collect active test IDs for tagging
	activeTestIDs := make([]string, 0)
	for testID := range c.ext.activeTestIDs {
		activeTestIDs = append(activeTestIDs, testID)
	}

	for i := range bodies {
		// Tag entry with active test IDs
		bodies[i].TestIDs = activeTestIDs

		// Detect binary format in request body
		if bodies[i].BinaryFormat == "" && len(bodies[i].RequestBody) > 0 {
			if format := util.DetectBinaryFormat([]byte(bodies[i].RequestBody)); format != nil {
				bodies[i].BinaryFormat = format.Name
				bodies[i].FormatConfidence = format.Confidence
			}
		}

		// Detect binary format in response body if not already detected
		if bodies[i].BinaryFormat == "" && len(bodies[i].ResponseBody) > 0 {
			if format := util.DetectBinaryFormat([]byte(bodies[i].ResponseBody)); format != nil {
				bodies[i].BinaryFormat = format.Name
				bodies[i].FormatConfidence = format.Confidence
			}
		}

		// Add to ring buffer
		c.networkBodies = append(c.networkBodies, bodies[i])
		c.networkAddedAt = append(c.networkAddedAt, now)
		c.nbMemoryTotal += nbEntryMemory(&bodies[i])
	}

	// Enforce max count
	if len(c.networkBodies) > MaxNetworkBodies {
		keep := len(c.networkBodies) - MaxNetworkBodies
		// Subtract memory for evicted entries
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

	// Enforce per-buffer memory limit
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
