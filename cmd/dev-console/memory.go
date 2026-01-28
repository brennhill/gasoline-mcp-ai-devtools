// memory.go — Memory enforcement with soft/hard limits and eviction policies.
// Periodic goroutine checks memory usage and evicts oldest entries when the
// soft limit (20MB) is exceeded. Hard limit (50MB) triggers aggressive eviction.
// Design: Each buffer (logs, network, websocket, actions) is evicted
// independently based on its contribution to total memory pressure.
package main

import (
	"time"
)

// ============================================
// Memory Enforcement Constants
// ============================================

const (
	memorySoftLimit     = 20 * 1024 * 1024  // 20MB - evict oldest 25%
	memoryCriticalLimit = 100 * 1024 * 1024 // 100MB - clear all, minimal mode
	memoryCheckInterval = 10 * time.Second  // periodic check interval
	evictionCooldown    = 1 * time.Second   // min time between eviction cycles

	// Per-entry memory estimates
	wsEventOverhead     = 200 // bytes overhead per WS event
	networkBodyOverhead = 300 // bytes overhead per network body
	actionMemoryFixed   = 500 // bytes per enhanced action (fixed estimate)
)

// ============================================
// Memory Status
// ============================================

// MemoryStatus represents the current memory enforcement state
type MemoryStatus struct {
	TotalBytes     int64 `json:"total_bytes"`
	WebSocketBytes int64 `json:"websocket_bytes"`
	NetworkBytes   int64 `json:"network_bytes"`
	ActionsBytes   int64 `json:"actions_bytes"`
	SoftLimit      int64 `json:"soft_limit"`
	HardLimit      int64 `json:"hard_limit"`
	CriticalLimit  int64 `json:"critical_limit"`
	MinimalMode    bool  `json:"minimal_mode"`
	TotalEvictions int   `json:"total_evictions"`
	EvictedEntries int   `json:"evicted_entries"`
}

// GetMemoryStatus returns the current memory enforcement state
func (c *Capture) GetMemoryStatus() MemoryStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	wsMem := c.calcWSMemory()
	nbMem := c.calcNBMemory()
	actionMem := c.calcActionMemory()

	return MemoryStatus{
		TotalBytes:     wsMem + nbMem + actionMem,
		WebSocketBytes: wsMem,
		NetworkBytes:   nbMem,
		ActionsBytes:   actionMem,
		SoftLimit:      memorySoftLimit,
		HardLimit:      memoryHardLimit,
		CriticalLimit:  memoryCriticalLimit,
		MinimalMode:    c.mem.minimalMode,
		TotalEvictions: c.mem.totalEvictions,
		EvictedEntries: c.mem.evictedEntries,
	}
}

// ============================================
// Memory Calculation
// ============================================

// calcTotalMemory returns total memory across all buffers (caller must hold lock)
func (c *Capture) calcTotalMemory() int64 {
	return c.calcWSMemory() + c.calcNBMemory() + c.calcActionMemory()
}

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

// wsEventMemory returns the memory estimate for a single WS event.
func wsEventMemory(e *WebSocketEvent) int64 {
	return int64(len(e.Data)) + wsEventOverhead
}

// nbEntryMemory returns the memory estimate for a single network body entry.
func nbEntryMemory(b *NetworkBody) int64 {
	return int64(len(b.RequestBody)+len(b.ResponseBody)) + networkBodyOverhead
}

// calcActionMemory approximates memory usage of enhanced actions buffer (caller must hold lock)
func (c *Capture) calcActionMemory() int64 {
	return int64(len(c.enhancedActions)) * actionMemoryFixed
}

// ============================================
// Effective Capacities
// ============================================

// effectiveWSCapacity returns the current WS buffer capacity (halved in minimal mode)
// caller must hold lock
func (c *Capture) effectiveWSCapacity() int {
	if c.mem.minimalMode {
		return maxWSEvents / 2
	}
	return maxWSEvents
}

// effectiveNBCapacity returns the current network body buffer capacity (halved in minimal mode)
// caller must hold lock
func (c *Capture) effectiveNBCapacity() int {
	if c.mem.minimalMode {
		return maxNetworkBodies / 2
	}
	return maxNetworkBodies
}

// effectiveActionCapacity returns the current action buffer capacity (halved in minimal mode)
// caller must hold lock
func (c *Capture) effectiveActionCapacity() int {
	if c.mem.minimalMode {
		return maxEnhancedActions / 2
	}
	return maxEnhancedActions
}

// ============================================
// Memory Enforcement (Eviction)
// ============================================

