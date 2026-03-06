// Purpose: Implements buffer clear/reset operations across capture telemetry stores.
// Why: Supports controlled memory recovery and explicit operator reset workflows.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
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
		NetworkWaterfall: c.networkWaterfall.count(),
		NetworkBodies:    len(c.buffers.networkBodies),
	}

	// Clear network waterfall buffer.
	c.networkWaterfall.clear()

	// Clear network bodies buffer and reset memory tracking
	c.buffers.clearNetworkBuffers()

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
		WebSocketStatus: c.wsConnections.connectionCount(),
	}

	// Clear WebSocket events buffer
	c.buffers.clearWebSocketBuffers()

	// Clear WebSocket connection tracker.
	c.wsConnections.clear()

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
	c.buffers.clearActionBuffers()

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

	return c.extensionLogs.clear()
}

// ClearAll resets all capture-owned in-memory telemetry state.
//
// Invariants:
// - Runs under one c.mu critical section to avoid partially-cleared mixed state.
func (c *Capture) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffers.clearAllEventBuffers()
	c.networkWaterfall.clear()
	c.wsConnections.clear()
	c.extensionState.activeTestIDs = make(map[string]bool)

	// Reset performance data
	c.perf.clear()
}
