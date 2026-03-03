// Purpose: Defines a focused buffer container for high-volume telemetry ring buffers.
// Why: Keeps websocket/network/action buffer state modular inside Capture.

package capture

import "time"

// BufferStore owns the in-memory ring buffers used for event/body/action capture.
// Access is synchronized by Capture.mu (this type has no independent lock).
type BufferStore struct {
	// WebSocket event buffer state.
	wsEvents      []WebSocketEvent
	wsAddedAt     []time.Time
	wsTotalAdded  int64
	wsMemoryTotal int64

	// Network body buffer state.
	networkBodies          []NetworkBody
	networkAddedAt         []time.Time
	networkTotalAdded      int64
	networkErrorTotalAdded int64
	networkBodyMemoryTotal int64

	// Enhanced action buffer state.
	enhancedActions []EnhancedAction
	actionAddedAt   []time.Time
	actionTotalAdded int64
}

func newBufferStore() BufferStore {
	return BufferStore{
		wsEvents:        make([]WebSocketEvent, 0, MaxWSEvents),
		networkBodies:   make([]NetworkBody, 0, MaxNetworkBodies),
		enhancedActions: make([]EnhancedAction, 0, MaxEnhancedActions),
	}
}
