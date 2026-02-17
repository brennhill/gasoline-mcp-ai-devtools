// Purpose: Owns network_waterfall.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// network_waterfall.go — Network waterfall (PerformanceResourceTiming) buffering.
// Captures browser resource timing data for CSP generation and performance analysis.
// Design: Ring buffer with configurable capacity.
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
		c.nw.entries = append(c.nw.entries, entries[i])
	}

	// Enforce capacity - keep only the most recent entries.
	// Allocate new slice to release old backing array for GC.
	if len(c.nw.entries) > c.nw.capacity {
		kept := make([]NetworkWaterfallEntry, c.nw.capacity)
		copy(kept, c.nw.entries[len(c.nw.entries)-c.nw.capacity:])
		c.nw.entries = kept
	}
}

// GetNetworkWaterfallCount returns the current number of waterfall entries.
func (c *Capture) GetNetworkWaterfallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.nw.entries)
}

// GetNetworkWaterfallEntries returns all waterfall entries.
func (c *Capture) GetNetworkWaterfallEntries() []NetworkWaterfallEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.nw.entries) == 0 {
		return []NetworkWaterfallEntry{}
	}

	result := make([]NetworkWaterfallEntry, len(c.nw.entries))
	copy(result, c.nw.entries)
	return result
}
