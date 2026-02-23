// Purpose: Implements buffer clear/reset operations across capture telemetry stores.
// Why: Supports controlled memory recovery and explicit operator reset workflows.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"
)

// BufferClearCounts reports per-buffer clear impact.
//
// Invariants:
// - Counts reflect pre-clear item totals from one critical section.
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

// ClearNetworkBuffers resets network telemetry buffers and related counters.
//
// Invariants:
// - network buffers and their monotonic counters are reset together under c.mu.
func (c *Capture) ClearNetworkBuffers() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		NetworkWaterfall: len(c.nw.entries),
		NetworkBodies:    len(c.networkBodies),
	}

	// Clear network waterfall buffer
	c.nw.entries = make([]NetworkWaterfallEntry, 0, c.nw.capacity)

	// Clear network bodies buffer and reset memory tracking
	c.networkBodies = make([]NetworkBody, 0)
	c.networkAddedAt = make([]time.Time, 0)
	c.networkTotalAdded = 0
	c.networkErrorTotalAdded = 0
	c.nbMemoryTotal = 0

	return counts
}

// ClearWebSocketBuffers resets websocket events and live-connection tracking.
//
// Invariants:
// - wsEvents/wsAddedAt/wsMemoryTotal/wsTotalAdded are reset atomically.
func (c *Capture) ClearWebSocketBuffers() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		WebSocketEvents: len(c.wsEvents),
		WebSocketStatus: len(c.ws.connections),
	}

	// Clear WebSocket events buffer
	c.wsEvents = make([]WebSocketEvent, 0)
	c.wsAddedAt = make([]time.Time, 0)
	c.wsTotalAdded = 0
	c.wsMemoryTotal = 0

	// Clear WebSocket connections map
	c.ws.connections = make(map[string]*connectionState)
	c.ws.connOrder = make([]string, 0)

	return counts
}

// ClearActionBuffer resets action telemetry ring and counters.
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
//
// Failure semantics:
// - Returns number of entries removed; 0 when already empty.
func (c *Capture) ClearExtensionLogs() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.elb.logs)
	c.elb.logs = make([]ExtensionLog, 0)

	return count
}

// ClearAll resets all capture-owned in-memory telemetry state.
//
// Invariants:
// - Runs under one c.mu critical section to avoid partially-cleared mixed state.
func (c *Capture) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wsEvents = make([]WebSocketEvent, 0)
	c.wsAddedAt = make([]time.Time, 0)
	c.wsMemoryTotal = 0
	c.networkBodies = make([]NetworkBody, 0)
	c.networkAddedAt = make([]time.Time, 0)
	c.nbMemoryTotal = 0
	c.nw.entries = make([]NetworkWaterfallEntry, 0, c.nw.capacity)
	c.enhancedActions = make([]EnhancedAction, 0)
	c.actionAddedAt = make([]time.Time, 0)
	c.ws.connections = make(map[string]*connectionState)
	c.ws.closedConns = make([]WebSocketClosedConnection, 0)
	c.ws.connOrder = make([]string, 0)
	c.ext.activeTestIDs = make(map[string]bool)

	// Reset performance data
	c.perf.snapshots = make(map[string]PerformanceSnapshot)
	c.perf.snapshotOrder = make([]string, 0)
	c.perf.baselines = make(map[string]PerformanceBaseline)
	c.perf.baselineOrder = make([]string, 0)
	c.perf.beforeSnapshots = make(map[string]PerformanceSnapshot)
}
