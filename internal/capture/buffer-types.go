// Purpose: Owns buffer-types.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// buffer-types.go — Ring buffer types for Capture composition.
// NetworkWaterfallBuffer, ExtensionLogBuffer, WSConnectionTracker.
package capture

// NetworkWaterfallBuffer groups network waterfall ring buffer fields.
// Protected by parent Capture.mu (no separate lock).
type NetworkWaterfallBuffer struct {
	entries  []NetworkWaterfallEntry // Ring buffer of PerformanceResourceTiming data
	capacity int                     // Configurable capacity (default DefaultNetworkWaterfallCapacity=1000)
}

// ExtensionLogBuffer groups extension log ring buffer fields.
// Protected by parent Capture.mu (no separate lock).
type ExtensionLogBuffer struct {
	logs []ExtensionLog // Ring buffer of extension internal logs (max MaxExtensionLogs=500)
}

// WSConnectionTracker groups WebSocket connection tracking fields.
// Protected by parent Capture.mu (no separate lock).
type WSConnectionTracker struct {
	connections map[string]*connectionState // Active WS connections by ID (max 20 total). LRU eviction via connOrder.
	closedConns []WebSocketClosedConnection // Ring buffer of closed connections (max 10, maxClosedConns). Preserves history for a while.
	connOrder   []string                    // Insertion order for LRU eviction of active connections.
}
