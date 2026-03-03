// Purpose: Implements buffer clear/reset operations across capture telemetry stores.
// Why: Supports controlled memory recovery and explicit operator reset workflows.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"
)

// BufferClearCounts is an alias to canonical definition in internal/types/buffer.go.
// Total() method is inherited through the type alias.
type BufferClearCounts = types.BufferClearCounts

// ClearNetworkBuffers resets network telemetry buffers and related counters.
//
// Invariants:
// - network buffers and their monotonic counters are reset together under c.mu.
func (c *Capture) ClearNetworkBuffers() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		NetworkWaterfall: len(c.networkWaterfall.entries),
		NetworkBodies:    len(c.buffers.networkBodies),
	}

	// Clear network waterfall buffer
	c.networkWaterfall.entries = make([]NetworkWaterfallEntry, 0, c.networkWaterfall.capacity)

	// Clear network bodies buffer and reset memory tracking
	c.buffers.networkBodies = make([]NetworkBody, 0)
	c.buffers.networkAddedAt = make([]time.Time, 0)
	c.buffers.networkTotalAdded = 0
	c.buffers.networkErrorTotalAdded = 0
	c.buffers.networkBodyMemoryTotal = 0

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
		WebSocketEvents: len(c.buffers.wsEvents),
		WebSocketStatus: len(c.wsConnections.connections),
	}

	// Clear WebSocket events buffer
	c.buffers.wsEvents = make([]WebSocketEvent, 0)
	c.buffers.wsAddedAt = make([]time.Time, 0)
	c.buffers.wsTotalAdded = 0
	c.buffers.wsMemoryTotal = 0

	// Clear WebSocket connections map
	c.wsConnections.connections = make(map[string]*connectionState)
	c.wsConnections.connOrder = make([]string, 0)

	return counts
}

// ClearActionBuffer resets action telemetry ring and counters.
func (c *Capture) ClearActionBuffer() BufferClearCounts {
	c.mu.Lock()
	defer c.mu.Unlock()

	counts := BufferClearCounts{
		Actions: len(c.buffers.enhancedActions),
	}

	// Clear actions buffer
	c.buffers.enhancedActions = make([]EnhancedAction, 0)
	c.buffers.actionAddedAt = make([]time.Time, 0)
	c.buffers.actionTotalAdded = 0

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

	count := len(c.extensionLogs.logs)
	c.extensionLogs.logs = make([]ExtensionLog, 0)

	return count
}

// ClearAll resets all capture-owned in-memory telemetry state.
//
// Invariants:
// - Runs under one c.mu critical section to avoid partially-cleared mixed state.
func (c *Capture) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffers.wsEvents = make([]WebSocketEvent, 0)
	c.buffers.wsAddedAt = make([]time.Time, 0)
	c.buffers.wsMemoryTotal = 0
	c.buffers.networkBodies = make([]NetworkBody, 0)
	c.buffers.networkAddedAt = make([]time.Time, 0)
	c.buffers.networkBodyMemoryTotal = 0
	c.networkWaterfall.entries = make([]NetworkWaterfallEntry, 0, c.networkWaterfall.capacity)
	c.buffers.enhancedActions = make([]EnhancedAction, 0)
	c.buffers.actionAddedAt = make([]time.Time, 0)
	c.wsConnections.connections = make(map[string]*connectionState)
	c.wsConnections.closedConns = make([]WebSocketClosedConnection, 0)
	c.wsConnections.connOrder = make([]string, 0)
	c.extensionState.activeTestIDs = make(map[string]bool)

	// Reset performance data
	c.perf.snapshots = make(map[string]PerformanceSnapshot)
	c.perf.snapshotOrder = make([]string, 0)
	c.perf.baselines = make(map[string]PerformanceBaseline)
	c.perf.baselineOrder = make([]string, 0)
	c.perf.beforeSnapshots = make(map[string]PerformanceSnapshot)
}
