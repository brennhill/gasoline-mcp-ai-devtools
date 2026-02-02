// test_helpers.go — Test helper methods for setting up test data
// These methods are ONLY for tests and bypass normal ingestion flow
//go:build !production

package capture

import "time"

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
