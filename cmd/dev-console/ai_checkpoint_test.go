package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ============================================
// Test Helpers
// ============================================

func setupCheckpointTest(t *testing.T) (*CheckpointManager, *Server, *V4Server) {
	t.Helper()
	server, _ := NewServer("", 1000)
	v4 := NewV4Server()
	cm := NewCheckpointManager(server, v4)
	return cm, server, v4
}

// addLogEntries is a helper to add console log entries with level and message
func addLogEntries(server *Server, entries ...LogEntry) {
	server.addEntries(entries)
}

// ============================================
// Test 1: Empty server → severity "clean"
// ============================================

func TestCheckpointEmptyServer(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Severity != "clean" {
		t.Errorf("Expected severity 'clean', got '%s'", resp.Severity)
	}
	if resp.Summary != "No significant changes." {
		t.Errorf("Expected summary 'No significant changes.', got '%s'", resp.Summary)
	}
	if resp.Console != nil {
		t.Error("Expected Console to be nil for empty server")
	}
	if resp.Network != nil {
		t.Error("Expected Network to be nil for empty server")
	}
	if resp.WebSocket != nil {
		t.Error("Expected WebSocket to be nil for empty server")
	}
	if resp.Actions != nil {
		t.Error("Expected Actions to be nil for empty server")
	}
}

// ============================================
// Test 2: New error logs after checkpoint
// ============================================

func TestCheckpointNewConsoleErrors(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// First call establishes the auto-checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add 3 error logs after checkpoint
	addLogEntries(server,
		LogEntry{"level": "error", "msg": "TypeError: Cannot read property 'x'", "source": "app.js:42"},
		LogEntry{"level": "error", "msg": "ReferenceError: foo is not defined", "source": "main.js:10"},
		LogEntry{"level": "error", "msg": "Network request failed", "source": "api.js:99"},
	)

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", resp.Severity)
	}
	if resp.Console == nil {
		t.Fatal("Expected Console diff to be present")
	}
	if len(resp.Console.Errors) != 3 {
		t.Errorf("Expected 3 console errors, got %d", len(resp.Console.Errors))
	}
	if resp.Console.TotalNew != 3 {
		t.Errorf("Expected TotalNew=3, got %d", resp.Console.TotalNew)
	}
}

// ============================================
// Test 3: Message deduplication by fingerprint
// ============================================

func TestCheckpointMessageDeduplication(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add same error 5 times (with different UUIDs that should normalize)
	addLogEntries(server,
		LogEntry{"level": "error", "msg": "Error loading user abc12345-def6-7890-abcd-ef1234567890", "source": "user.js:15"},
		LogEntry{"level": "error", "msg": "Error loading user def45678-abc1-2345-cdef-ab6789012345", "source": "user.js:15"},
		LogEntry{"level": "error", "msg": "Error loading user 11111111-2222-3333-4444-555555555555", "source": "user.js:15"},
		LogEntry{"level": "error", "msg": "Error loading user aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "source": "user.js:15"},
		LogEntry{"level": "error", "msg": "Error loading user 99999999-8888-7777-6666-555544443333", "source": "user.js:15"},
	)

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Console == nil {
		t.Fatal("Expected Console diff")
	}
	if len(resp.Console.Errors) != 1 {
		t.Errorf("Expected 1 deduplicated error entry, got %d", len(resp.Console.Errors))
	}
	if resp.Console.Errors[0].Count != 5 {
		t.Errorf("Expected count=5, got %d", resp.Console.Errors[0].Count)
	}
}

// ============================================
// Test 4: Network failure detection
// ============================================

func TestCheckpointNetworkFailure(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)

	// Add a successful request before checkpoint
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users", Status: 200, Method: "GET"},
	})

	// Establish checkpoint (which records known endpoint status)
	cm.GetChangesSince(GetChangesSinceParams{})

	// Now the endpoint fails
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users?page=2", Status: 500, Method: "GET"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", resp.Severity)
	}
	if resp.Network == nil {
		t.Fatal("Expected Network diff")
	}
	if len(resp.Network.Failures) != 1 {
		t.Errorf("Expected 1 failure, got %d", len(resp.Network.Failures))
	}
	if resp.Network.Failures[0].PreviousStatus != 200 {
		t.Errorf("Expected previous_status=200, got %d", resp.Network.Failures[0].PreviousStatus)
	}
	if resp.Network.Failures[0].Status != 500 {
		t.Errorf("Expected status=500, got %d", resp.Network.Failures[0].Status)
	}
	// URL path should be normalized (no query params)
	if resp.Network.Failures[0].Path != "/api/users" {
		t.Errorf("Expected path='/api/users', got '%s'", resp.Network.Failures[0].Path)
	}
}