// enforceMemory checks memory thresholds and evicts as needed (caller must hold lock).
// This is the primary enforcement point, called on every ingest operation.
//
// Three-Tier Enforcement Strategy:
//   1. Soft (20MB): Evict oldest 25% of each buffer (prioritize network bodies).
//   2. Hard (50MB): Evict oldest 50% of each buffer (aggressive, still prioritized).
//   3. Critical (100MB): Emergency mode — clear most buffers and enter minimal mode.
//
// Cooldown (1s) prevents thrashing when traffic approaches soft limit.
// State: Updates c.mem.lastEvictionTime on each enforcement check (regardless of action taken).
func (c *Capture) enforceMemory() {
	// Respect cooldown
	if !c.mem.lastEvictionTime.IsZero() && time.Since(c.mem.lastEvictionTime) < evictionCooldown {
		return
	}

	totalMem := c.calcTotalMemory()

	// Check critical limit first (100MB)
	if totalMem > memoryCriticalLimit {
		c.evictCritical()
		return
	}

	// Check hard limit (50MB)
	if totalMem > memoryHardLimit {
		c.evictHard()
		return
	}

	// Check soft limit (20MB)
	if totalMem > memorySoftLimit {
		c.evictSoft()
		return
	}
}

// evictSoft removes oldest 25% from each buffer (prioritizing network bodies).
// denominator=4 means 1/4 of each buffer removed. Memory target: 20MB soft limit.
// Called when total memory exceeds 20MB but is below hard limit (50MB).
// Strategy: Gentle eviction, preserve most events, network bodies removed first (largest per-entry).
func (c *Capture) evictSoft() {
	c.evictBuffers(4, memorySoftLimit)  // 1/4 removed from each buffer
}

// evictHard removes oldest 50% from each buffer (prioritizing network bodies).
// denominator=2 means 1/2 of each buffer removed. Memory target: 50MB hard limit.
// Called when total memory exceeds 50MB but is below critical limit (100MB).
// Strategy: Aggressive eviction, recover memory quickly, network bodies removed first.
// Once memory drops below limit, early exit (don't evict actions unnecessarily).
func (c *Capture) evictHard() {
	c.evictBuffers(2, memoryHardLimit)  // 1/2 removed from each buffer
}

