// test_helpers.go — Test helper methods for setting up test data
// These methods are ONLY for tests and bypass normal ingestion flow
//go:build !production

package capture

import (
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// AddNetworkBodiesForTest adds network bodies directly to the buffer (TEST ONLY)
// Normal production code should use HTTP handlers
func (c *Capture) AddNetworkBodiesForTest(bodies []NetworkBody) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, body := range bodies {
		c.networkBodies = append(c.networkBodies, body)
		c.networkAddedAt = append(c.networkAddedAt, now)
		c.networkTotalAdded++
	}
}

// AddWebSocketEventsForTest adds WebSocket events directly to the buffer (TEST ONLY)
func (c *Capture) AddWebSocketEventsForTest(events []WebSocketEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, event := range events {
		c.wsEvents = append(c.wsEvents, event)
		c.wsAddedAt = append(c.wsAddedAt, now)
		c.wsTotalAdded++
	}
}

// AddEnhancedActionsForTest adds enhanced actions directly to the buffer (TEST ONLY)
func (c *Capture) AddEnhancedActionsForTest(actions []EnhancedAction) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for _, action := range actions {
		c.enhancedActions = append(c.enhancedActions, action)
		c.actionAddedAt = append(c.actionAddedAt, now)
		c.actionTotalAdded++
	}
}

// SetPilotEnabled sets the pilot enabled state (TEST ONLY)
func (c *Capture) SetPilotEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ext.pilotEnabled = enabled
}

// SetTrackingStatusForTest sets the tracked tab URL and ID (TEST ONLY)
func (c *Capture) SetTrackingStatusForTest(tabID int, tabURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ext.trackingEnabled = true
	c.ext.trackedTabID = tabID
	c.ext.trackedTabURL = tabURL
	c.ext.trackingUpdated = time.Now()
}

// SetClientRegistryForTest sets the client registry (TEST ONLY)
func (c *Capture) SetClientRegistryForTest(reg ClientRegistry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientRegistry = reg
}

// SetWSParallelMismatchForTest sets up mismatched wsEvents/wsAddedAt arrays (TEST ONLY)
// Adds extraEvents additional wsEvents entries beyond the wsAddedAt length to simulate mismatch.
func (c *Capture) SetWSParallelMismatchForTest(extraEvents int, extraAddedAt int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	// Add extra wsEvents entries (without matching wsAddedAt)
	for i := 0; i < extraEvents; i++ {
		c.wsEvents = append(c.wsEvents, WebSocketEvent{
			Event: "message",
			Data:  "extra-event",
			ID:    "ws-extra",
		})
	}
	// Add extra wsAddedAt entries (without matching wsEvents)
	for i := 0; i < extraAddedAt; i++ {
		c.wsAddedAt = append(c.wsAddedAt, now)
	}
}

// GetWSLengthsForTest returns wsEvents and wsAddedAt lengths (TEST ONLY)
func (c *Capture) GetWSLengthsForTest() (events int, addedAt int, memoryTotal int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.wsEvents), len(c.wsAddedAt), c.wsMemoryTotal
}

// GetLastPendingQuery returns the most recently created pending query (TEST ONLY)
// Returns nil if no queries exist.
func (c *Capture) GetLastPendingQuery() *queries.PendingQuery {
	c.qd.mu.Lock()
	defer c.qd.mu.Unlock()
	if len(c.qd.pendingQueries) == 0 {
		return nil
	}
	last := c.qd.pendingQueries[len(c.qd.pendingQueries)-1]
	return &queries.PendingQuery{
		Type:          last.query.Type,
		Params:        last.query.Params,
		TabID:         last.query.TabID,
		CorrelationID: last.query.CorrelationID,
	}
}