// ============================================
// Test 5: New endpoint detection
// ============================================

func TestCheckpointNewEndpoint(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)

	// Establish checkpoint with known endpoints
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users", Status: 200, Method: "GET"},
	})
	cm.GetChangesSince(GetChangesSinceParams{})

	// New endpoint appears
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/orders?limit=10", Status: 200, Method: "GET"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff")
	}
	if len(resp.Network.NewEndpoints) != 1 {
		t.Errorf("Expected 1 new endpoint, got %d", len(resp.Network.NewEndpoints))
	}
	if resp.Network.NewEndpoints[0] != "/api/orders" {
		t.Errorf("Expected new endpoint '/api/orders', got '%s'", resp.Network.NewEndpoints[0])
	}
}

// ============================================
// Test 6: WebSocket disconnection
// ============================================

func TestCheckpointWebSocketDisconnection(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// WebSocket close event after checkpoint
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Event: "close", ID: "ws-1", URL: "wss://chat.example.com/ws", CloseCode: 1006, CloseReason: "abnormal"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Severity != "warning" {
		t.Errorf("Expected severity 'warning', got '%s'", resp.Severity)
	}
	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff")
	}
	if len(resp.WebSocket.Disconnections) != 1 {
		t.Errorf("Expected 1 disconnection, got %d", len(resp.WebSocket.Disconnections))
	}
	if resp.WebSocket.Disconnections[0].URL != "wss://chat.example.com/ws" {
		t.Errorf("Expected URL 'wss://chat.example.com/ws', got '%s'", resp.WebSocket.Disconnections[0].URL)
	}
	if resp.WebSocket.Disconnections[0].CloseCode != 1006 {
		t.Errorf("Expected close code 1006, got %d", resp.WebSocket.Disconnections[0].CloseCode)
	}
}

// ============================================
// Test 7: Auto-checkpoint advancement
// ============================================

func TestCheckpointAutoAdvancement(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// Add errors
	addLogEntries(server, LogEntry{"level": "error", "msg": "first error"})

	// First call sees the error
	resp1 := cm.GetChangesSince(GetChangesSinceParams{})
	if resp1.Severity != "error" {
		t.Errorf("First call: expected severity 'error', got '%s'", resp1.Severity)
	}

	// Second call with no new events sees nothing
	resp2 := cm.GetChangesSince(GetChangesSinceParams{})
	if resp2.Severity != "clean" {
		t.Errorf("Second call: expected severity 'clean', got '%s'", resp2.Severity)
	}
	if resp2.Summary != "No significant changes." {
		t.Errorf("Second call: expected 'No significant changes.', got '%s'", resp2.Summary)
	}
}

// ============================================
// Test 8: Named checkpoint stability
// ============================================