// evictBuffers removes oldest 1/denominator entries from each buffer in priority order.
// This is the core eviction algorithm, used by both soft and hard limits.
//
// Algorithm (Single-Pass Proportional Eviction):
//   1. Calculate drop count: n = max(len(buffer)/denominator, 1)
//   2. Evict oldest n entries: copy survivors (entries from index n onward) to new slice
//   3. Subtract evicted entries from memory total (subtract per-entry cost)
//   4. Keep parallel slices in sync: wsAddedAt trimmed with wsEvents, etc.
//   5. Early exit: Stop buffer eviction when total memory < limit (save cycles)
//
// Priority Order (why network bodies first?):
//   - Network bodies are typically largest per-entry (request+response bodies)
//   - Removing 10 network bodies frees more memory than 100 WS events
//   - Early exit on memory satisfaction prevents unnecessary action eviction
//
// Memory Accounting (Single-Pass, O(n)):
//   - Loop through evicted entries: subtract wsEventMemory(entry) from wsMemoryTotal
//   - Maintains accurate memory totals without full recalculation
//   - Parallel slice invariants maintained: new(buffer) and new(addedAt) same length
//
// GC Strategy:
//   - Slice truncation alone (c.wsEvents = c.wsEvents[n:]) doesn't free backing array
//   - Instead: make(new slice), copy survivors, assign to c.wsEvents
//   - Old backing array becomes unreachable → GC reclaims it
//
// Caller must hold lock. Denominator typically 4 (soft) or 2 (hard).
func (c *Capture) evictBuffers(denominator int, limit int64) {
	var evicted int

	// Network bodies first (largest per entry)
	if len(c.networkBodies) > 0 {
		n := max(len(c.networkBodies)/denominator, 1)
		// Clamp n to actual array length to prevent panic
		if n > len(c.networkBodies) {
			n = len(c.networkBodies)
		}
		// Subtract memory for evicted entries
		for j := 0; j < n; j++ {
			c.nbMemoryTotal -= nbEntryMemory(&c.networkBodies[j])
		}
		surviving := make([]NetworkBody, len(c.networkBodies)-n)
		copy(surviving, c.networkBodies[n:])
		c.networkBodies = surviving
		// Keep networkAddedAt and networkBodies in sync
		nAt := n
		if nAt > len(c.networkAddedAt) {
			nAt = len(c.networkAddedAt)
		}
		survivingAt := make([]time.Time, len(c.networkAddedAt)-nAt)
		copy(survivingAt, c.networkAddedAt[nAt:])
		c.networkAddedAt = survivingAt
		evicted += n
	}

	if c.calcTotalMemory() > limit && len(c.wsEvents) > 0 {
		n := max(len(c.wsEvents)/denominator, 1)
		// Clamp n to actual array length to prevent panic
		if n > len(c.wsEvents) {
			n = len(c.wsEvents)
		}
		// Subtract memory for evicted entries
		for j := 0; j < n; j++ {
			c.wsMemoryTotal -= wsEventMemory(&c.wsEvents[j])
		}
		surviving := make([]WebSocketEvent, len(c.wsEvents)-n)
		copy(surviving, c.wsEvents[n:])
		c.wsEvents = surviving
		// Keep wsAddedAt and wsEvents in sync
		nAt := n
		if nAt > len(c.wsAddedAt) {
			nAt = len(c.wsAddedAt)
		}
		survivingAt := make([]time.Time, len(c.wsAddedAt)-nAt)
		copy(survivingAt, c.wsAddedAt[nAt:])
		c.wsAddedAt = survivingAt
		evicted += n
	}

	if c.calcTotalMemory() > limit && len(c.enhancedActions) > 0 {
		n := max(len(c.enhancedActions)/denominator, 1)
		// Clamp n to actual array length to prevent panic
		if n > len(c.enhancedActions) {
			n = len(c.enhancedActions)
		}
		surviving := make([]EnhancedAction, len(c.enhancedActions)-n)
		copy(surviving, c.enhancedActions[n:])
		c.enhancedActions = surviving
		// Keep actionAddedAt and enhancedActions in sync
		nAt := n
		if nAt > len(c.actionAddedAt) {
			nAt = len(c.actionAddedAt)
		}
		survivingAt := make([]time.Time, len(c.actionAddedAt)-nAt)
		copy(survivingAt, c.actionAddedAt[nAt:])
		c.actionAddedAt = survivingAt
		evicted += n
	}

	c.mem.lastEvictionTime = time.Now()
	c.mem.totalEvictions++
	c.mem.evictedEntries += evicted
}

// evictCritical clears ALL buffers and enters minimal mode.
// Uses nil assignment instead of [:0] reslicing so the GC can reclaim backing arrays.
func (c *Capture) evictCritical() {
	evicted := len(c.wsEvents) + len(c.networkBodies) + len(c.enhancedActions)

	c.wsEvents = nil
	c.wsAddedAt = nil
	c.wsMemoryTotal = 0
	c.networkBodies = nil
	c.networkAddedAt = nil
	c.nbMemoryTotal = 0
	c.enhancedActions = nil
	c.actionAddedAt = nil

	c.mem.minimalMode = true
	c.mem.lastEvictionTime = time.Now()
	c.mem.totalEvictions++
	c.mem.evictedEntries += evicted
}

// ============================================
// Memory-Exceeded Check
// ============================================

// IsMemoryExceeded checks if memory is over the hard limit (acquires lock).
// Uses simulated memory if set (for testing), otherwise checks real buffer memory.
func (c *Capture) IsMemoryExceeded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isMemoryExceeded()
}

// isMemoryExceeded is the internal version (caller must hold lock)
func (c *Capture) isMemoryExceeded() bool {
	if c.mem.simulatedMemory > 0 {
		return c.mem.simulatedMemory > memoryHardLimit
	}
	return c.calcTotalMemory() > memoryHardLimit
}

// ============================================
// Public Memory Accessors
// ============================================

// GetTotalBufferMemory returns the sum of all buffer memory usage
func (c *Capture) GetTotalBufferMemory() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.calcTotalMemory()
}

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

// ============================================
// Periodic Memory Check
// ============================================

// checkMemoryAndEvict runs the memory check and eviction (called periodically).
// This is a safety net - the primary enforcement happens on ingest.
func (c *Capture) checkMemoryAndEvict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enforceMemory()
}

// StartMemoryEnforcement starts the background goroutine that periodically checks memory.
// Returns a stop function that can be called to terminate the goroutine.
func (c *Capture) StartMemoryEnforcement() func() {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(memoryCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.checkMemoryAndEvict()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

// SetMemoryUsage sets simulated memory usage for testing
func (c *Capture) SetMemoryUsage(bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mem.simulatedMemory = bytes
}
