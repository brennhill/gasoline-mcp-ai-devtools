// buffer_clear.go — Capture buffer-specific clearing methods.
// Provides granular control over clearing different Capture buffers:
// network (waterfall + bodies), websocket (events + status), actions.
package capture

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

// ClearExtensionLogs clears the extension logs buffer.
// This is a public accessor for clearing extension logs from outside the capture package.
func (c *Capture) ClearExtensionLogs() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.extensionLogs)
	c.extensionLogs = make([]ExtensionLog, 0)

	return count
}

// ClearAll resets all capture buffers atomically.
// Used for test boundary reset and CI/CD pipeline cleanup.
func (c *Capture) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wsEvents = make([]WebSocketEvent, 0)
	c.wsAddedAt = make([]time.Time, 0)
	c.wsMemoryTotal = 0
	c.networkBodies = make([]NetworkBody, 0)
	c.networkAddedAt = make([]time.Time, 0)
	c.nbMemoryTotal = 0
	c.networkWaterfall = make([]NetworkWaterfallEntry, 0, c.networkWaterfallCapacity)
	c.enhancedActions = make([]EnhancedAction, 0)
	c.actionAddedAt = make([]time.Time, 0)
	c.connections = make(map[string]*connectionState)
	c.closedConns = make([]WebSocketClosedConnection, 0)
	c.connOrder = make([]string, 0)
	c.ext.activeTestIDs = make(map[string]bool)

	// Reset performance data
	c.perf.snapshots = make(map[string]PerformanceSnapshot)
	c.perf.snapshotOrder = make([]string, 0)
	c.perf.baselines = make(map[string]PerformanceBaseline)
	c.perf.baselineOrder = make([]string, 0)
	c.perf.beforeSnapshots = make(map[string]PerformanceSnapshot)
}
