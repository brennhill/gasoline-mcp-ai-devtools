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
		c.networkWaterfall = append(c.networkWaterfall, entries[i])
	}

	// Enforce capacity - keep only the most recent entries.
	// Allocate new slice to release old backing array for GC.
	if len(c.networkWaterfall) > c.networkWaterfallCapacity {
		kept := make([]NetworkWaterfallEntry, c.networkWaterfallCapacity)
		copy(kept, c.networkWaterfall[len(c.networkWaterfall)-c.networkWaterfallCapacity:])
		c.networkWaterfall = kept
	}
}

// GetNetworkWaterfallCount returns the current number of waterfall entries.
func (c *Capture) GetNetworkWaterfallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.networkWaterfall)
}

// GetNetworkWaterfallEntries returns all waterfall entries.
func (c *Capture) GetNetworkWaterfallEntries() []NetworkWaterfallEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.networkWaterfall) == 0 {
		return []NetworkWaterfallEntry{}
	}

	result := make([]NetworkWaterfallEntry, len(c.networkWaterfall))
	copy(result, c.networkWaterfall)
	return result
}
