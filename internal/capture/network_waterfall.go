// Purpose: Implements buffering and retrieval for captured network waterfall timing entries.
// Why: Preserves request-timing evidence for performance and diagnostics tooling.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"
)

// AddNetworkWaterfallEntries adds network waterfall entries to the buffer.
// Each entry is tagged with the page URL and current timestamp.
func (c *Capture) AddNetworkWaterfallEntries(entries []NetworkWaterfallEntry, pageURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	for i := range entries {
		// Tag entry with page URL and timestamp
		entries[i].PageURL = pageURL
		entries[i].Timestamp = now

		// Add to ring buffer
		c.networkWaterfall.entries = append(c.networkWaterfall.entries, entries[i])
	}

	// Enforce capacity - keep only the most recent entries.
	// Allocate new slice to release old backing array for GC.
	if len(c.networkWaterfall.entries) > c.networkWaterfall.capacity {
		kept := make([]NetworkWaterfallEntry, c.networkWaterfall.capacity)
		copy(kept, c.networkWaterfall.entries[len(c.networkWaterfall.entries)-c.networkWaterfall.capacity:])
		c.networkWaterfall.entries = kept
	}
}

// GetNetworkWaterfallCount returns the current number of waterfall entries.
func (c *Capture) GetNetworkWaterfallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.networkWaterfall.entries)
}

// GetNetworkWaterfallEntries returns all waterfall entries.
func (c *Capture) GetNetworkWaterfallEntries() []NetworkWaterfallEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.networkWaterfall.entries) == 0 {
		return []NetworkWaterfallEntry{}
	}

	result := make([]NetworkWaterfallEntry, len(c.networkWaterfall.entries))
	copy(result, c.networkWaterfall.entries)
	return result
}