func TestCheckpointNamedStability(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// Add initial error
	addLogEntries(server, LogEntry{"level": "error", "msg": "initial error"})

	// Create named checkpoint
	cm.CreateCheckpoint("before_refactor")

	// Add more errors after the named checkpoint
	addLogEntries(server, LogEntry{"level": "error", "msg": "post-checkpoint error"})

	// Query the named checkpoint - should see the post-checkpoint error
	resp1 := cm.GetChangesSince(GetChangesSinceParams{Checkpoint: "before_refactor"})
	if resp1.Console == nil || len(resp1.Console.Errors) != 1 {
		t.Fatal("Expected 1 error from named checkpoint query")
	}

	// Add yet more errors
	addLogEntries(server, LogEntry{"level": "error", "msg": "another error"})

	// Query same named checkpoint again - should see BOTH post-checkpoint errors
	resp2 := cm.GetChangesSince(GetChangesSinceParams{Checkpoint: "before_refactor"})
	if resp2.Console == nil || len(resp2.Console.Errors) != 2 {
		t.Errorf("Expected 2 errors from named checkpoint, got %d", len(resp2.Console.Errors))
	}

	// Named checkpoint queries should NOT advance auto-checkpoint
	// Calling auto should still see changes from the beginning
	resp3 := cm.GetChangesSince(GetChangesSinceParams{})
	if resp3.Console == nil {
		t.Error("Auto-checkpoint should not have been advanced by named checkpoint query")
	}
}

// ============================================
// Test 9: Severity filtering - errors_only
// ============================================

func TestCheckpointSeverityFilterErrorsOnly(t *testing.T) {
	cm, server, v4 := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add a mix of warnings and errors
	addLogEntries(server,
		LogEntry{"level": "warn", "msg": "deprecation warning"},
		LogEntry{"level": "error", "msg": "fatal error"},
	)
	// Add a WebSocket disconnection (warning-level)
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Event: "close", ID: "ws-1", URL: "wss://example.com", CloseCode: 1000},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{Severity: "errors_only"})

	// Should only include error-level items
	if resp.Console != nil && len(resp.Console.Warnings) > 0 {
		t.Error("errors_only filter should exclude warnings")
	}
	if resp.Console == nil || len(resp.Console.Errors) != 1 {
		t.Error("errors_only filter should still include errors")
	}
	if resp.WebSocket != nil {
		t.Error("errors_only filter should exclude warning-level WebSocket disconnections")
	}
}

// ============================================
// Test 10: Include filtering
// ============================================

func TestCheckpointIncludeFiltering(t *testing.T) {
	cm, server, v4 := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add data in all categories
	addLogEntries(server, LogEntry{"level": "error", "msg": "error"})
	v4.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/test", Status: 500, Method: "GET"}})
	v4.AddWebSocketEvents([]WebSocketEvent{{Event: "close", ID: "ws-1", URL: "wss://x.com"}})
	v4.AddEnhancedActions([]EnhancedAction{{Type: "click", Timestamp: time.Now().UnixMilli()}})

	// Only include console and network
	resp := cm.GetChangesSince(GetChangesSinceParams{
		Include: []string{"console", "network"},
	})

	if resp.Console == nil {
		t.Error("Expected Console to be included")
	}
	if resp.Network == nil {
		t.Error("Expected Network to be included")
	}
	if resp.WebSocket != nil {
		t.Error("Expected WebSocket to be nil when not included")
	}
	if resp.Actions != nil {
		t.Error("Expected Actions to be nil when not included")
	}
}

// ============================================
// Test 11: Buffer overflow - best effort
// ============================================

func TestCheckpointBufferOverflow(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// Add some entries and establish checkpoint
	addLogEntries(server, LogEntry{"level": "info", "msg": "old entry"})
	cm.GetChangesSince(GetChangesSinceParams{})

	// Now flood the buffer past its max to cause rotation
	// Server maxEntries is 1000, so add more than that
	entries := make([]LogEntry, 1100)
	for i := range entries {
		entries[i] = LogEntry{"level": "error", "msg": "flood entry"}
	}
	addLogEntries(server, entries...)

	// The checkpoint index is now invalid (entries rotated past it)
	// Should fall back to returning all available entries, not panic
	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Severity != "error" {
		t.Errorf("Expected severity 'error' after overflow, got '%s'", resp.Severity)
	}
	// Should get entries (best-effort, up to max 50 per category)
	if resp.Console == nil {
		t.Error("Expected Console diff even after buffer overflow")
	}
}

