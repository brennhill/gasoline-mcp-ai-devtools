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
func (v *Capture) GetMemoryStatus() MemoryStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()

	wsMem := v.calcWSMemory()
	nbMem := v.calcNBMemory()
	actionMem := v.calcActionMemory()

	return MemoryStatus{
		TotalBytes:     wsMem + nbMem + actionMem,
		WebSocketBytes: wsMem,
		NetworkBytes:   nbMem,
		ActionsBytes:   actionMem,
		SoftLimit:      memorySoftLimit,
		HardLimit:      memoryHardLimit,
		CriticalLimit:  memoryCriticalLimit,
		MinimalMode:    v.mem.minimalMode,
		TotalEvictions: v.mem.totalEvictions,
		EvictedEntries: v.mem.evictedEntries,
	}
}

// ============================================
// Memory Calculation
// ============================================

// calcTotalMemory returns total memory across all buffers (caller must hold lock)
func (v *Capture) calcTotalMemory() int64 {
	return v.calcWSMemory() + v.calcNBMemory() + v.calcActionMemory()
}

// calcWSMemory approximates memory usage of WS buffer (caller must hold lock)
func (v *Capture) calcWSMemory() int64 {
	var total int64
	for i := range v.wsEvents {
		total += int64(len(v.wsEvents[i].Data)) + wsEventOverhead
	}
	return total
}

// calcNBMemory approximates memory usage of network bodies buffer (caller must hold lock)
func (v *Capture) calcNBMemory() int64 {
	var total int64
	for i := range v.networkBodies {
		total += int64(len(v.networkBodies[i].RequestBody)+len(v.networkBodies[i].ResponseBody)) + networkBodyOverhead
	}
	return total
}

// calcActionMemory approximates memory usage of enhanced actions buffer (caller must hold lock)
func (v *Capture) calcActionMemory() int64 {
	return int64(len(v.enhancedActions)) * actionMemoryFixed
}

// ============================================
// Effective Capacities
// ============================================

// effectiveWSCapacity returns the current WS buffer capacity (halved in minimal mode)
// caller must hold lock
func (v *Capture) effectiveWSCapacity() int {
	if v.mem.minimalMode {
		return maxWSEvents / 2
	}
	return maxWSEvents
}

// effectiveNBCapacity returns the current network body buffer capacity (halved in minimal mode)
// caller must hold lock
func (v *Capture) effectiveNBCapacity() int {
	if v.mem.minimalMode {
		return maxNetworkBodies / 2
	}
	return maxNetworkBodies
}

// effectiveActionCapacity returns the current action buffer capacity (halved in minimal mode)
// caller must hold lock
func (v *Capture) effectiveActionCapacity() int {
	if v.mem.minimalMode {
		return maxEnhancedActions / 2
	}
	return maxEnhancedActions
}

// ============================================
// Memory Enforcement (Eviction)
// ============================================

// enforceMemory checks memory thresholds and evicts as needed (caller must hold lock).
// This is the primary enforcement point, called on every ingest operation.
func (v *Capture) enforceMemory() {
	// Respect cooldown
	if !v.mem.lastEvictionTime.IsZero() && time.Since(v.mem.lastEvictionTime) < evictionCooldown {
		return
	}

	totalMem := v.calcTotalMemory()

	// Check critical limit first (100MB)
	if totalMem > memoryCriticalLimit {
		v.evictCritical()
		return
	}

	// Check hard limit (50MB)
	if totalMem > memoryHardLimit {
		v.evictHard()
		return
	}

	// Check soft limit (20MB)
	if totalMem > memorySoftLimit {
		v.evictSoft()
		return
	}
}

// evictSoft removes oldest 25% from each buffer (prioritizing network bodies)
func (v *Capture) evictSoft() {
	var evicted int

	// Network bodies first (largest per entry)
	if len(v.networkBodies) > 0 {
		removeCount := len(v.networkBodies) / 4
		if removeCount == 0 {
			removeCount = 1
		}
		v.networkBodies = v.networkBodies[removeCount:]
		v.networkAddedAt = v.networkAddedAt[removeCount:]
		evicted += removeCount
	}

	// Check if still above soft limit
	if v.calcTotalMemory() > memorySoftLimit {
		// WS events
		if len(v.wsEvents) > 0 {
			removeCount := len(v.wsEvents) / 4
			if removeCount == 0 {
				removeCount = 1
			}
			v.wsEvents = v.wsEvents[removeCount:]
			v.wsAddedAt = v.wsAddedAt[removeCount:]
			evicted += removeCount
		}
	}

	// Check if still above soft limit
	if v.calcTotalMemory() > memorySoftLimit {
		// Enhanced actions
		if len(v.enhancedActions) > 0 {
			removeCount := len(v.enhancedActions) / 4
			if removeCount == 0 {
				removeCount = 1
			}
			v.enhancedActions = v.enhancedActions[removeCount:]
			v.actionAddedAt = v.actionAddedAt[removeCount:]
			evicted += removeCount
		}
	}

	v.mem.lastEvictionTime = time.Now()
	v.mem.totalEvictions++
	v.mem.evictedEntries += evicted
}

