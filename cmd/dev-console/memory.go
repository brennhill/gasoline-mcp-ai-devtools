// memory.go — Memory enforcement with soft/hard limits and eviction policies.
// Periodic goroutine checks memory usage and evicts oldest entries when the
// soft limit (20MB) is exceeded. Hard limit (50MB) triggers aggressive eviction.
// Design: Each buffer (logs, network, websocket, actions) is evicted
// independently based on its contribution to total memory pressure.
package main

import (
	"math"
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

// calcWSMemory approximates memory usage of WS buffer (caller must hold lock)
func (c *Capture) calcWSMemory() int64 {
	var total int64
	for i := range c.wsEvents {
		size := int64(len(c.wsEvents[i].Data)) + wsEventOverhead
		// Cap at max int64 to prevent overflow
		if total > math.MaxInt64-size {
			return math.MaxInt64
		}
		total += size
	}
	return total
}

// calcNBMemory approximates memory usage of network bodies buffer (caller must hold lock)
func (c *Capture) calcNBMemory() int64 {
	var total int64
	for i := range c.networkBodies {
		size := int64(len(c.networkBodies[i].RequestBody)+len(c.networkBodies[i].ResponseBody)) + networkBodyOverhead
		// Cap at max int64 to prevent overflow
		if total > math.MaxInt64-size {
			return math.MaxInt64
		}
		total += size
	}
	return total
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

// evictSoft removes oldest 25% from each buffer (prioritizing network bodies)
func (c *Capture) evictSoft() {
	c.evictBuffers(4, memorySoftLimit)
}

// evictHard removes oldest 50% from each buffer (prioritizing network bodies)
func (c *Capture) evictHard() {
	c.evictBuffers(2, memoryHardLimit)
}

// evictBuffers removes oldest 1/denominator entries from each buffer in priority order
// (network bodies → WS events → actions), stopping early if memory drops below limit.
// Survivors are copied to new slices so the GC can reclaim the old backing arrays.
func (c *Capture) evictBuffers(denominator int, limit int64) {
	var evicted int

	// Network bodies first (largest per entry)
	if len(c.networkBodies) > 0 {
		n := max(len(c.networkBodies)/denominator, 1)
		// Clamp n to actual array length to prevent panic
		if n > len(c.networkBodies) {
			n = len(c.networkBodies)
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
	c.networkBodies = nil
	c.networkAddedAt = nil
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