// ============================================
// Test 12: Timestamp as checkpoint reference
// ============================================

func TestCheckpointTimestampReference(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// Add entries with timestamps
	addLogEntries(server, LogEntry{"level": "error", "msg": "before timestamp"})

	// Record a timestamp
	refTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	// Add entries after the timestamp
	addLogEntries(server, LogEntry{"level": "error", "msg": "after timestamp"})

	// Query using the timestamp
	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: refTime.Format(time.RFC3339Nano),
	})

	if resp.Console == nil {
		t.Fatal("Expected Console diff")
	}
	// Should only see the entry after the timestamp
	if resp.Console.TotalNew != 1 {
		t.Errorf("Expected 1 new entry after timestamp, got %d", resp.Console.TotalNew)
	}
}

// ============================================
// Test 13: Token count approximation
// ============================================

func TestCheckpointTokenCount(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	cm.GetChangesSince(GetChangesSinceParams{})
	addLogEntries(server, LogEntry{"level": "error", "msg": "test error message"})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	// Marshal response to JSON to check size
	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	expectedTokens := len(jsonBytes) / 4
	// Allow 20% tolerance
	if resp.TokenCount < expectedTokens*80/100 || resp.TokenCount > expectedTokens*120/100 {
		t.Errorf("Token count %d not close to expected %d (JSON size %d / 4)", resp.TokenCount, expectedTokens, len(jsonBytes))
	}
}

// ============================================
// Test 14: Max entries cap (50 per category)
// ============================================

func TestCheckpointMaxEntriesCap(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add 100 different errors (all unique to avoid dedup)
	entries := make([]LogEntry, 100)
	for i := range entries {
		entries[i] = LogEntry{"level": "error", "msg": fmt.Sprintf("unique error %d with some extra text", i), "source": fmt.Sprintf("file%d.js:%d", i, i)}
	}
	addLogEntries(server, entries...)

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Console == nil {
		t.Fatal("Expected Console diff")
	}
	if len(resp.Console.Errors) > 50 {
		t.Errorf("Expected max 50 error entries, got %d", len(resp.Console.Errors))
	}
	// TotalNew should still reflect the actual count
	if resp.Console.TotalNew != 100 {
		t.Errorf("Expected TotalNew=100, got %d", resp.Console.TotalNew)
	}
}

// ============================================
// Test 15: Concurrent access safety
// ============================================

func TestCheckpointConcurrency(t *testing.T) {
	cm, server, v4 := setupCheckpointTest(t)

	var wg sync.WaitGroup
	const goroutines = 10
	const iterations = 50

	// Concurrent writers
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				addLogEntries(server, LogEntry{"level": "error", "msg": "concurrent error"})
				v4.AddWebSocketEvents([]WebSocketEvent{{Event: "message", ID: "ws-1", Data: "test"}})
				v4.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/x", Status: 200, Method: "GET"}})
			}
		}(i)
	}

	// Concurrent readers (checkpoint operations)
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				cm.GetChangesSince(GetChangesSinceParams{})
				if j%10 == 0 {
					cm.CreateCheckpoint(fmt.Sprintf("test_%d_%d", id, j))
				}
			}
		}(i)
	}

	wg.Wait()
	// If we get here without panicking or deadlocking, the test passes
}

// ============================================
// Test 16: UUID/number fingerprint normalization
// ============================================

