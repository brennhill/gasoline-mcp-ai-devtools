// memory.go — Per-buffer memory tracking and estimation.
// Each buffer (WS events, network bodies, actions) tracks its own memory
// independently. Per-buffer limits are enforced at ingest time by each buffer's
// own eviction function (evictWSForMemory, evictNBForMemory).
package capture

const (
	// Per-entry memory estimates
	wsEventOverhead     = 200 // bytes overhead per WS event
	networkBodyOverhead = 300 // bytes overhead per network body
	actionMemoryFixed   = 500 // bytes per enhanced action (fixed estimate)
)

// ============================================
// Per-Entry Memory Calculation
// ============================================

// wsEventMemory returns the memory estimate for a single WS event.
func wsEventMemory(e *WebSocketEvent) int64 {
	return int64(len(e.Data)) + wsEventOverhead
}

// nbEntryMemory returns the memory estimate for a single network body entry.
func nbEntryMemory(b *NetworkBody) int64 {
	return int64(len(b.RequestBody)+len(b.ResponseBody)) + networkBodyOverhead
}

// ============================================
// Per-Buffer Memory Accessors (caller must hold lock)
// ============================================

// calcWSMemory returns the running total of WS buffer memory (caller must hold lock).
// O(1) — maintained incrementally by add/evict/clear operations.
func (c *Capture) calcWSMemory() int64 {
	return c.wsMemoryTotal
}

// calcNBMemory returns the running total of network bodies buffer memory (caller must hold lock).
// O(1) — maintained incrementally by add/evict/clear operations.
func (c *Capture) calcNBMemory() int64 {
	return c.nbMemoryTotal
}

// calcActionMemory approximates memory usage of enhanced actions buffer (caller must hold lock)
func (c *Capture) calcActionMemory() int64 {
	return int64(len(c.enhancedActions)) * actionMemoryFixed
}

// ============================================
// Public Memory Accessors
// ============================================

// GetWebSocketBufferMemory returns approximate memory usage of WS buffer
func (c *Capture) GetWebSocketBufferMemory() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calcWSMemory()
}

// GetNetworkBodiesBufferMemory returns approximate memory usage of network bodies buffer
func (c *Capture) GetNetworkBodiesBufferMemory() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calcNBMemory()
}
