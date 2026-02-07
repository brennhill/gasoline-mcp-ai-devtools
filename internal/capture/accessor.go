// accessor.go — Public accessor methods for Capture buffer state.
// Provides safe access to monotonic counters and timestamps without exposing the mutex.
package capture

import "time"

// GetNetworkTotalAdded returns the monotonic total of network bodies ever added
func (c *Capture) GetNetworkTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkTotalAdded
}

// GetWebSocketTotalAdded returns the monotonic total of WebSocket events ever added
func (c *Capture) GetWebSocketTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.wsTotalAdded
}

// GetActionTotalAdded returns the monotonic total of actions ever added
func (c *Capture) GetActionTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actionTotalAdded
}

// GetNetworkTimestamps returns a copy of the network body timestamps
func (c *Capture) GetNetworkTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.networkAddedAt) == 0 {
		return []time.Time{}
	}

	copy := make([]time.Time, len(c.networkAddedAt))
	for i, t := range c.networkAddedAt {
		copy[i] = t
	}
	return copy
}

// GetWebSocketTimestamps returns a copy of the WebSocket event timestamps
func (c *Capture) GetWebSocketTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.wsAddedAt) == 0 {
		return []time.Time{}
	}

	copy := make([]time.Time, len(c.wsAddedAt))
	for i, t := range c.wsAddedAt {
		copy[i] = t
	}
	return copy
}

// GetActionTimestamps returns a copy of the action timestamps
func (c *Capture) GetActionTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.actionAddedAt) == 0 {
		return []time.Time{}
	}

	copy := make([]time.Time, len(c.actionAddedAt))
	for i, t := range c.actionAddedAt {
		copy[i] = t
	}
	return copy
}

// GetCaptureSnapshot returns a snapshot of buffer state with proper locking
type CaptureSnapshot struct {
	NetworkTotalAdded int64
	WebSocketTotalAdded int64
	ActionTotalAdded int64
	NetworkCount    int
	WebSocketCount  int
	ActionCount     int
}

// GetSnapshot returns a thread-safe snapshot of the capture buffer state
func (c *Capture) GetSnapshot() CaptureSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CaptureSnapshot{
		NetworkTotalAdded:   c.networkTotalAdded,
		WebSocketTotalAdded: c.wsTotalAdded,
		ActionTotalAdded:    c.actionTotalAdded,
		NetworkCount:        len(c.networkBodies),
		WebSocketCount:      len(c.wsEvents),
		ActionCount:         len(c.enhancedActions),
	}
}

// GetNetworkBodies returns a copy of the network bodies slice (thread-safe)
func (c *Capture) GetNetworkBodies() []NetworkBody {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.networkBodies) == 0 {
		return []NetworkBody{}
	}

	copy := make([]NetworkBody, len(c.networkBodies))
	for i, body := range c.networkBodies {
		// Create a shallow copy of each entry (maps are reference types)
		bodyCopy := body
		copy[i] = bodyCopy
	}
	return copy
}

// GetAllWebSocketEvents returns a copy of all WebSocket events slice (thread-safe)
func (c *Capture) GetAllWebSocketEvents() []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.wsEvents) == 0 {
		return []WebSocketEvent{}
	}

	copy := make([]WebSocketEvent, len(c.wsEvents))
	for i, evt := range c.wsEvents {
		evtCopy := evt
		copy[i] = evtCopy
	}
	return copy
}

// GetAllEnhancedActions returns a copy of all enhanced actions slice (thread-safe)
func (c *Capture) GetAllEnhancedActions() []EnhancedAction {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.enhancedActions) == 0 {
		return []EnhancedAction{}
	}

	copy := make([]EnhancedAction, len(c.enhancedActions))
	for i, action := range c.enhancedActions {
		actionCopy := action
		copy[i] = actionCopy
	}
	return copy
}

// GetClientRegistry returns the client registry (thread-safe)
func (c *Capture) GetClientRegistry() ClientRegistry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientRegistry
}

// HealthSnapshot contains health information about capture state
type HealthSnapshot struct {
	WebSocketCount      int
	NetworkBodyCount    int
	ActionCount         int
	ConnectionCount     int
	LastPollTime        time.Time
	ExtensionSession    string
	SessionChangedTime  time.Time
	PilotEnabled        bool
	CircuitOpen         bool
	WindowEventCount    int
	MemoryBytes         int64
	CircuitReason       string
	CircuitOpenedTime   time.Time
	PendingQueryCount   int
	QueryResultCount    int
	ActiveTestIDCount   int
	QueryTimeout        time.Duration
}