func TestCheckpointFingerprintNormalization(t *testing.T) {
	tests := []struct {
		name     string
		messages []string
		expected int // expected deduplicated count
	}{
		{
			name: "UUIDs normalized",
			messages: []string{
				"Error for user abc12345-def6-7890-abcd-ef1234567890",
				"Error for user 11111111-2222-3333-4444-555555555555",
			},
			expected: 1,
		},
		{
			name: "Large numbers normalized",
			messages: []string{
				"Request 12345 failed",
				"Request 67890 failed",
			},
			expected: 1,
		},
		{
			name: "Timestamps normalized",
			messages: []string{
				"Error at 2024-01-15T10:30:00.000Z in handler",
				"Error at 2024-06-20T14:45:30.123Z in handler",
			},
			expected: 1,
		},
		{
			name: "Short numbers NOT normalized",
			messages: []string{
				"Error code 42",
				"Error code 99",
			},
			expected: 2, // numbers < 4 digits are not normalized
		},
		{
			name: "Different messages stay separate",
			messages: []string{
				"TypeError: Cannot read property",
				"ReferenceError: x is not defined",
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, server, _ := setupCheckpointTest(t)
			cm.GetChangesSince(GetChangesSinceParams{})

			entries := make([]LogEntry, len(tt.messages))
			for i, msg := range tt.messages {
				entries[i] = LogEntry{"level": "error", "msg": msg, "source": "test.js:1"}
			}
			addLogEntries(server, entries...)

			resp := cm.GetChangesSince(GetChangesSinceParams{})
			if resp.Console == nil {
				t.Fatal("Expected Console diff")
			}
			if len(resp.Console.Errors) != tt.expected {
				t.Errorf("Expected %d deduplicated entries, got %d", tt.expected, len(resp.Console.Errors))
			}
		})
	}
}

// ============================================
// Test 17: URL path extraction strips query params
// ============================================

func TestCheckpointURLPathExtraction(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)

	// Multiple requests to same path with different query params
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users?page=1&limit=10", Status: 200, Method: "GET"},
	})
	cm.GetChangesSince(GetChangesSinceParams{})

	// Same endpoint with different query params now fails
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users?page=2&limit=20", Status: 500, Method: "GET"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff")
	}
	if len(resp.Network.Failures) != 1 {
		t.Errorf("Expected 1 failure, got %d", len(resp.Network.Failures))
	}
	if resp.Network.Failures[0].Path != "/api/users" {
		t.Errorf("Expected path '/api/users', got '%s'", resp.Network.Failures[0].Path)
	}
}

// ============================================
// Test 18: Severity hierarchy
// ============================================

func TestCheckpointSeverityHierarchy(t *testing.T) {
	// Test: error > warning > clean

	t.Run("error beats warning", func(t *testing.T) {
		cm, server, v4 := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		// Add both a warning and an error
		addLogEntries(server, LogEntry{"level": "warn", "msg": "warning"})
		addLogEntries(server, LogEntry{"level": "error", "msg": "error"})
		v4.AddWebSocketEvents([]WebSocketEvent{{Event: "close", ID: "ws-1", URL: "wss://x.com", CloseCode: 1006}})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Severity != "error" {
			t.Errorf("Expected 'error' severity, got '%s'", resp.Severity)
		}
	})

	t.Run("warning when no errors", func(t *testing.T) {
		cm, server, _ := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		addLogEntries(server, LogEntry{"level": "warn", "msg": "just a warning"})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Severity != "warning" {
			t.Errorf("Expected 'warning' severity, got '%s'", resp.Severity)
		}
	})

	t.Run("websocket disconnection is warning", func(t *testing.T) {
		cm, _, v4 := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		v4.AddWebSocketEvents([]WebSocketEvent{{Event: "close", ID: "ws-1", URL: "wss://x.com", CloseCode: 1006}})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Severity != "warning" {
			t.Errorf("Expected 'warning' severity for WS disconnect, got '%s'", resp.Severity)
		}
	})

	t.Run("network failure is error", func(t *testing.T) {
		cm, _, v4 := setupCheckpointTest(t)
		v4.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/test", Status: 200, Method: "GET"}})
		cm.GetChangesSince(GetChangesSinceParams{})

		v4.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/test", Status: 500, Method: "GET"}})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Severity != "error" {
			t.Errorf("Expected 'error' severity for network failure, got '%s'", resp.Severity)
		}
	})

	t.Run("clean when nothing notable", func(t *testing.T) {
		cm, server, _ := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		// Add only info-level entries (not errors or warnings)
		addLogEntries(server, LogEntry{"level": "info", "msg": "just info"})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Severity != "clean" {
			t.Errorf("Expected 'clean' severity for info-only, got '%s'", resp.Severity)
		}
	})
}

