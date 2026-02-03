package capture

import (
	"testing"
	"time"
)

// TestClearNetworkBuffers verifies clearing network_waterfall and network_bodies buffers.
func TestClearNetworkBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add network data directly to buffers
	capture.mu.Lock()
	capture.networkWaterfall = []NetworkWaterfallEntry{
		{URL: "https://example.com/1"},
		{URL: "https://example.com/2"},
	}
	capture.mu.Unlock()

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://example.com/1"},
	})

	// Verify data exists
	capture.mu.RLock()
	initialWaterfall := len(capture.networkWaterfall)
	initialBodies := len(capture.networkBodies)
	capture.mu.RUnlock()

	if initialWaterfall != 2 {
		t.Fatalf("Expected 2 waterfall entries, got %d", initialWaterfall)
	}
	if initialBodies != 1 {
		t.Fatalf("Expected 1 body entry, got %d", initialBodies)
	}

	// Clear
	counts := capture.ClearNetworkBuffers()

	// Verify counts
	if counts.NetworkWaterfall != 2 {
		t.Errorf("Expected NetworkWaterfall count = 2, got %d", counts.NetworkWaterfall)
	}
	if counts.NetworkBodies != 1 {
		t.Errorf("Expected NetworkBodies count = 1, got %d", counts.NetworkBodies)
	}
	if counts.Total() != 3 {
		t.Errorf("Expected total = 3, got %d", counts.Total())
	}

	// Verify buffers empty
	capture.mu.RLock()
	if len(capture.networkWaterfall) != 0 {
		t.Errorf("Expected networkWaterfall to be empty, got %d entries", len(capture.networkWaterfall))
	}
	if len(capture.networkBodies) != 0 {
		t.Errorf("Expected networkBodies to be empty, got %d entries", len(capture.networkBodies))
	}
	if capture.networkTotalAdded != 0 {
		t.Errorf("Expected networkTotalAdded = 0, got %d", capture.networkTotalAdded)
	}
	capture.mu.RUnlock()
}

// TestClearWebSocketBuffers verifies clearing websocket_events and websocket_status buffers.
func TestClearWebSocketBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add WS events
	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "conn1", Direction: "outgoing", Data: "test"},
		{ID: "conn1", Direction: "incoming", Data: "response"},
	})

	// Add WS connections
	capture.mu.Lock()
	capture.connections["conn1"] = &connectionState{id: "conn1", url: "ws://localhost", state: "open"}
	capture.mu.Unlock()

	// Clear
	counts := capture.ClearWebSocketBuffers()

	// Verify counts
	if counts.WebSocketEvents != 2 {
		t.Errorf("Expected WebSocketEvents count = 2, got %d", counts.WebSocketEvents)
	}
	if counts.WebSocketStatus != 1 {
		t.Errorf("Expected WebSocketStatus count = 1, got %d", counts.WebSocketStatus)
	}

	// Verify buffers empty
	capture.mu.RLock()
	if len(capture.wsEvents) != 0 {
		t.Errorf("Expected wsEvents to be empty, got %d entries", len(capture.wsEvents))
	}
	if len(capture.connections) != 0 {
		t.Errorf("Expected connections to be empty, got %d entries", len(capture.connections))
	}
	capture.mu.RUnlock()
}

// TestClearActionBuffer verifies clearing enhancedActions buffer.
func TestClearActionBuffer(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add actions
	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1738238000000},
		{Type: "input", Timestamp: 1738238001000},
	})

	// Clear
	counts := capture.ClearActionBuffer()

	// Verify counts
	if counts.Actions != 2 {
		t.Errorf("Expected Actions count = 2, got %d", counts.Actions)
	}

	// Verify buffer empty
	capture.mu.RLock()
	if len(capture.enhancedActions) != 0 {
		t.Errorf("Expected enhancedActions to be empty, got %d entries", len(capture.enhancedActions))
	}
	capture.mu.RUnlock()
}

// TestClearExtensionLogs verifies clearing extension logs buffer.
func TestClearExtensionLogs(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add extension logs
	capture.mu.Lock()
	capture.extensionLogs = append(capture.extensionLogs, ExtensionLog{Level: "debug", Message: "ext log", Timestamp: time.Now()})
	capture.mu.Unlock()

	// Clear
	count := capture.ClearExtensionLogs()

	// Verify count
	if count != 1 {
		t.Errorf("Expected ExtensionLogs count = 1, got %d", count)
	}

	// Verify buffer empty
	capture.mu.RLock()
	if len(capture.extensionLogs) != 0 {
		t.Errorf("Expected extensionLogs to be empty, got %d entries", len(capture.extensionLogs))
	}
	capture.mu.RUnlock()
}

// TestClearAllCapture verifies clearing all capture buffers at once.
func TestClearAllCapture(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add data to all capture buffers
	capture.mu.Lock()
	capture.networkWaterfall = []NetworkWaterfallEntry{{URL: "test"}}
	capture.mu.Unlock()

	capture.AddWebSocketEvents([]WebSocketEvent{{ID: "conn1", Data: "test"}})
	capture.AddEnhancedActions([]EnhancedAction{{Type: "click", Timestamp: 1738238000000}})

	// Clear all
	capture.ClearAll()

	// Verify all buffers empty
	capture.mu.RLock()
	defer capture.mu.RUnlock()

	if len(capture.networkWaterfall) != 0 {
		t.Error("Expected networkWaterfall to be empty")
	}
	if len(capture.wsEvents) != 0 {
		t.Error("Expected wsEvents to be empty")
	}
	if len(capture.enhancedActions) != 0 {
		t.Error("Expected enhancedActions to be empty")
	}
}

// TestClearEmptyBuffers verifies clearing empty buffers returns zero counts without error.
func TestClearEmptyBuffers(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Clear empty network buffers
	counts := capture.ClearNetworkBuffers()

	// Should return zero counts, not error
	if counts.NetworkWaterfall != 0 {
		t.Errorf("Expected NetworkWaterfall count = 0, got %d", counts.NetworkWaterfall)
	}
	if counts.NetworkBodies != 0 {
		t.Errorf("Expected NetworkBodies count = 0, got %d", counts.NetworkBodies)
	}
	if counts.Total() != 0 {
		t.Errorf("Expected total = 0, got %d", counts.Total())
	}
}