// GetHealthSnapshot returns a snapshot of capture health state for /health endpoint
func (c *Capture) GetHealthSnapshot() HealthSnapshot {
	// Get sub-struct state (own locks) before acquiring c.mu
	circuitOpen, circuitReason, circuitOpenedAt, windowEventCount := c.circuit.GetState()
	querySnap := c.qd.GetSnapshot()

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Compute memory inline — we already hold c.mu.RLock, so calling
	// getMemoryForCircuit() would risk reentrant lock deadlock.
	var memBytes int64
	if c.mem.simulatedMemory > 0 {
		memBytes = c.mem.simulatedMemory
	} else {
		memBytes = c.calcTotalMemory()
	}

	return HealthSnapshot{
		WebSocketCount:      len(c.wsEvents),
		NetworkBodyCount:    len(c.networkBodies),
		ActionCount:         len(c.enhancedActions),
		ConnectionCount:     len(c.connections),
		LastPollTime:        c.ext.lastPollAt,
		ExtensionSession:    c.ext.extensionSession,
		SessionChangedTime:  c.ext.sessionChangedAt,
		PilotEnabled:        c.ext.pilotEnabled,
		CircuitOpen:         circuitOpen,
		WindowEventCount:    windowEventCount,
		MemoryBytes:         memBytes,
		CircuitReason:       circuitReason,
		CircuitOpenedTime:   circuitOpenedAt,
		PendingQueryCount:   querySnap.PendingQueryCount,
		QueryResultCount:    querySnap.QueryResultCount,
		ActiveTestIDCount:   len(c.ext.activeTestIDs),
		QueryTimeout:        querySnap.QueryTimeout,
	}
}

// LogHTTPDebugEntry logs an HTTP debug entry. Delegates to DebugLogger (own lock).
func (c *Capture) LogHTTPDebugEntry(entry HTTPDebugEntry) {
	c.debug.LogHTTPDebugEntry(entry)
}

// GetHTTPDebugLog returns a copy of the HTTP debug log. Delegates to DebugLogger (own lock).
func (c *Capture) GetHTTPDebugLog() []HTTPDebugEntry {
	return c.debug.GetHTTPDebugLog()
}

// AddPerformanceSnapshots stores performance snapshots from the extension.
// Snapshots are keyed by URL with LRU eviction (max 100 entries).
func (c *Capture) AddPerformanceSnapshots(snapshots []PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	const maxSnapshots = 100

	for _, snapshot := range snapshots {
		key := snapshot.URL
		if key == "" {
			continue
		}

		// Check if this URL already exists
		_, exists := c.perf.snapshots[key]
		if !exists {
			// Add to order tracking
			c.perf.snapshotOrder = append(c.perf.snapshotOrder, key)
		}

		// Store snapshot
		c.perf.snapshots[key] = snapshot

		// Evict oldest if over capacity
		for len(c.perf.snapshots) > maxSnapshots && len(c.perf.snapshotOrder) > 0 {
			oldestKey := c.perf.snapshotOrder[0]
			c.perf.snapshotOrder = c.perf.snapshotOrder[1:]
			delete(c.perf.snapshots, oldestKey)
		}
	}
}

// GetPerformanceSnapshots returns all stored performance snapshots (thread-safe)
func (c *Capture) GetPerformanceSnapshots() []PerformanceSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.perf.snapshots) == 0 {
		return []PerformanceSnapshot{}
	}

	result := make([]PerformanceSnapshot, 0, len(c.perf.snapshots))
	for _, snapshot := range c.perf.snapshots {
		result = append(result, snapshot)
	}
	return result
}

// GetPerformanceSnapshotByURL returns a specific snapshot by URL key (thread-safe).
func (c *Capture) GetPerformanceSnapshotByURL(url string) (PerformanceSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap, ok := c.perf.snapshots[url]
	return snap, ok
}

// StoreBeforeSnapshot stores a performance snapshot keyed by correlation_id
// for later perf_diff computation. Max 50 entries with oldest eviction.
func (c *Capture) StoreBeforeSnapshot(correlationID string, snapshot PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.beforeSnapshots[correlationID] = snapshot
	// Evict oldest if over capacity (simple: just cap size)
	if len(c.perf.beforeSnapshots) > 50 {
		for k := range c.perf.beforeSnapshots {
			delete(c.perf.beforeSnapshots, k)
			break // delete one
		}
	}
}

// GetAndDeleteBeforeSnapshot retrieves and removes a before-snapshot by correlation_id.
// Consume-on-read: the snapshot is deleted after retrieval to prevent memory leaks.
func (c *Capture) GetAndDeleteBeforeSnapshot(correlationID string) (PerformanceSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	snap, ok := c.perf.beforeSnapshots[correlationID]
	if ok {
		delete(c.perf.beforeSnapshots, correlationID)
	}
	return snap, ok
}