// ============================================
// Test 19: Summary formatting
// ============================================

func TestCheckpointSummaryFormatting(t *testing.T) {
	t.Run("console errors and network failures", func(t *testing.T) {
		cm, server, v4 := setupCheckpointTest(t)
		v4.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/a", Status: 200, Method: "GET"}})
		cm.GetChangesSince(GetChangesSinceParams{})

		addLogEntries(server,
			LogEntry{"level": "error", "msg": "err1"},
			LogEntry{"level": "error", "msg": "err2"},
		)
		v4.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/a", Status: 500, Method: "GET"}})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Summary != "2 new console error(s), 1 network failure(s)" {
			t.Errorf("Unexpected summary: '%s'", resp.Summary)
		}
	})

	t.Run("warnings only", func(t *testing.T) {
		cm, server, _ := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		addLogEntries(server,
			LogEntry{"level": "warn", "msg": "warn1"},
			LogEntry{"level": "warn", "msg": "warn2"},
			LogEntry{"level": "warn", "msg": "warn3"},
		)

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Summary != "3 new console warning(s)" {
			t.Errorf("Unexpected summary: '%s'", resp.Summary)
		}
	})

	t.Run("disconnections", func(t *testing.T) {
		cm, _, v4 := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		v4.AddWebSocketEvents([]WebSocketEvent{
			{Event: "close", ID: "ws-1", URL: "wss://a.com", CloseCode: 1006},
			{Event: "close", ID: "ws-2", URL: "wss://b.com", CloseCode: 1001},
		})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Summary != "2 websocket disconnection(s)" {
			t.Errorf("Unexpected summary: '%s'", resp.Summary)
		}
	})
}

// ============================================
// Additional: Checkpoint naming and limits
// ============================================

func TestCheckpointNamedLimit(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// Create 25 named checkpoints (limit is 20)
	for i := 0; i < 25; i++ {
		cm.CreateCheckpoint(fmt.Sprintf("checkpoint_%d", i))
	}

	// Should only have 20 named checkpoints
	count := cm.GetNamedCheckpointCount()
	if count != 20 {
		t.Errorf("Expected max 20 named checkpoints, got %d", count)
	}
}

func TestCheckpointNamingValidation(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// Valid names
	if err := cm.CreateCheckpoint("session_start"); err != nil {
		t.Errorf("Expected valid name to succeed: %v", err)
	}
	if err := cm.CreateCheckpoint("before_refactor"); err != nil {
		t.Errorf("Expected valid name to succeed: %v", err)
	}

	// Name too long (>50 chars)
	longName := "this_is_a_really_long_checkpoint_name_that_exceeds_fifty_characters_total"
	if err := cm.CreateCheckpoint(longName); err == nil {
		t.Error("Expected error for name > 50 chars")
	}

	// Empty name
	if err := cm.CreateCheckpoint(""); err == nil {
		t.Error("Expected error for empty name")
	}
}

// ============================================
// Additional: Message truncation
// ============================================

func TestCheckpointMessageTruncation(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add an error with a very long message (>200 chars)
	longMsg := ""
	for i := 0; i < 300; i++ {
		longMsg += "x"
	}
	addLogEntries(server, LogEntry{"level": "error", "msg": longMsg, "source": "test.js:1"})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Console == nil || len(resp.Console.Errors) == 0 {
		t.Fatal("Expected console error")
	}
	if len(resp.Console.Errors[0].Message) > 200 {
		t.Errorf("Expected message truncated to 200 chars, got %d", len(resp.Console.Errors[0].Message))
	}
}

