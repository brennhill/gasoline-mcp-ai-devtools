// buffer_clear.go — Buffer-specific clearing methods
// Provides granular control over clearing different buffer types:
// network (waterfall + bodies), websocket (events + status), actions, logs.
// Design: Destructive and immediate. No undo. Returns counts of cleared items.
package main

import (
	"time"
)

// BufferClearCounts holds counts of cleared items from each buffer.
type BufferClearCounts struct {
	NetworkWaterfall int `json:"network_waterfall,omitempty"`
	NetworkBodies    int `json:"network_bodies,omitempty"`
	WebSocketEvents  int `json:"websocket_events,omitempty"`
	WebSocketStatus  int `json:"websocket_status,omitempty"`
	Actions          int `json:"actions,omitempty"`
	Logs             int `json:"logs,omitempty"`
	ExtensionLogs    int `json:"extension_logs,omitempty"`
}

// Total returns sum of all cleared items.
func (c *BufferClearCounts) Total() int {
	return c.NetworkWaterfall + c.NetworkBodies + c.WebSocketEvents +
		c.WebSocketStatus + c.Actions + c.Logs + c.ExtensionLogs
}

// ClearNetworkBuffers clears network_waterfall and network_bodies buffers.
func (c *Capture) ClearNetworkBuffers() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		NetworkWaterfall: len(c.networkWaterfall),
		NetworkBodies:    len(c.networkBodies),
	}

	// Clear network waterfall buffer
	c.networkWaterfall = make([]NetworkWaterfallEntry, 0, c.networkWaterfallCapacity)

	// Clear network bodies buffer and reset memory tracking
	c.networkBodies = make([]NetworkBody, 0)
	c.networkAddedAt = make([]time.Time, 0)
	c.networkTotalAdded = 0
	c.nbMemoryTotal = 0

	return counts
}

// ClearWebSocketBuffers clears websocket_events and websocket_status buffers.
func (c *Capture) ClearWebSocketBuffers() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		WebSocketEvents: len(c.wsEvents),
		WebSocketStatus: len(c.connections),
	}

	// Clear WebSocket events buffer
	c.wsEvents = make([]WebSocketEvent, 0)
	c.wsAddedAt = make([]time.Time, 0)
	c.wsTotalAdded = 0
	c.wsMemoryTotal = 0

	// Clear WebSocket connections map
	c.connections = make(map[string]*connectionState)
	c.connOrder = make([]string, 0)

	return counts
}

// ClearActionBuffer clears enhancedActions buffer.
func (c *Capture) ClearActionBuffer() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		Actions: len(c.enhancedActions),
	}

	// Clear actions buffer
	c.enhancedActions = make([]EnhancedAction, 0)
	c.actionAddedAt = make([]time.Time, 0)
	c.actionTotalAdded = 0

	return counts
}

// ClearLogBuffers clears console logs and extension logs (Server + Capture).
func ClearLogBuffers(s *Server, c *Capture) BufferClearCounts {
	// Clear server console logs
	s.mu.Lock()
	logCount := len(s.entries)
	s.entries = make([]LogEntry, 0)
	s.logAddedAt = make([]time.Time, 0)
	s.logTotalAdded = 0
	s.mu.Unlock()

	// Clear capture extension logs
	c.mu.Lock()
	extLogCount := len(c.extensionLogs)
	c.extensionLogs = make([]ExtensionLog, 0)
	c.mu.Unlock()

	return BufferClearCounts{
		Logs:          logCount,
		ExtensionLogs: extLogCount,
	}
}

// ClearAllBuffers clears all buffers (network, websocket, actions, logs).
func ClearAllBuffers(s *Server, c *Capture) BufferClearCounts {
	networkCounts := c.ClearNetworkBuffers()
	wsCounts := c.ClearWebSocketBuffers()
	actionCounts := c.ClearActionBuffer()
	logCounts := ClearLogBuffers(s, c)

	return BufferClearCounts{
		NetworkWaterfall: networkCounts.NetworkWaterfall,
		NetworkBodies:    networkCounts.NetworkBodies,
		WebSocketEvents:  wsCounts.WebSocketEvents,
		WebSocketStatus:  wsCounts.WebSocketStatus,
		Actions:          actionCounts.Actions,
		Logs:             logCounts.Logs,
		ExtensionLogs:    logCounts.ExtensionLogs,
	}
}