// evictHard removes oldest 50% from each buffer (prioritizing network bodies)
func (v *Capture) evictHard() {
	var evicted int

	// Network bodies first (largest per entry)
	if len(v.networkBodies) > 0 {
		removeCount := len(v.networkBodies) / 2
		if removeCount == 0 {
			removeCount = 1
		}
		v.networkBodies = v.networkBodies[removeCount:]
		v.networkAddedAt = v.networkAddedAt[removeCount:]
		evicted += removeCount
	}

	// Check if still above hard limit
	if v.calcTotalMemory() > memoryHardLimit {
		// WS events
		if len(v.wsEvents) > 0 {
			removeCount := len(v.wsEvents) / 2
			if removeCount == 0 {
				removeCount = 1
			}
			v.wsEvents = v.wsEvents[removeCount:]
			v.wsAddedAt = v.wsAddedAt[removeCount:]
			evicted += removeCount
		}
	}

	// Check if still above hard limit
	if v.calcTotalMemory() > memoryHardLimit {
		// Enhanced actions
		if len(v.enhancedActions) > 0 {
			removeCount := len(v.enhancedActions) / 2
			if removeCount == 0 {
				removeCount = 1
			}
			v.enhancedActions = v.enhancedActions[removeCount:]
			v.actionAddedAt = v.actionAddedAt[removeCount:]
			evicted += removeCount
		}
	}

	v.mem.lastEvictionTime = time.Now()
	v.mem.totalEvictions++
	v.mem.evictedEntries += evicted
}

// evictCritical clears ALL buffers and enters minimal mode
func (v *Capture) evictCritical() {
	evicted := len(v.wsEvents) + len(v.networkBodies) + len(v.enhancedActions)

	v.wsEvents = v.wsEvents[:0]
	v.wsAddedAt = v.wsAddedAt[:0]
	v.networkBodies = v.networkBodies[:0]
	v.networkAddedAt = v.networkAddedAt[:0]
	v.enhancedActions = v.enhancedActions[:0]
	v.actionAddedAt = v.actionAddedAt[:0]

	v.mem.minimalMode = true
	v.mem.lastEvictionTime = time.Now()
	v.mem.totalEvictions++
	v.mem.evictedEntries += evicted
}

// ============================================
// Memory-Exceeded Check
// ============================================

// IsMemoryExceeded checks if memory is over the hard limit (acquires lock).
// Uses simulated memory if set (for testing), otherwise checks real buffer memory.
func (v *Capture) IsMemoryExceeded() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isMemoryExceeded()
}

// isMemoryExceeded is the internal version (caller must hold lock)
func (v *Capture) isMemoryExceeded() bool {
	if v.mem.simulatedMemory > 0 {
		return v.mem.simulatedMemory > memoryHardLimit
	}
	return v.calcTotalMemory() > memoryHardLimit
}

// ============================================
// Public Memory Accessors
// ============================================

// GetTotalBufferMemory returns the sum of all buffer memory usage
func (v *Capture) GetTotalBufferMemory() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.calcTotalMemory()
}

// GetWebSocketBufferMemory returns approximate memory usage of WS buffer
func (v *Capture) GetWebSocketBufferMemory() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.calcWSMemory()
}

// GetNetworkBodiesBufferMemory returns approximate memory usage of network bodies buffer
func (v *Capture) GetNetworkBodiesBufferMemory() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.calcNBMemory()
}

// ============================================
// Periodic Memory Check
// ============================================

// checkMemoryAndEvict runs the memory check and eviction (called periodically).
// This is a safety net - the primary enforcement happens on ingest.
func (v *Capture) checkMemoryAndEvict() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.enforceMemory()
}

// StartMemoryEnforcement starts the background goroutine that periodically checks memory.
// Returns a stop function that can be called to terminate the goroutine.
func (v *Capture) StartMemoryEnforcement() func() {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(memoryCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				v.checkMemoryAndEvict()
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}

// SetMemoryUsage sets simulated memory usage for testing
func (v *Capture) SetMemoryUsage(bytes int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.mem.simulatedMemory = bytes
}
