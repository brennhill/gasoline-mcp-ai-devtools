package capture

import "time"

// GetNetworkTotalAdded returns the monotonic total of network bodies ever added
func (c *Capture) GetNetworkTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkTotalAdded
}

// GetNetworkErrorTotalAdded returns the monotonic total of error network bodies ever added.
func (c *Capture) GetNetworkErrorTotalAdded() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkErrorTotalAdded
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

// CaptureSnapshot is an immutable point-in-time view of core ring-buffer counters.
//
// Invariants:
// - Counts and totals in one snapshot come from the same c.mu critical section.
type CaptureSnapshot struct {
	NetworkTotalAdded   int64
	WebSocketTotalAdded int64
	ActionTotalAdded    int64
	NetworkCount        int
	WebSocketCount      int
	ActionCount         int
}

// GetSnapshot returns a thread-safe capture counter snapshot.
//
// Failure semantics:
// - Snapshot can be stale immediately after return; callers should treat it as diagnostic-only.
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

// GetClientRegistry returns the client registry (thread-safe)
func (c *Capture) GetClientRegistry() ClientRegistry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clientRegistry
}

// HealthSnapshot aggregates capture + dispatcher + circuit health state.
//
// Invariants:
// - Subsystem snapshots (circuit/queries) are sampled before c.mu to avoid lock inversion.
type HealthSnapshot struct {
	WebSocketCount        int
	NetworkBodyCount      int
	ActionCount           int
	ConnectionCount       int
	LastPollTime          time.Time
	ExtSessionID          string
	ExtSessionChangedTime time.Time
	PilotEnabled          bool
	CircuitOpen           bool
	WindowEventCount      int
	CircuitReason         string
	CircuitOpenedTime     time.Time
	PendingQueryCount     int
	QueryResultCount      int
	ActiveTestIDCount     int
	QueryTimeout          time.Duration
}

// GetHealthSnapshot returns a lock-safe aggregate health view.
//
// Invariants:
// - Reads c.circuit/c.queryDispatcher first, then c.mu, preserving declared lock hierarchy.
func (c *Capture) GetHealthSnapshot() HealthSnapshot {
	// Get sub-struct state (own locks) before acquiring c.mu
	circuitOpen, circuitReason, circuitOpenedAt, windowEventCount := c.circuit.GetState()
	querySnap := c.queryDispatcher.GetSnapshot()

	c.mu.RLock()
	defer c.mu.RUnlock()

	return HealthSnapshot{
		WebSocketCount:        len(c.wsEvents),
		NetworkBodyCount:      len(c.networkBodies),
		ActionCount:           len(c.enhancedActions),
		ConnectionCount:       len(c.wsConnections.connections),
		LastPollTime:          c.extensionState.lastPollAt,
		ExtSessionID:          c.extensionState.extSessionID,
		ExtSessionChangedTime: c.extensionState.extSessionChangedAt,
		PilotEnabled:          c.extensionState.pilotEnabled,
		CircuitOpen:           circuitOpen,
		WindowEventCount:      windowEventCount,
		CircuitReason:         circuitReason,
		CircuitOpenedTime:     circuitOpenedAt,
		PendingQueryCount:     querySnap.PendingQueryCount,
		QueryResultCount:      querySnap.QueryResultCount,
		ActiveTestIDCount:     len(c.extensionState.activeTestIDs),
		QueryTimeout:          querySnap.QueryTimeout,
	}
}
