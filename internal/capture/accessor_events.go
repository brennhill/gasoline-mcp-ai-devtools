// Purpose: Provides thread-safe accessors for network, WebSocket, action, and log timestamps.
// Why: Separates timestamp-based event accessors from count and performance accessors.
package capture

import "time"

// GetNetworkTimestamps returns a copy of the network body timestamps
func (c *Capture) GetNetworkTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkTimestamps()
}

// GetWebSocketTimestamps returns a copy of the WebSocket event timestamps
func (c *Capture) GetWebSocketTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketTimestamps()
}

// GetActionTimestamps returns a copy of the action timestamps
func (c *Capture) GetActionTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.actionTimestamps()
}

// GetNetworkBodies returns a copy of the network bodies slice (thread-safe)
func (c *Capture) GetNetworkBodies() []NetworkBody {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.networkBodiesCopy()
}

// GetAllWebSocketEvents returns a copy of all WebSocket events slice (thread-safe)
func (c *Capture) GetAllWebSocketEvents() []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.webSocketEventsCopy()
}

// GetAllEnhancedActions returns a copy of all enhanced actions slice (thread-safe)
func (c *Capture) GetAllEnhancedActions() []EnhancedAction {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.buffers.enhancedActionsCopy()
}
