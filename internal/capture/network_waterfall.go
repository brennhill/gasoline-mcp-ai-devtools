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

	c.networkWaterfall.appendEntries(entries, pageURL, time.Now())
}

// GetNetworkWaterfallCount returns the current number of waterfall entries.
func (c *Capture) GetNetworkWaterfallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkWaterfall.count()
}

// GetNetworkWaterfallEntries returns all waterfall entries.
func (c *Capture) GetNetworkWaterfallEntries() []NetworkWaterfallEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.networkWaterfall.count() == 0 {
		return []NetworkWaterfallEntry{}
	}
	return c.networkWaterfall.snapshot()
}
