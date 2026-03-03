package capture

import "time"

// GetNetworkTimestamps returns a copy of the network body timestamps
func (c *Capture) GetNetworkTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneTimes(c.buffers.networkAddedAt)
}

// GetWebSocketTimestamps returns a copy of the WebSocket event timestamps
func (c *Capture) GetWebSocketTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneTimes(c.buffers.wsAddedAt)
}

// GetActionTimestamps returns a copy of the action timestamps
func (c *Capture) GetActionTimestamps() []time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneTimes(c.buffers.actionAddedAt)
}

func cloneTimes(src []time.Time) []time.Time {
	if len(src) == 0 {
		return []time.Time{}
	}
	out := make([]time.Time, len(src))
	copy(out, src)
	return out
}

// GetNetworkBodies returns a copy of the network bodies slice (thread-safe)
func (c *Capture) GetNetworkBodies() []NetworkBody {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.buffers.networkBodies) == 0 {
		return []NetworkBody{}
	}

	out := make([]NetworkBody, len(c.buffers.networkBodies))
	copy(out, c.buffers.networkBodies)
	return out
}

// GetAllWebSocketEvents returns a copy of all WebSocket events slice (thread-safe)
func (c *Capture) GetAllWebSocketEvents() []WebSocketEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.buffers.wsEvents) == 0 {
		return []WebSocketEvent{}
	}

	out := make([]WebSocketEvent, len(c.buffers.wsEvents))
	copy(out, c.buffers.wsEvents)
	return out
}

// GetAllEnhancedActions returns a copy of all enhanced actions slice (thread-safe)
func (c *Capture) GetAllEnhancedActions() []EnhancedAction {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.buffers.enhancedActions) == 0 {
		return []EnhancedAction{}
	}

	out := make([]EnhancedAction, len(c.buffers.enhancedActions))
	copy(out, c.buffers.enhancedActions)
	return out
}