// ============================================
// Additional: Diff response timestamps
// ============================================

func TestCheckpointDiffTimestamps(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)

	// First call sets the from timestamp
	cm.GetChangesSince(GetChangesSinceParams{})
	time.Sleep(10 * time.Millisecond)

	addLogEntries(server, LogEntry{"level": "error", "msg": "test"})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.From.IsZero() {
		t.Error("Expected From timestamp to be set")
	}
	if resp.To.IsZero() {
		t.Error("Expected To timestamp to be set")
	}
	if resp.DurationMs <= 0 {
		t.Errorf("Expected positive duration, got %d", resp.DurationMs)
	}
	if resp.To.Before(resp.From) {
		t.Error("To should be after From")
	}
}

// ============================================
// Additional: WebSocket new connections
// ============================================

func TestCheckpointWebSocketNewConnections(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	v4.AddWebSocketEvents([]WebSocketEvent{
		{Event: "open", ID: "ws-new", URL: "wss://realtime.example.com/feed"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff")
	}
	if len(resp.WebSocket.Connections) != 1 {
		t.Errorf("Expected 1 new connection, got %d", len(resp.WebSocket.Connections))
	}
	if resp.WebSocket.Connections[0].URL != "wss://realtime.example.com/feed" {
		t.Errorf("Unexpected connection URL: %s", resp.WebSocket.Connections[0].URL)
	}
}

// ============================================
// Additional: Actions diff
// ============================================

func TestCheckpointActionsDiff(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	now := time.Now().UnixMilli()
	v4.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: now, URL: "http://localhost/page"},
		{Type: "navigation", Timestamp: now + 100, URL: "http://localhost/page", ToURL: "http://localhost/other"},
		{Type: "input", Timestamp: now + 200, URL: "http://localhost/other"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Actions == nil {
		t.Fatal("Expected Actions diff")
	}
	if resp.Actions.TotalNew != 3 {
		t.Errorf("Expected TotalNew=3, got %d", resp.Actions.TotalNew)
	}
	if len(resp.Actions.Actions) != 3 {
		t.Errorf("Expected 3 actions, got %d", len(resp.Actions.Actions))
	}
}

// ============================================
// Additional: Degraded endpoint detection
// ============================================

func TestCheckpointDegradedEndpoint(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)

	// Baseline: endpoint responds in 50ms
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/data", Status: 200, Method: "GET", Duration: 50},
	})
	cm.GetChangesSince(GetChangesSinceParams{})

	// Same endpoint now takes >3x longer (200ms > 3*50ms)
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/data", Status: 200, Method: "GET", Duration: 200},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff")
	}
	if len(resp.Network.Degraded) != 1 {
		t.Errorf("Expected 1 degraded endpoint, got %d", len(resp.Network.Degraded))
	}
	if resp.Network.Degraded[0].Path != "/api/data" {
		t.Errorf("Expected path '/api/data', got '%s'", resp.Network.Degraded[0].Path)
	}
}

// ============================================
// Additional: First call behavior
// ============================================

func TestCheckpointFirstCallReturnsEverything(t *testing.T) {
	cm, server, v4 := setupCheckpointTest(t)

	// Add data before any checkpoint call
	addLogEntries(server,
		LogEntry{"level": "error", "msg": "pre-existing error"},
		LogEntry{"level": "warn", "msg": "pre-existing warning"},
	)
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Event: "open", ID: "ws-1", URL: "wss://example.com"},
	})

	// First call should return everything in the buffers
	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Console == nil {
		t.Fatal("First call should include existing console entries")
	}
	if resp.Console.TotalNew != 2 {
		t.Errorf("Expected TotalNew=2 on first call, got %d", resp.Console.TotalNew)
	}
	if resp.WebSocket == nil {
		t.Fatal("First call should include existing WebSocket entries")
	}
}

// ============================================
// Additional: Severity filter "warnings"
// ============================================

