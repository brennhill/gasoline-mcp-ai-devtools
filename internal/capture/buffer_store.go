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
	enhancedActions  []EnhancedAction
	actionAddedAt    []time.Time
	actionTotalAdded int64
}

func newBufferStore() BufferStore {
	return BufferStore{
		wsEvents:        make([]WebSocketEvent, 0, MaxWSEvents),
		networkBodies:   make([]NetworkBody, 0, MaxNetworkBodies),
		enhancedActions: make([]EnhancedAction, 0, MaxEnhancedActions),
	}
}

func (s *BufferStore) networkTimestamps() []time.Time {
	return cloneTimes(s.networkAddedAt)
}

func (s *BufferStore) webSocketTimestamps() []time.Time {
	return cloneTimes(s.wsAddedAt)
}

func (s *BufferStore) actionTimestamps() []time.Time {
	return cloneTimes(s.actionAddedAt)
}

func (s *BufferStore) networkBodiesCopy() []NetworkBody {
	if len(s.networkBodies) == 0 {
		return []NetworkBody{}
	}
	out := make([]NetworkBody, len(s.networkBodies))
	copy(out, s.networkBodies)
	return out
}

func (s *BufferStore) webSocketEventsCopy() []WebSocketEvent {
	if len(s.wsEvents) == 0 {
		return []WebSocketEvent{}
	}
	out := make([]WebSocketEvent, len(s.wsEvents))
	copy(out, s.wsEvents)
	return out
}

func (s *BufferStore) enhancedActionsCopy() []EnhancedAction {
	if len(s.enhancedActions) == 0 {
		return []EnhancedAction{}
	}
	out := make([]EnhancedAction, len(s.enhancedActions))
	copy(out, s.enhancedActions)
	return out
}

func (s *BufferStore) clearNetworkBuffers() {
	s.networkBodies = make([]NetworkBody, 0)
	s.networkAddedAt = make([]time.Time, 0)
	s.networkTotalAdded = 0
	s.networkErrorTotalAdded = 0
	s.networkBodyMemoryTotal = 0
}

func (s *BufferStore) clearWebSocketBuffers() {
	s.wsEvents = make([]WebSocketEvent, 0)
	s.wsAddedAt = make([]time.Time, 0)
	s.wsTotalAdded = 0
	s.wsMemoryTotal = 0
}

func (s *BufferStore) clearActionBuffers() {
	s.enhancedActions = make([]EnhancedAction, 0)
	s.actionAddedAt = make([]time.Time, 0)
	s.actionTotalAdded = 0
}

func (s *BufferStore) clearAllEventBuffers() {
	s.wsEvents = make([]WebSocketEvent, 0)
	s.wsAddedAt = make([]time.Time, 0)
	s.wsMemoryTotal = 0
	s.networkBodies = make([]NetworkBody, 0)
	s.networkAddedAt = make([]time.Time, 0)
	s.networkBodyMemoryTotal = 0
	s.enhancedActions = make([]EnhancedAction, 0)
	s.actionAddedAt = make([]time.Time, 0)
}

func (s *BufferStore) networkTotal() int64 {
	return s.networkTotalAdded
}

func (s *BufferStore) networkErrorTotal() int64 {
	return s.networkErrorTotalAdded
}

func (s *BufferStore) webSocketTotal() int64 {
	return s.wsTotalAdded
}

func (s *BufferStore) actionTotal() int64 {
	return s.actionTotalAdded
}

func (s *BufferStore) networkCount() int {
	return len(s.networkBodies)
}

func (s *BufferStore) webSocketCount() int {
	return len(s.wsEvents)
}

func (s *BufferStore) actionCount() int {
	return len(s.enhancedActions)
}
