// Purpose: Defines a focused buffer container for high-volume telemetry ring buffers.
// Why: Keeps websocket/network/action buffer state modular inside Capture.

package capture

import (
	"fmt"
	"os"
	"time"
)

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

func (s *BufferStore) repairActionParallelArrays() {
	if len(s.enhancedActions) == len(s.actionAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: enhancedActions/actionAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(s.enhancedActions), len(s.actionAddedAt))
	minLen := min(len(s.enhancedActions), len(s.actionAddedAt))
	s.enhancedActions = s.enhancedActions[:minLen]
	s.actionAddedAt = s.actionAddedAt[:minLen]
}

func (s *BufferStore) appendEnhancedActions(actions []EnhancedAction, now time.Time) bool {
	s.repairActionParallelArrays()
	s.actionTotalAdded += int64(len(actions))
	hasNavigation := false
	for i := range actions {
		s.enhancedActions = append(s.enhancedActions, actions[i])
		s.actionAddedAt = append(s.actionAddedAt, now)
		if actions[i].Type == "navigation" {
			hasNavigation = true
		}
	}
	if len(s.enhancedActions) > MaxEnhancedActions {
		keep := len(s.enhancedActions) - MaxEnhancedActions
		newActions := make([]EnhancedAction, MaxEnhancedActions)
		copy(newActions, s.enhancedActions[keep:])
		s.enhancedActions = newActions
		newAddedAt := make([]time.Time, MaxEnhancedActions)
		copy(newAddedAt, s.actionAddedAt[keep:])
		s.actionAddedAt = newAddedAt
	}
	return hasNavigation
}

func (s *BufferStore) appendNetworkBodies(bodies []NetworkBody, testIDs []string, now time.Time) {
	s.repairNetworkParallelArrays()
	s.networkTotalAdded += int64(len(bodies))
	for i := range bodies {
		if bodies[i].Status >= 400 {
			s.networkErrorTotalAdded++
		}
		bodies[i].TestIDs = testIDs
		detectAndSetBinaryFormat(&bodies[i])
		s.networkBodies = append(s.networkBodies, bodies[i])
		s.networkAddedAt = append(s.networkAddedAt, now)
		s.networkBodyMemoryTotal += nbEntryMemory(&bodies[i])
	}
	s.evictNetworkByCount()
	s.evictNetworkForMemory()
}

func (s *BufferStore) appendWebSocketEvents(events []WebSocketEvent, testIDs []string, now time.Time, onEvent func(WebSocketEvent)) {
	s.repairWebSocketParallelArrays()
	s.wsTotalAdded += int64(len(events))
	for i := range events {
		events[i].TestIDs = testIDs
		detectWSBinaryFormat(&events[i])
		if onEvent != nil {
			onEvent(events[i])
		}
		s.wsEvents = append(s.wsEvents, events[i])
		s.wsAddedAt = append(s.wsAddedAt, now)
		s.wsMemoryTotal += wsEventMemory(&events[i])
	}
	s.evictWebSocketByCount()
	s.evictWebSocketForMemory()
}

func (s *BufferStore) repairNetworkParallelArrays() {
	if len(s.networkBodies) == len(s.networkAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: networkBodies/networkAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(s.networkBodies), len(s.networkAddedAt))
	minLen := min(len(s.networkBodies), len(s.networkAddedAt))
	s.networkBodies = s.networkBodies[:minLen]
	s.networkAddedAt = s.networkAddedAt[:minLen]
}

func (s *BufferStore) evictNetworkByCount() {
	if len(s.networkBodies) <= MaxNetworkBodies {
		return
	}
	keep := len(s.networkBodies) - MaxNetworkBodies
	for j := 0; j < keep; j++ {
		s.networkBodyMemoryTotal -= nbEntryMemory(&s.networkBodies[j])
	}
	newBodies := make([]NetworkBody, MaxNetworkBodies)
	copy(newBodies, s.networkBodies[keep:])
	s.networkBodies = newBodies
	newAddedAt := make([]time.Time, MaxNetworkBodies)
	copy(newAddedAt, s.networkAddedAt[keep:])
	s.networkAddedAt = newAddedAt
}

func (s *BufferStore) evictNetworkForMemory() {
	excess := s.networkBodyMemoryTotal - nbBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(s.networkBodies) && excess > 0 {
		entryMem := nbEntryMemory(&s.networkBodies[drop])
		excess -= entryMem
		s.networkBodyMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]NetworkBody, len(s.networkBodies)-drop)
	copy(surviving, s.networkBodies[drop:])
	s.networkBodies = surviving
	if len(s.networkAddedAt) >= drop {
		survivingAt := make([]time.Time, len(s.networkAddedAt)-drop)
		copy(survivingAt, s.networkAddedAt[drop:])
		s.networkAddedAt = survivingAt
	}
}

func (s *BufferStore) repairWebSocketParallelArrays() {
	if len(s.wsEvents) == len(s.wsAddedAt) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gasoline] WARNING: wsEvents/wsAddedAt length mismatch: %d != %d (recovering by truncating)\n",
		len(s.wsEvents), len(s.wsAddedAt))
	minLen := min(len(s.wsEvents), len(s.wsAddedAt))
	s.wsMemoryTotal = 0
	for i := 0; i < minLen; i++ {
		s.wsMemoryTotal += wsEventMemory(&s.wsEvents[i])
	}
	s.wsEvents = s.wsEvents[:minLen]
	s.wsAddedAt = s.wsAddedAt[:minLen]
}

func (s *BufferStore) evictWebSocketByCount() {
	if len(s.wsEvents) <= MaxWSEvents {
		return
	}
	drop := len(s.wsEvents) - MaxWSEvents
	for j := 0; j < drop; j++ {
		s.wsMemoryTotal -= wsEventMemory(&s.wsEvents[j])
	}
	newEvents := make([]WebSocketEvent, MaxWSEvents)
	copy(newEvents, s.wsEvents[drop:])
	s.wsEvents = newEvents
	newAddedAt := make([]time.Time, MaxWSEvents)
	copy(newAddedAt, s.wsAddedAt[drop:])
	s.wsAddedAt = newAddedAt
}

func (s *BufferStore) evictWebSocketForMemory() {
	s.repairWebSocketParallelArrays()
	excess := s.wsMemoryTotal - wsBufferMemoryLimit
	if excess <= 0 {
		return
	}
	drop := 0
	for drop < len(s.wsEvents) && excess > 0 {
		entryMem := wsEventMemory(&s.wsEvents[drop])
		excess -= entryMem
		s.wsMemoryTotal -= entryMem
		drop++
	}
	surviving := make([]WebSocketEvent, len(s.wsEvents)-drop)
	copy(surviving, s.wsEvents[drop:])
	s.wsEvents = surviving
	if len(s.wsAddedAt) >= drop {
		survivingAt := make([]time.Time, len(s.wsAddedAt)-drop)
		copy(survivingAt, s.wsAddedAt[drop:])
		s.wsAddedAt = survivingAt
	}
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
