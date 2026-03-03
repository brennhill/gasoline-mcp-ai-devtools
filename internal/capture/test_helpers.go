// Purpose: Provides test-only capture mutation helpers for deterministic unit/integration setup.
// Why: Enables focused tests without exposing unsafe mutation primitives in production APIs.
// Docs: docs/features/feature/self-testing/index.md

package capture

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

// AddNetworkBodiesForTest adds network bodies directly to the buffer (TEST ONLY)
// Normal production code should use HTTP handlers
func (c *Capture) AddNetworkBodiesForTest(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, body := range bodies {
		c.buffers.networkBodies = append(c.buffers.networkBodies, body)
		c.buffers.networkAddedAt = append(c.buffers.networkAddedAt, now)
		c.buffers.networkTotalAdded++
		if body.Status >= 400 {
			c.buffers.networkErrorTotalAdded++
		}
	}
}

// AddWebSocketEventsForTest adds WebSocket events directly to the buffer (TEST ONLY)
func (c *Capture) AddWebSocketEventsForTest(events []WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, event := range events {
		c.buffers.wsEvents = append(c.buffers.wsEvents, event)
		c.buffers.wsAddedAt = append(c.buffers.wsAddedAt, now)
		c.buffers.wsTotalAdded++
	}
}

// AddEnhancedActionsForTest adds enhanced actions directly to the buffer (TEST ONLY)
func (c *Capture) AddEnhancedActionsForTest(actions []EnhancedAction) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, action := range actions {
		c.buffers.enhancedActions = append(c.buffers.enhancedActions, action)
		c.buffers.actionAddedAt = append(c.buffers.actionAddedAt, now)
		c.buffers.actionTotalAdded++
	}
}

// SetPilotEnabled sets the pilot enabled state (TEST ONLY)
func (c *Capture) SetPilotEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.pilotEnabled = enabled
	c.extensionState.pilotStatusKnown = true
	c.extensionState.pilotUpdatedAt = time.Now()
	c.extensionState.pilotSource = PilotSourceTestHelper
}

// SetPilotUnknownForTest resets pilot to startup-uncertain state (TEST ONLY).
func (c *Capture) SetPilotUnknownForTest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.pilotEnabled = false
	c.extensionState.pilotStatusKnown = false
	c.extensionState.pilotUpdatedAt = time.Time{}
	c.extensionState.pilotSource = PilotSourceAssumedStartup
}

// SetTrackingStatusForTest sets the tracked tab URL and ID (TEST ONLY)
func (c *Capture) SetTrackingStatusForTest(tabID int, tabURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackingEnabled = true
	c.extensionState.trackedTabID = tabID
	c.extensionState.trackedTabURL = tabURL
	c.extensionState.trackingUpdated = time.Now()
}

// SetClientRegistryForTest sets the client registry (TEST ONLY)
func (c *Capture) SetClientRegistryForTest(reg ClientRegistry) {
	c.SetClientRegistry(reg)
}

// SetWSParallelMismatchForTest sets up mismatched wsEvents/wsAddedAt arrays (TEST ONLY)
// Adds extraEvents additional wsEvents entries beyond the wsAddedAt length to simulate mismatch.
func (c *Capture) SetWSParallelMismatchForTest(extraEvents int, extraAddedAt int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	// Add extra wsEvents entries (without matching wsAddedAt)
	for i := 0; i < extraEvents; i++ {
		c.buffers.wsEvents = append(c.buffers.wsEvents, WebSocketEvent{
			Event: "message",
			Data:  "extra-event",
			ID:    "ws-extra",
		})
	}
	// Add extra wsAddedAt entries (without matching wsEvents)
	for i := 0; i < extraAddedAt; i++ {
		c.buffers.wsAddedAt = append(c.buffers.wsAddedAt, now)
	}
}

// GetWSLengthsForTest returns wsEvents and wsAddedAt lengths (TEST ONLY)
func (c *Capture) GetWSLengthsForTest() (events int, addedAt int, memoryTotal int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buffers.wsEvents), len(c.buffers.wsAddedAt), c.buffers.wsMemoryTotal
}

// SimulateExtensionConnectForTest marks the extension as connected by
// setting lastSyncSeen to now. Thread-safe (operates on the instance, not a global).
func (c *Capture) SimulateExtensionConnectForTest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.lastSyncSeen = time.Now()
	c.extensionState.lastExtensionConnected = true
}

// SimulateExtensionDisconnectForTest marks the extension as disconnected by
// setting lastSyncSeen far in the past. Thread-safe (operates on the instance, not a global).
func (c *Capture) SimulateExtensionDisconnectForTest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.lastSyncSeen = time.Now().Add(-1 * time.Hour)
}

// SetTabStatusForTest sets the tracked tab status (TEST ONLY).
// Valid values: "loading", "complete".
func (c *Capture) SetTabStatusForTest(status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.tabStatus = status
}

// SetCSPStatusForTest sets the CSP restriction state (TEST ONLY)
func (c *Capture) SetCSPStatusForTest(restricted bool, level string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.cspRestricted = restricted
	c.extensionState.cspLevel = level
}

// GetLastPendingQuery returns the most recently created pending query (TEST ONLY)
// Returns nil if no queries exist.
func (c *Capture) GetLastPendingQuery() *queries.PendingQuery {
	return c.queryDispatcher.GetLastPendingQuery()
}

// SimulateSyncForTest simulates a /sync connection from the extension,
// triggering lifecycle callbacks (extension_connected) like a real sync would.
// This is faster than calling HandleSync because it avoids the 5-second long-poll.
// Thread-safe (TEST ONLY).
func (c *Capture) SimulateSyncForTest(extSessionID string, clientID string) {
	now := time.Now()
	req := SyncRequest{
		ExtSessionID: extSessionID,
		Settings: &SyncSettings{
			PilotEnabled:    false,
			TrackingEnabled: true,
			TrackedTabID:    1,
		},
	}
	state := c.updateSyncConnectionState(req, clientID, now)

	if !state.wasConnected || state.isReconnect {
		c.emitLifecycleEvent("extension_connected", map[string]any{
			"ext_session_id":     state.extSessionID,
			"is_reconnect":       state.isReconnect,
			"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
		})
	}
}