func TestCheckpointSeverityFilterWarnings(t *testing.T) {
	cm, server, _ := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	addLogEntries(server,
		LogEntry{"level": "info", "msg": "just info"},
		LogEntry{"level": "warn", "msg": "a warning"},
		LogEntry{"level": "error", "msg": "an error"},
	)

	resp := cm.GetChangesSince(GetChangesSinceParams{Severity: "warnings"})

	// "warnings" filter should include warnings and errors, but not info
	if resp.Console == nil {
		t.Fatal("Expected Console diff")
	}
	if len(resp.Console.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(resp.Console.Errors))
	}
	if len(resp.Console.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(resp.Console.Warnings))
	}
}

// ============================================
// Additional: WebSocket error messages
// ============================================

func TestCheckpointWebSocketErrors(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	v4.AddWebSocketEvents([]WebSocketEvent{
		{Event: "error", ID: "ws-1", URL: "wss://example.com/ws", Data: "connection refused"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff")
	}
	if len(resp.WebSocket.Errors) != 1 {
		t.Errorf("Expected 1 WS error, got %d", len(resp.WebSocket.Errors))
	}
	if resp.WebSocket.Errors[0].Message != "connection refused" {
		t.Errorf("Expected error message 'connection refused', got '%s'", resp.WebSocket.Errors[0].Message)
	}
}

// ============================================
// Additional: Network new endpoint with failure
// ============================================

func TestCheckpointNewEndpointWithFailure(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	// New endpoint that immediately fails (never seen before returning success)
	v4.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/new-thing", Status: 404, Method: "GET"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff")
	}
	// Should appear as new endpoint, and also as a failure (since it's 4xx)
	if len(resp.Network.NewEndpoints) != 1 {
		t.Errorf("Expected 1 new endpoint, got %d", len(resp.Network.NewEndpoints))
	}
}

// ============================================
// Additional: Total event counts in WebSocket diff
// ============================================

func TestCheckpointWebSocketTotalCount(t *testing.T) {
	cm, _, v4 := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add various WS events
	v4.AddWebSocketEvents([]WebSocketEvent{
		{Event: "open", ID: "ws-1", URL: "wss://a.com"},
		{Event: "message", ID: "ws-1", Direction: "incoming", Data: "hello"},
		{Event: "message", ID: "ws-1", Direction: "outgoing", Data: "world"},
		{Event: "close", ID: "ws-1", URL: "wss://a.com", CloseCode: 1000},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff")
	}
	if resp.WebSocket.TotalNew != 4 {
		t.Errorf("Expected TotalNew=4, got %d", resp.WebSocket.TotalNew)
	}
}

// ============================================
// Fingerprint unit tests
// ============================================

func TestFingerprintMessage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Error loading user abc12345-def6-7890-abcd-ef1234567890",
			expected: "Error loading user {uuid}",
		},
		{
			input:    "Request 12345 failed with status 50000",
			expected: "Request {n} failed with status {n}",
		},
		{
			input:    "Error at 2024-01-15T10:30:00.000Z in handler",
			expected: "Error at {ts} in handler",
		},
		{
			input:    "Simple error message",
			expected: "Simple error message",
		},
		{
			input:    "Error 42 is small",
			expected: "Error 42 is small", // numbers < 4 digits are kept
		},
	}

	for _, tt := range tests {
		result := FingerprintMessage(tt.input)
		if result != tt.expected {
			t.Errorf("FingerprintMessage(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractURLPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://localhost/api/users?page=1&limit=10", "/api/users"},
		{"http://localhost/api/users", "/api/users"},
		{"https://example.com/path/to/resource?key=val", "/path/to/resource"},
		{"/api/orders", "/api/orders"},
		{"http://localhost/", "/"},
	}

	for _, tt := range tests {
		result := ExtractURLPath(tt.input)
		if result != tt.expected {
			t.Errorf("ExtractURLPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
