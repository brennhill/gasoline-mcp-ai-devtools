package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================
// Test Helpers
// ============================================

func setupCheckpointTest(t *testing.T) (*CheckpointManager, *Server, *Capture) {
	t.Helper()
	server, _ := NewServer("", 1000)
	capture := NewCapture()
	cm := NewCheckpointManager(server, capture)
	return cm, server, capture
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
	cm, _, capture := setupCheckpointTest(t)

	// Add a successful request before checkpoint
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users", Status: 200, Method: "GET"},
	})

	// Establish checkpoint (which records known endpoint status)
	cm.GetChangesSince(GetChangesSinceParams{})

	// Now the endpoint fails
	capture.AddNetworkBodies([]NetworkBody{
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
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint with known endpoints
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users", Status: 200, Method: "GET"},
	})
	cm.GetChangesSince(GetChangesSinceParams{})

	// New endpoint appears
	capture.AddNetworkBodies([]NetworkBody{
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
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// WebSocket close event after checkpoint
	capture.AddWebSocketEvents([]WebSocketEvent{
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
	cm, server, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add a mix of warnings and errors
	addLogEntries(server,
		LogEntry{"level": "warn", "msg": "deprecation warning"},
		LogEntry{"level": "error", "msg": "fatal error"},
	)
	// Add a WebSocket disconnection (warning-level)
	capture.AddWebSocketEvents([]WebSocketEvent{
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
	cm, server, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add data in all categories
	addLogEntries(server, LogEntry{"level": "error", "msg": "error"})
	capture.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/test", Status: 500, Method: "GET"}})
	capture.AddWebSocketEvents([]WebSocketEvent{{Event: "close", ID: "ws-1", URL: "wss://x.com"}})
	capture.AddEnhancedActions([]EnhancedAction{{Type: "click", Timestamp: time.Now().UnixMilli()}})

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
	cm, server, capture := setupCheckpointTest(t)

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
				capture.AddWebSocketEvents([]WebSocketEvent{{Event: "message", ID: "ws-1", Data: "test"}})
				capture.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/x", Status: 200, Method: "GET"}})
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
	cm, _, capture := setupCheckpointTest(t)

	// Multiple requests to same path with different query params
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/users?page=1&limit=10", Status: 200, Method: "GET"},
	})
	cm.GetChangesSince(GetChangesSinceParams{})

	// Same endpoint with different query params now fails
	capture.AddNetworkBodies([]NetworkBody{
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
		cm, server, capture := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		// Add both a warning and an error
		addLogEntries(server, LogEntry{"level": "warn", "msg": "warning"})
		addLogEntries(server, LogEntry{"level": "error", "msg": "error"})
		capture.AddWebSocketEvents([]WebSocketEvent{{Event: "close", ID: "ws-1", URL: "wss://x.com", CloseCode: 1006}})

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
		cm, _, capture := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		capture.AddWebSocketEvents([]WebSocketEvent{{Event: "close", ID: "ws-1", URL: "wss://x.com", CloseCode: 1006}})

		resp := cm.GetChangesSince(GetChangesSinceParams{})
		if resp.Severity != "warning" {
			t.Errorf("Expected 'warning' severity for WS disconnect, got '%s'", resp.Severity)
		}
	})

	t.Run("network failure is error", func(t *testing.T) {
		cm, _, capture := setupCheckpointTest(t)
		capture.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/test", Status: 200, Method: "GET"}})
		cm.GetChangesSince(GetChangesSinceParams{})

		capture.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/test", Status: 500, Method: "GET"}})

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
		cm, server, capture := setupCheckpointTest(t)
		capture.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/a", Status: 200, Method: "GET"}})
		cm.GetChangesSince(GetChangesSinceParams{})

		addLogEntries(server,
			LogEntry{"level": "error", "msg": "err1"},
			LogEntry{"level": "error", "msg": "err2"},
		)
		capture.AddNetworkBodies([]NetworkBody{{URL: "http://localhost/api/a", Status: 500, Method: "GET"}})

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
		cm, _, capture := setupCheckpointTest(t)
		cm.GetChangesSince(GetChangesSinceParams{})

		capture.AddWebSocketEvents([]WebSocketEvent{
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
	cm, _, capture := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	capture.AddWebSocketEvents([]WebSocketEvent{
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
	cm, _, capture := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	now := time.Now().UnixMilli()
	capture.AddEnhancedActions([]EnhancedAction{
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
	cm, _, capture := setupCheckpointTest(t)

	// Baseline: endpoint responds in 50ms
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "http://localhost/api/data", Status: 200, Method: "GET", Duration: 50},
	})
	cm.GetChangesSince(GetChangesSinceParams{})

	// Same endpoint now takes >3x longer (200ms > 3*50ms)
	capture.AddNetworkBodies([]NetworkBody{
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
	cm, server, capture := setupCheckpointTest(t)

	// Add data before any checkpoint call
	addLogEntries(server,
		LogEntry{"level": "error", "msg": "pre-existing error"},
		LogEntry{"level": "warn", "msg": "pre-existing warning"},
	)
	capture.AddWebSocketEvents([]WebSocketEvent{
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
	cm, _, capture := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	capture.AddWebSocketEvents([]WebSocketEvent{
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
	cm, _, capture := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	// New endpoint that immediately fails (never seen before returning success)
	capture.AddNetworkBodies([]NetworkBody{
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
	cm, _, capture := setupCheckpointTest(t)
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add various WS events
	capture.AddWebSocketEvents([]WebSocketEvent{
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

// ============================================
// Push Regression Notification Tests
// ============================================

// Helper: creates a PerformanceSnapshot and detects regressions against the current baseline.
// Captures the baseline BEFORE adding the snapshot (simulating real-time detection).
func addSnapshotAndDetect(cm *CheckpointManager, capture *Capture, snapshot PerformanceSnapshot) {
	baseline, hasBaseline := capture.GetPerformanceBaseline(snapshot.URL)
	capture.AddPerformanceSnapshot(snapshot)
	if hasBaseline {
		cm.DetectAndStoreAlerts(snapshot, baseline)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

// Helper: creates a baseline snapshot for a given URL and adds it to the capture.
func addBaselineSnapshot(capture *Capture, url string, load float64, fcp *float64, lcp *float64, ttfb float64, transferSize int64, cls *float64) {
	snapshot := PerformanceSnapshot{
		URL:       url,
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load:                   load,
			FirstContentfulPaint:   fcp,
			LargestContentfulPaint: lcp,
			TimeToFirstByte:        ttfb,
			DomContentLoaded:       load * 0.8,
			DomInteractive:         load * 0.6,
		},
		Network: NetworkSummary{TransferSize: transferSize, RequestCount: 10},
		CLS:     cls,
	}
	capture.AddPerformanceSnapshot(snapshot)
}

// Test 1: Snapshot within threshold -> no alert generated
func TestPushRegression_WithinThreshold_NoAlert(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Create baseline (first snapshot)
	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, floatPtr(0.05))

	// Add a second snapshot within threshold (15% regression, under 20%)
	withinThreshold := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load:                   1150,
			FirstContentfulPaint:   floatPtr(575),
			LargestContentfulPaint: floatPtr(920),
			TimeToFirstByte:        230,
			DomContentLoaded:       920,
			DomInteractive:         690,
		},
		Network: NetworkSummary{TransferSize: 230000, RequestCount: 10},
		CLS:     floatPtr(0.06),
	}
	addSnapshotAndDetect(cm, capture, withinThreshold)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if resp.PerformanceAlerts != nil && len(resp.PerformanceAlerts) > 0 {
		t.Errorf("Expected no performance alerts for within-threshold snapshot, got %d", len(resp.PerformanceAlerts))
	}
}

// Test 2: Snapshot with load time 30% over baseline -> alert generated with correct delta
func TestPushRegression_LoadTimeRegression_AlertGenerated(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Create baseline
	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, floatPtr(0.05))

	// Trigger regression: 30% load time increase (1000 -> 1300)
	regressing := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load:                   1300,
			FirstContentfulPaint:   floatPtr(500),
			LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte:        200,
			DomContentLoaded:       1040,
			DomInteractive:         780,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if resp.PerformanceAlerts == nil || len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected performance alert for load time regression")
	}

	alert := resp.PerformanceAlerts[0]
	if alert.Type != "regression" {
		t.Errorf("Expected alert type 'regression', got '%s'", alert.Type)
	}
	if alert.URL != "/dashboard" {
		t.Errorf("Expected alert URL '/dashboard', got '%s'", alert.URL)
	}

	loadMetric, ok := alert.Metrics["load"]
	if !ok {
		t.Fatal("Expected 'load' metric in alert")
	}
	if loadMetric.Baseline != 1000 {
		t.Errorf("Expected baseline 1000, got %f", loadMetric.Baseline)
	}
	if loadMetric.Current != 1300 {
		t.Errorf("Expected current 1300, got %f", loadMetric.Current)
	}
	if loadMetric.DeltaMs != 300 {
		t.Errorf("Expected delta_ms 300, got %f", loadMetric.DeltaMs)
	}
	if loadMetric.DeltaPct < 29 || loadMetric.DeltaPct > 31 {
		t.Errorf("Expected delta_pct ~30, got %f", loadMetric.DeltaPct)
	}
}

// Test 3: Alert appears in get_changes_since JSON response
func TestPushRegression_AlertInResponse(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	respJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(respJSON, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["performance_alerts"]; !ok {
		t.Error("Expected 'performance_alerts' key in response JSON")
	}
}

// Test 4: Alert not repeated on subsequent auto-advancing call
func TestPushRegression_AlertNotRepeated(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp1 := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp1.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert on first call")
	}

	resp2 := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp2.PerformanceAlerts) > 0 {
		t.Errorf("Expected no alerts on second call, got %d", len(resp2.PerformanceAlerts))
	}
}

// Test 5: Multiple regressions on different URLs -> multiple alerts
func TestPushRegression_MultipleAlerts(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page-a", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)
	addBaselineSnapshot(capture, "/page-b", 800, floatPtr(400), floatPtr(600), 150, 150000, nil)

	snapshotA := PerformanceSnapshot{
		URL:       "/page-a",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	snapshotB := PerformanceSnapshot{
		URL:       "/page-b",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1400, FirstContentfulPaint: floatPtr(400), LargestContentfulPaint: floatPtr(600),
			TimeToFirstByte: 150, DomContentLoaded: 1120, DomInteractive: 840,
		},
		Network: NetworkSummary{TransferSize: 150000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, snapshotA)
	addSnapshotAndDetect(cm, capture, snapshotB)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) < 2 {
		t.Errorf("Expected at least 2 alerts, got %d", len(resp.PerformanceAlerts))
	}
}

// Test 6: Max 10 pending alerts, oldest dropped
func TestPushRegression_MaxAlertsCapped(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	for i := 0; i < 11; i++ {
		url := fmt.Sprintf("/page-%d", i)
		addBaselineSnapshot(capture, url, 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)
	}

	for i := 0; i < 11; i++ {
		url := fmt.Sprintf("/page-%d", i)
		snapshot := PerformanceSnapshot{
			URL:       url,
			Timestamp: time.Now().Format(time.RFC3339),
			Timing: PerformanceTiming{
				Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
				TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
			},
			Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
		}
		addSnapshotAndDetect(cm, capture, snapshot)
	}

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) > 10 {
		t.Errorf("Expected max 10 alerts, got %d", len(resp.PerformanceAlerts))
	}

	foundOldest := false
	foundNewest := false
	for _, alert := range resp.PerformanceAlerts {
		if alert.URL == "/page-0" {
			foundOldest = true
		}
		if alert.URL == "/page-10" {
			foundNewest = true
		}
	}
	if foundOldest {
		t.Error("Expected oldest alert (page-0) to be dropped")
	}
	if !foundNewest {
		t.Error("Expected newest alert (page-10) to be present")
	}
}

// Test 7: Regression resolved by subsequent good snapshot
func TestPushRegression_RegressionResolved(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resolved := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1050, FirstContentfulPaint: floatPtr(520), LargestContentfulPaint: floatPtr(830),
			TimeToFirstByte: 210, DomContentLoaded: 840, DomInteractive: 630,
		},
		Network: NetworkSummary{TransferSize: 210000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, resolved)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	for _, alert := range resp.PerformanceAlerts {
		if alert.URL == "/dashboard" {
			t.Error("Expected resolved regression alert to be cleared")
		}
	}
}

// Test 8: No baseline -> no alert
func TestPushRegression_NoBaseline_NoAlert(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	snapshot := PerformanceSnapshot{
		URL:       "/new-page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 2000, FirstContentfulPaint: floatPtr(1000), LargestContentfulPaint: floatPtr(1500),
			TimeToFirstByte: 500, DomContentLoaded: 1600, DomInteractive: 1200,
		},
		Network: NetworkSummary{TransferSize: 500000, RequestCount: 20},
	}
	addSnapshotAndDetect(cm, capture, snapshot)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) > 0 {
		t.Errorf("Expected no alerts for first snapshot (no baseline), got %d", len(resp.PerformanceAlerts))
	}
}

// Test 9: Only regressed metrics included
func TestPushRegression_OnlyRegressedMetrics(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1300, FirstContentfulPaint: floatPtr(550), LargestContentfulPaint: floatPtr(880),
			TimeToFirstByte: 220, DomContentLoaded: 1040, DomInteractive: 780,
		},
		Network: NetworkSummary{TransferSize: 220000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert for load regression")
	}

	alert := resp.PerformanceAlerts[0]
	if _, ok := alert.Metrics["load"]; !ok {
		t.Error("Expected 'load' metric in alert")
	}
	if _, ok := alert.Metrics["fcp"]; ok {
		t.Error("Expected 'fcp' metric to be omitted (within threshold)")
	}
	if _, ok := alert.Metrics["lcp"]; ok {
		t.Error("Expected 'lcp' metric to be omitted (within threshold)")
	}
	if _, ok := alert.Metrics["ttfb"]; ok {
		t.Error("Expected 'ttfb' metric to be omitted (within threshold)")
	}
	if _, ok := alert.Metrics["transfer_bytes"]; ok {
		t.Error("Expected 'transfer_bytes' metric to be omitted (within threshold)")
	}
}

// Test 10: Recommendation field is populated
func TestPushRegression_RecommendationField(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert")
	}
	if resp.PerformanceAlerts[0].Recommendation == "" {
		t.Error("Expected non-empty recommendation")
	}
}

// Test 11: Query from earlier named checkpoint includes alert
func TestPushRegression_CheckpointTracking_Included(t *testing.T) {
	cm, server, capture := setupCheckpointTest(t)

	addLogEntries(server, LogEntry{"level": "info", "msg": "setup"})
	cm.CreateCheckpoint("before")

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{Checkpoint: "before"})
	if len(resp.PerformanceAlerts) == 0 {
		t.Error("Expected alert when querying from earlier checkpoint")
	}
}

// Test 12: Query from checkpoint after alert -> not included
func TestPushRegression_CheckpointTracking_NotIncluded(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp1 := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp1.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert on first call")
	}

	cm.CreateCheckpoint("after")

	resp2 := cm.GetChangesSince(GetChangesSinceParams{Checkpoint: "after"})
	if len(resp2.PerformanceAlerts) > 0 {
		t.Errorf("Expected no alerts from checkpoint after alert, got %d", len(resp2.PerformanceAlerts))
	}
}

// Test 13: CLS regression (absolute increase > 0.1) -> alert
func TestPushRegression_CLSRegression(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, floatPtr(0.05))

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1000, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 800, DomInteractive: 600,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
		CLS:     floatPtr(0.2),
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert for CLS regression")
	}

	clsMetric, ok := resp.PerformanceAlerts[0].Metrics["cls"]
	if !ok {
		t.Fatal("Expected 'cls' metric in alert")
	}
	if clsMetric.DeltaMs < 0.14 || clsMetric.DeltaMs > 0.16 {
		t.Errorf("Expected CLS delta ~0.15, got %f", clsMetric.DeltaMs)
	}
}

// Test 14: TTFB under 50% threshold -> no alert
func TestPushRegression_TTFBUnderThreshold_NoAlert(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1000, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 290, DomContentLoaded: 800, DomInteractive: 600,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	for _, alert := range resp.PerformanceAlerts {
		if _, ok := alert.Metrics["ttfb"]; ok {
			t.Error("Expected no TTFB alert for under-50% regression")
		}
	}
}

// Test 15: Summary field populated
func TestPushRegression_SummaryField(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/dashboard", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/dashboard",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert")
	}
	if resp.PerformanceAlerts[0].Summary == "" {
		t.Error("Expected non-empty alert summary")
	}
}

// Test 16: Transfer size regression (>25%) -> alert
func TestPushRegression_TransferSizeRegression(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1000, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 800, DomInteractive: 600,
		},
		Network: NetworkSummary{TransferSize: 260000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert for transfer size regression")
	}

	transferMetric, ok := resp.PerformanceAlerts[0].Metrics["transfer_bytes"]
	if !ok {
		t.Fatal("Expected 'transfer_bytes' metric in alert")
	}
	if transferMetric.DeltaMs != 60000 {
		t.Errorf("Expected transfer delta 60000, got %f", transferMetric.DeltaMs)
	}
}

// Test 17: Concurrent access safety
func TestPushRegression_ConcurrentAccess(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(idx int) {
			defer wg.Done()
			snapshot := PerformanceSnapshot{
				URL:       "/page",
				Timestamp: time.Now().Format(time.RFC3339),
				Timing: PerformanceTiming{
					Load: 1500, TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
				},
				Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
			}
			baseline, hasBaseline := capture.GetPerformanceBaseline(snapshot.URL)
			capture.AddPerformanceSnapshot(snapshot)
			if hasBaseline {
				cm.DetectAndStoreAlerts(snapshot, baseline)
			}
		}(i)
		go func() {
			defer wg.Done()
			cm.GetChangesSince(GetChangesSinceParams{})
		}()
	}
	wg.Wait()
}

// Test 18: DetectedAt timestamp is reasonable
func TestPushRegression_DetectedAtTimestamp(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	addBaselineSnapshot(capture, "/page", 1000, floatPtr(500), floatPtr(800), 200, 200000, nil)

	before := time.Now()

	regressing := PerformanceSnapshot{
		URL:       "/page",
		Timestamp: time.Now().Format(time.RFC3339),
		Timing: PerformanceTiming{
			Load: 1500, FirstContentfulPaint: floatPtr(500), LargestContentfulPaint: floatPtr(800),
			TimeToFirstByte: 200, DomContentLoaded: 1200, DomInteractive: 900,
		},
		Network: NetworkSummary{TransferSize: 200000, RequestCount: 10},
	}
	addSnapshotAndDetect(cm, capture, regressing)

	after := time.Now()

	resp := cm.GetChangesSince(GetChangesSinceParams{})
	if len(resp.PerformanceAlerts) == 0 {
		t.Fatal("Expected alert")
	}

	detectedAt, err := time.Parse(time.RFC3339Nano, resp.PerformanceAlerts[0].DetectedAt)
	if err != nil {
		t.Fatalf("Failed to parse DetectedAt: %v", err)
	}
	if detectedAt.Before(before) || detectedAt.After(after) {
		t.Errorf("DetectedAt %v not between %v and %v", detectedAt, before, after)
	}
}

// ============================================
// Coverage Gap Tests
// ============================================

// TestContainsString_FoundTrue exercises the return-true path of containsString
func TestContainsString_FoundTrue(t *testing.T) {
	slice := []string{"/api/users", "/api/posts", "/api/comments"}
	if !containsString(slice, "/api/posts") {
		t.Error("Expected containsString to return true for existing element")
	}
	if !containsString(slice, "/api/users") {
		t.Error("Expected containsString to return true for first element")
	}
	if !containsString(slice, "/api/comments") {
		t.Error("Expected containsString to return true for last element")
	}
}

func TestContainsString_NotFound(t *testing.T) {
	slice := []string{"/api/users", "/api/posts"}
	if containsString(slice, "/api/unknown") {
		t.Error("Expected containsString to return false for missing element")
	}
}

func TestContainsString_EmptySlice(t *testing.T) {
	if containsString(nil, "anything") {
		t.Error("Expected containsString to return false for nil slice")
	}
	if containsString([]string{}, "anything") {
		t.Error("Expected containsString to return false for empty slice")
	}
}

// TestBuildAlertSummary_MultipleCategories tests the fallback path
// when the "load" metric is not present, covering the for-range fallback.
func TestBuildAlertSummary_MultipleCategories(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// No "load" metric: should use fallback path
	metrics := map[string]AlertMetricDelta{
		"fcp": {Baseline: 200, Current: 500, DeltaMs: 300, DeltaPct: 150.0},
		"lcp": {Baseline: 800, Current: 1200, DeltaMs: 400, DeltaPct: 50.0},
	}

	summary := cm.buildAlertSummary("http://example.com/page", metrics)

	// The fallback picks the first metric from the map iteration — it should contain the URL
	if !strings.Contains(summary, "http://example.com/page") {
		t.Errorf("Expected summary to contain URL, got: %s", summary)
	}
	// It should mention "regressed" in the fallback format
	if !strings.Contains(summary, "regressed") {
		t.Errorf("Expected summary to contain 'regressed', got: %s", summary)
	}
}

// TestBuildAlertSummary_LoadMetricPresent tests the primary path
func TestBuildAlertSummary_LoadMetricPresent(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	metrics := map[string]AlertMetricDelta{
		"load": {Baseline: 1000, Current: 1500, DeltaMs: 500, DeltaPct: 50.0},
		"fcp":  {Baseline: 200, Current: 400, DeltaMs: 200, DeltaPct: 100.0},
	}

	summary := cm.buildAlertSummary("http://example.com", metrics)

	if !strings.Contains(summary, "Load time regressed") {
		t.Errorf("Expected 'Load time regressed' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "500ms") {
		t.Errorf("Expected '500ms' delta in summary, got: %s", summary)
	}
}

// TestBuildAlertSummary_EmptyMetrics tests the final fallback
func TestBuildAlertSummary_EmptyMetrics(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	summary := cm.buildAlertSummary("http://example.com", map[string]AlertMetricDelta{})
	expected := "Performance regression detected on http://example.com"
	if summary != expected {
		t.Errorf("Expected %q, got %q", expected, summary)
	}
}

// TestComputeWebSocketDiff_DirectionFilter tests WS events with various event types
func TestComputeWebSocketDiff_DirectionFilter(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Create checkpoint before adding events
	cm.CreateCheckpoint("ws-test")

	// Add WS events with different event types
	capture.AddWebSocketEvents([]WebSocketEvent{
		{Event: "open", URL: "ws://example.com/ws", ID: "conn-1", Direction: "incoming"},
		{Event: "close", URL: "ws://example.com/ws", ID: "conn-1", CloseCode: 1000, CloseReason: "normal"},
		{Event: "error", URL: "ws://example.com/ws2", ID: "conn-2", Data: "connection failed"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "ws-test",
		Include:    []string{"websocket"},
	})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if resp.WebSocket.TotalNew != 3 {
		t.Errorf("Expected 3 total new WS events, got %d", resp.WebSocket.TotalNew)
	}
	if len(resp.WebSocket.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(resp.WebSocket.Connections))
	}
	if resp.WebSocket.Connections[0].URL != "ws://example.com/ws" {
		t.Errorf("Expected connection URL 'ws://example.com/ws', got '%s'", resp.WebSocket.Connections[0].URL)
	}
	if len(resp.WebSocket.Disconnections) != 1 {
		t.Errorf("Expected 1 disconnection, got %d", len(resp.WebSocket.Disconnections))
	}
	if resp.WebSocket.Disconnections[0].CloseCode != 1000 {
		t.Errorf("Expected close code 1000, got %d", resp.WebSocket.Disconnections[0].CloseCode)
	}
	if resp.WebSocket.Disconnections[0].CloseReason != "normal" {
		t.Errorf("Expected close reason 'normal', got '%s'", resp.WebSocket.Disconnections[0].CloseReason)
	}
	if len(resp.WebSocket.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(resp.WebSocket.Errors))
	}
	if resp.WebSocket.Errors[0].Message != "connection failed" {
		t.Errorf("Expected error message 'connection failed', got '%s'", resp.WebSocket.Errors[0].Message)
	}
}

// TestComputeWebSocketDiff_ErrorsOnly tests that severity=errors_only suppresses disconnections
func TestComputeWebSocketDiff_ErrorsOnly(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	cm.CreateCheckpoint("ws-errors")

	capture.AddWebSocketEvents([]WebSocketEvent{
		{Event: "close", URL: "ws://example.com/ws", ID: "conn-1", CloseCode: 1006},
		{Event: "error", URL: "ws://example.com/ws", ID: "conn-1", Data: "abnormal closure"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "ws-errors",
		Include:    []string{"websocket"},
		Severity:   "errors_only",
	})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff because there are errors")
	}
	if len(resp.WebSocket.Disconnections) != 0 {
		t.Errorf("Expected 0 disconnections with errors_only, got %d", len(resp.WebSocket.Disconnections))
	}
	if len(resp.WebSocket.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(resp.WebSocket.Errors))
	}
}

// TestComputeNetworkDiff_ResponseBodies tests network bodies with response bodies present
func TestComputeNetworkDiff_ResponseBodies(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Set up known endpoints in a checkpoint
	cm.mu.Lock()
	cm.autoCheckpoint = &Checkpoint{
		CreatedAt:    time.Now().Add(-1 * time.Minute),
		NetworkTotal: 0,
		KnownEndpoints: map[string]endpointState{
			"/api/users": {Status: 200, Duration: 50},
		},
	}
	cm.mu.Unlock()

	// Add network bodies with response body content
	capture.AddNetworkBodies([]NetworkBody{
		{Method: "GET", URL: "http://example.com/api/users", Status: 200, Duration: 200, ResponseBody: `{"users":[]}`},
		{Method: "POST", URL: "http://example.com/api/new-endpoint", Status: 201, ResponseBody: `{"id":1}`},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{Include: []string{"network"}})

	if resp.Network == nil {
		t.Fatal("Expected Network diff to be non-nil")
	}
	if resp.Network.TotalNew != 2 {
		t.Errorf("Expected 2 total new, got %d", resp.Network.TotalNew)
	}
	// /api/users has duration 200 vs baseline 50, factor 4x > 3x threshold
	if len(resp.Network.Degraded) != 1 {
		t.Errorf("Expected 1 degraded endpoint, got %d", len(resp.Network.Degraded))
	}
	if len(resp.Network.NewEndpoints) != 1 {
		t.Errorf("Expected 1 new endpoint, got %d", len(resp.Network.NewEndpoints))
	}
}

// TestComputeActionsDiff_DifferentTypes tests actions with various types
func TestComputeActionsDiff_DifferentTypes(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	cm.CreateCheckpoint("actions-test")

	capture.AddEnhancedActions([]EnhancedAction{
		{Type: "click", Timestamp: 1000, URL: "http://example.com/page1"},
		{Type: "input", Timestamp: 2000, URL: "http://example.com/page1"},
		{Type: "navigation", Timestamp: 3000, URL: "http://example.com/page2"},
		{Type: "scroll", Timestamp: 4000, URL: "http://example.com/page2"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "actions-test",
		Include:    []string{"actions"},
	})

	if resp.Actions == nil {
		t.Fatal("Expected Actions diff to be non-nil")
	}
	if resp.Actions.TotalNew != 4 {
		t.Errorf("Expected 4 total new actions, got %d", resp.Actions.TotalNew)
	}
	if len(resp.Actions.Actions) != 4 {
		t.Fatalf("Expected 4 action entries, got %d", len(resp.Actions.Actions))
	}

	types := map[string]bool{}
	for _, a := range resp.Actions.Actions {
		types[a.Type] = true
	}
	for _, expected := range []string{"click", "input", "navigation", "scroll"} {
		if !types[expected] {
			t.Errorf("Expected action type %q in diff", expected)
		}
	}
}

// TestFindPositionAtTime_TimeBeforeAll tests findPositionAtTime when all entries are after t
func TestFindPositionAtTime_TimeBeforeAll(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// Create timestamps all in the future relative to our query time
	now := time.Now()
	addedAt := []time.Time{
		now.Add(1 * time.Second),
		now.Add(2 * time.Second),
		now.Add(3 * time.Second),
	}

	// Query for a time before all entries
	pos := cm.findPositionAtTime(addedAt, 10, now.Add(-1*time.Second))

	// All 3 entries are after the query time, so position = 10 - 3 = 7
	if pos != 7 {
		t.Errorf("Expected position 7, got %d", pos)
	}
}

// TestFindPositionAtTime_EmptySlice tests findPositionAtTime with empty addedAt
func TestFindPositionAtTime_EmptySlice(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	pos := cm.findPositionAtTime([]time.Time{}, 5, time.Now())
	if pos != 5 {
		t.Errorf("Expected currentTotal (5), got %d", pos)
	}
}

// TestFindPositionAtTime_TimeAfterAll tests findPositionAtTime when all entries are before t
func TestFindPositionAtTime_TimeAfterAll(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	now := time.Now()
	addedAt := []time.Time{
		now.Add(-3 * time.Second),
		now.Add(-2 * time.Second),
		now.Add(-1 * time.Second),
	}

	pos := cm.findPositionAtTime(addedAt, 10, now.Add(1*time.Second))

	// All entries are before query time, so entriesAfter=0, pos = 10
	if pos != 10 {
		t.Errorf("Expected position 10, got %d", pos)
	}
}

// TestFindPositionAtTime_PositionClampsToZero tests the pos<0 guard
func TestFindPositionAtTime_PositionClampsToZero(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	now := time.Now()
	addedAt := []time.Time{
		now.Add(1 * time.Second),
		now.Add(2 * time.Second),
		now.Add(3 * time.Second),
	}

	// currentTotal=2, but 3 entries are after query time -> 2-3 = -1 -> clamp to 0
	pos := cm.findPositionAtTime(addedAt, 2, now.Add(-1*time.Second))
	if pos != 0 {
		t.Errorf("Expected position to be clamped to 0, got %d", pos)
	}
}

// ============================================
// Coverage: computeWebSocketDiff — open, error, close events and capping
// ============================================

func TestComputeWebSocketDiff_OpenEvents(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add WebSocket open events
	capture.mu.Lock()
	capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
		Event: "open",
		URL:   "ws://example.com/socket",
		ID:    "conn-1",
	})
	capture.wsTotalAdded = 1
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if resp.WebSocket.TotalNew != 1 {
		t.Errorf("Expected TotalNew=1, got %d", resp.WebSocket.TotalNew)
	}
	if len(resp.WebSocket.Connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(resp.WebSocket.Connections))
	}
	if resp.WebSocket.Connections[0].URL != "ws://example.com/socket" {
		t.Errorf("Expected URL ws://example.com/socket, got %s", resp.WebSocket.Connections[0].URL)
	}
	if resp.WebSocket.Connections[0].ID != "conn-1" {
		t.Errorf("Expected ID conn-1, got %s", resp.WebSocket.Connections[0].ID)
	}
}

func TestComputeWebSocketDiff_ErrorEvents(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add WebSocket error events
	capture.mu.Lock()
	capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
		Event: "error",
		URL:   "ws://example.com/socket",
		Data:  "Connection refused",
	})
	capture.wsTotalAdded = 1
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if len(resp.WebSocket.Errors) != 1 {
		t.Fatalf("Expected 1 error, got %d", len(resp.WebSocket.Errors))
	}
	if resp.WebSocket.Errors[0].URL != "ws://example.com/socket" {
		t.Errorf("Expected URL ws://example.com/socket, got %s", resp.WebSocket.Errors[0].URL)
	}
	if resp.WebSocket.Errors[0].Message != "Connection refused" {
		t.Errorf("Expected message 'Connection refused', got '%s'", resp.WebSocket.Errors[0].Message)
	}
}

func TestComputeWebSocketDiff_CloseEvents(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add WebSocket close events
	capture.mu.Lock()
	capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
		Event:       "close",
		URL:         "ws://example.com/socket",
		CloseCode:   1006,
		CloseReason: "Abnormal closure",
	})
	capture.wsTotalAdded = 1
	capture.mu.Unlock()

	// Default severity (not errors_only) should include disconnections
	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if len(resp.WebSocket.Disconnections) != 1 {
		t.Fatalf("Expected 1 disconnection, got %d", len(resp.WebSocket.Disconnections))
	}
	if resp.WebSocket.Disconnections[0].CloseCode != 1006 {
		t.Errorf("Expected close code 1006, got %d", resp.WebSocket.Disconnections[0].CloseCode)
	}
}

func TestComputeWebSocketDiff_CapsConnections(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add more than maxDiffEntriesPerCat (50) open events
	capture.mu.Lock()
	for i := 0; i < 60; i++ {
		capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
			Event: "open",
			URL:   fmt.Sprintf("ws://example.com/socket-%d", i),
			ID:    fmt.Sprintf("conn-%d", i),
		})
	}
	capture.wsTotalAdded = 60
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if len(resp.WebSocket.Connections) != 50 {
		t.Errorf("Expected connections capped at 50, got %d", len(resp.WebSocket.Connections))
	}
}

func TestComputeWebSocketDiff_CapsErrors(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add more than maxDiffEntriesPerCat (50) error events
	capture.mu.Lock()
	for i := 0; i < 55; i++ {
		capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
			Event: "error",
			URL:   fmt.Sprintf("ws://example.com/socket-%d", i),
			Data:  fmt.Sprintf("error-%d", i),
		})
	}
	capture.wsTotalAdded = 55
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if len(resp.WebSocket.Errors) != 50 {
		t.Errorf("Expected errors capped at 50, got %d", len(resp.WebSocket.Errors))
	}
}

func TestComputeWebSocketDiff_CapsDisconnections(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add more than maxDiffEntriesPerCat (50) close events
	capture.mu.Lock()
	for i := 0; i < 55; i++ {
		capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
			Event:     "close",
			URL:       fmt.Sprintf("ws://example.com/socket-%d", i),
			CloseCode: 1000,
		})
	}
	capture.wsTotalAdded = 55
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.WebSocket == nil {
		t.Fatal("Expected WebSocket diff to be non-nil")
	}
	if len(resp.WebSocket.Disconnections) != 50 {
		t.Errorf("Expected disconnections capped at 50, got %d", len(resp.WebSocket.Disconnections))
	}
}

// ============================================
// Coverage: computeActionsDiff — various action types
// ============================================

func TestComputeActionsDiff_VariousActionTypes(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add actions of various types
	capture.mu.Lock()
	capture.enhancedActions = append(capture.enhancedActions,
		EnhancedAction{Type: "click", Timestamp: 1000, URL: "/page1"},
		EnhancedAction{Type: "keypress", Timestamp: 1001, URL: "/page1"},
		EnhancedAction{Type: "scroll", Timestamp: 1002, URL: "/page1"},
		EnhancedAction{Type: "input", Timestamp: 1003, URL: "/page2"},
		EnhancedAction{Type: "navigation", Timestamp: 1004, URL: "/page3"},
	)
	capture.actionTotalAdded = 5
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Actions == nil {
		t.Fatal("Expected Actions diff to be non-nil")
	}
	if resp.Actions.TotalNew != 5 {
		t.Errorf("Expected TotalNew=5, got %d", resp.Actions.TotalNew)
	}
	if len(resp.Actions.Actions) != 5 {
		t.Fatalf("Expected 5 actions, got %d", len(resp.Actions.Actions))
	}

	// Verify action types are preserved
	expectedTypes := []string{"click", "keypress", "scroll", "input", "navigation"}
	for i, expected := range expectedTypes {
		if resp.Actions.Actions[i].Type != expected {
			t.Errorf("Action[%d]: expected type '%s', got '%s'", i, expected, resp.Actions.Actions[i].Type)
		}
	}
}

func TestComputeActionsDiff_CapsAtMax(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add more than maxDiffEntriesPerCat (50) actions
	capture.mu.Lock()
	for i := 0; i < 60; i++ {
		capture.enhancedActions = append(capture.enhancedActions,
			EnhancedAction{Type: "click", Timestamp: int64(1000 + i), URL: fmt.Sprintf("/page-%d", i)},
		)
	}
	capture.actionTotalAdded = 60
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Actions == nil {
		t.Fatal("Expected Actions diff to be non-nil")
	}
	if resp.Actions.TotalNew != 60 {
		t.Errorf("Expected TotalNew=60, got %d", resp.Actions.TotalNew)
	}
	if len(resp.Actions.Actions) != 50 {
		t.Errorf("Expected actions capped at 50, got %d", len(resp.Actions.Actions))
	}
}

// ============================================
// Coverage: DetectAndStoreAlerts — various regression types
// ============================================

func TestDetectAndStoreAlerts_CLSRegression(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baselineCLS := 0.05
	snapshotCLS := 0.25 // >0.1 absolute increase

	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load: 1000,
		},
		CLS: &baselineCLS,
	}

	snapshot := PerformanceSnapshot{
		URL: "/test-page",
		Timing: PerformanceTiming{
			Load: 1000, // No load regression
		},
		CLS: &snapshotCLS,
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	alert := cm.pendingAlerts[0]
	if _, ok := alert.Metrics["cls"]; !ok {
		t.Error("Expected CLS metric in alert")
	}
	if alert.URL != "/test-page" {
		t.Errorf("Expected URL '/test-page', got '%s'", alert.URL)
	}
}

func TestDetectAndStoreAlerts_FCPRegression(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baselineFCP := 200.0
	snapshotFCP := 300.0 // >20% increase (50%)

	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:                 1000,
			FirstContentfulPaint: &baselineFCP,
		},
	}

	snapshot := PerformanceSnapshot{
		URL: "/fcp-page",
		Timing: PerformanceTiming{
			Load:                 1000, // No load regression
			FirstContentfulPaint: &snapshotFCP,
		},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	if _, ok := cm.pendingAlerts[0].Metrics["fcp"]; !ok {
		t.Error("Expected FCP metric in alert")
	}
}

func TestDetectAndStoreAlerts_TransferSizeRegression(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load: 1000,
		},
		Network: BaselineNetwork{
			TransferSize: 100000,
		},
	}

	snapshot := PerformanceSnapshot{
		URL: "/bundle-page",
		Timing: PerformanceTiming{
			Load: 1000, // No load regression
		},
		Network: NetworkSummary{
			TransferSize: 150000, // >25% increase (50%)
		},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	if _, ok := cm.pendingAlerts[0].Metrics["transfer_bytes"]; !ok {
		t.Error("Expected transfer_bytes metric in alert")
	}
}

func TestDetectAndStoreAlerts_BaselineTooFew(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baseline := PerformanceBaseline{
		SampleCount: 0, // Less than 1, should early-return
		Timing: BaselineTiming{
			Load: 500,
		},
	}

	snapshot := PerformanceSnapshot{
		URL: "/new-page",
		Timing: PerformanceTiming{
			Load: 5000, // Would be a huge regression but baseline too few
		},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 0 {
		t.Errorf("Expected 0 alerts when baseline SampleCount<1, got %d", len(cm.pendingAlerts))
	}
}

func TestDetectAndStoreAlerts_NoRegression_ResolvesExisting(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// First: create a regression alert
	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load: 500,
		},
	}
	snapshot := PerformanceSnapshot{
		URL: "/resolve-page",
		Timing: PerformanceTiming{
			Load: 1500, // >20% regression
		},
	}
	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert after regression, got %d", len(cm.pendingAlerts))
	}
	cm.mu.Unlock()

	// Now: send a snapshot that does NOT regress — should resolve the alert
	goodSnapshot := PerformanceSnapshot{
		URL: "/resolve-page",
		Timing: PerformanceTiming{
			Load: 500, // Same as baseline
		},
	}
	cm.DetectAndStoreAlerts(goodSnapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()
	if len(cm.pendingAlerts) != 0 {
		t.Errorf("Expected alert to be resolved, got %d pending", len(cm.pendingAlerts))
	}
}

func TestDetectAndStoreAlerts_LCPRegression(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baselineLCP := 1000.0
	snapshotLCP := 1500.0 // >20% increase (50%)

	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:                   1000,
			LargestContentfulPaint: &baselineLCP,
		},
	}

	snapshot := PerformanceSnapshot{
		URL: "/lcp-page",
		Timing: PerformanceTiming{
			Load:                   1000, // No load regression
			LargestContentfulPaint: &snapshotLCP,
		},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	if _, ok := cm.pendingAlerts[0].Metrics["lcp"]; !ok {
		t.Error("Expected LCP metric in alert")
	}
}

func TestDetectAndStoreAlerts_TTFBRegression(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load:            1000,
			TimeToFirstByte: 100,
		},
	}

	snapshot := PerformanceSnapshot{
		URL: "/ttfb-page",
		Timing: PerformanceTiming{
			Load:            1000, // No load regression
			TimeToFirstByte: 200,  // >50% increase (100%)
		},
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	if _, ok := cm.pendingAlerts[0].Metrics["ttfb"]; !ok {
		t.Error("Expected TTFB metric in alert")
	}
}

// ============================================
// Coverage: buildAlertSummary — fallback metrics
// ============================================

func TestBuildAlertSummary_FallbackToNonLoadMetric(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// When there's no "load" metric, buildAlertSummary should use a fallback
	metrics := map[string]AlertMetricDelta{
		"cls": {
			Baseline: 0.05,
			Current:  0.25,
			DeltaMs:  0.2,
			DeltaPct: 400.0,
		},
	}

	summary := cm.buildAlertSummary("/test", metrics)

	if !strings.Contains(summary, "cls") {
		t.Errorf("Expected summary to mention 'cls', got '%s'", summary)
	}
	if !strings.Contains(summary, "/test") {
		t.Errorf("Expected summary to mention URL '/test', got '%s'", summary)
	}
}

func TestBuildAlertSummary_LoadMetricPreferred(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	// When "load" metric is present, it should be used for the summary
	metrics := map[string]AlertMetricDelta{
		"load": {
			Baseline: 500,
			Current:  1000,
			DeltaMs:  500,
			DeltaPct: 100.0,
		},
		"cls": {
			Baseline: 0.05,
			Current:  0.2,
			DeltaMs:  0.15,
			DeltaPct: 300.0,
		},
	}

	summary := cm.buildAlertSummary("/page", metrics)

	if !strings.Contains(summary, "Load time regressed") {
		t.Errorf("Expected summary to start with 'Load time regressed', got '%s'", summary)
	}
	if !strings.Contains(summary, "500ms") {
		t.Errorf("Expected summary to mention delta ms, got '%s'", summary)
	}
}

// ============================================
// Coverage: computeNetworkDiff — capping branches
// ============================================

func TestComputeNetworkDiff_CapsFailures(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add more than 50 network failures (all new endpoints with status >= 400)
	capture.mu.Lock()
	for i := 0; i < 55; i++ {
		capture.networkBodies = append(capture.networkBodies, NetworkBody{
			URL:    fmt.Sprintf("https://example.com/api/fail-%d", i),
			Status: 500,
		})
	}
	capture.networkTotalAdded = 55
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff to be non-nil")
	}
	if len(resp.Network.Failures) > 50 {
		t.Errorf("Expected failures capped at 50, got %d", len(resp.Network.Failures))
	}
}

func TestComputeNetworkDiff_CapsNewEndpoints(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Add more than 50 new successful endpoints
	capture.mu.Lock()
	for i := 0; i < 55; i++ {
		capture.networkBodies = append(capture.networkBodies, NetworkBody{
			URL:    fmt.Sprintf("https://example.com/api/new-%d", i),
			Status: 200,
		})
	}
	capture.networkTotalAdded = 55
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff to be non-nil")
	}
	if len(resp.Network.NewEndpoints) > 50 {
		t.Errorf("Expected new endpoints capped at 50, got %d", len(resp.Network.NewEndpoints))
	}
}

func TestComputeNetworkDiff_CapsDegraded(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// First, establish a checkpoint with known endpoints
	capture.mu.Lock()
	for i := 0; i < 55; i++ {
		capture.networkBodies = append(capture.networkBodies, NetworkBody{
			URL:      fmt.Sprintf("https://example.com/api/slow-%d", i),
			Status:   200,
			Duration: 100,
		})
	}
	capture.networkTotalAdded = 55
	capture.mu.Unlock()

	// Get changes to establish checkpoint with known endpoints
	cm.GetChangesSince(GetChangesSinceParams{})

	// Now add the same endpoints but much slower (>2x baseline)
	capture.mu.Lock()
	capture.networkBodies = capture.networkBodies[:0]
	for i := 0; i < 55; i++ {
		capture.networkBodies = append(capture.networkBodies, NetworkBody{
			URL:      fmt.Sprintf("https://example.com/api/slow-%d", i),
			Status:   200,
			Duration: 500, // Much slower than baseline of 100
		})
	}
	capture.networkTotalAdded = 110
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff to be non-nil")
	}
	if len(resp.Network.Degraded) > 50 {
		t.Errorf("Expected degraded capped at 50, got %d", len(resp.Network.Degraded))
	}
}

// ============================================
// Coverage: computeNetworkDiff — toRead > available branch
// ============================================

func TestComputeNetworkDiff_ToReadExceedsAvailable(t *testing.T) {
	cm, _, capture := setupCheckpointTest(t)

	// Establish checkpoint
	cm.GetChangesSince(GetChangesSinceParams{})

	// Set networkTotalAdded high but only have a few bodies in the buffer
	// This simulates ring buffer eviction
	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies, NetworkBody{
		URL:    "https://example.com/api/test",
		Status: 200,
	})
	capture.networkTotalAdded = 100 // Says 100 were added, but only 1 in buffer
	capture.mu.Unlock()

	resp := cm.GetChangesSince(GetChangesSinceParams{})

	if resp.Network == nil {
		t.Fatal("Expected Network diff to be non-nil")
	}
	// Should still work with the available bodies
	if resp.Network.TotalNew != 1 {
		t.Errorf("Expected TotalNew=1 (clamped to available), got %d", resp.Network.TotalNew)
	}
}

// ============================================
// Coverage: CLS regression with baseline CLS == 0
// ============================================

func TestDetectAndStoreAlerts_CLSRegressionWithZeroBaseline(t *testing.T) {
	cm, _, _ := setupCheckpointTest(t)

	baselineCLS := 0.0  // Zero baseline CLS
	snapshotCLS := 0.15 // >0.1 absolute increase

	baseline := PerformanceBaseline{
		SampleCount: 3,
		Timing: BaselineTiming{
			Load: 1000,
		},
		CLS: &baselineCLS,
	}

	snapshot := PerformanceSnapshot{
		URL: "/cls-zero-baseline",
		Timing: PerformanceTiming{
			Load: 1000,
		},
		CLS: &snapshotCLS,
	}

	cm.DetectAndStoreAlerts(snapshot, baseline)

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.pendingAlerts) != 1 {
		t.Fatalf("Expected 1 alert, got %d", len(cm.pendingAlerts))
	}
	clsMetric, ok := cm.pendingAlerts[0].Metrics["cls"]
	if !ok {
		t.Fatal("Expected CLS metric in alert")
	}
	// When baseline is 0, DeltaPct should be 0 (special case in code)
	if clsMetric.DeltaPct != 0 {
		t.Errorf("Expected DeltaPct=0 when baseline CLS is 0, got %f", clsMetric.DeltaPct)
	}
}

// ============================================
// Coverage: buildAlertSummary in alerts.go — singular count
// ============================================

func TestBuildAlertSummaryAlerts_SingleCategory(t *testing.T) {
	alerts := []Alert{
		{Category: "regression", Title: "test"},
	}
	summary := buildAlertSummary(alerts)
	if !strings.Contains(summary, "1 regression") {
		t.Errorf("Expected '1 regression' in summary, got '%s'", summary)
	}
	if !strings.Contains(summary, "1 alerts:") {
		t.Errorf("Expected '1 alerts:' prefix, got '%s'", summary)
	}
}

func TestBuildAlertSummaryAlerts_MultipleCategoriesWithSingulars(t *testing.T) {
	alerts := []Alert{
		{Category: "regression", Title: "r1"},
		{Category: "anomaly", Title: "a1"},
		{Category: "ci", Title: "c1"},
	}
	summary := buildAlertSummary(alerts)
	if !strings.Contains(summary, "1 regression") {
		t.Errorf("Expected '1 regression' in summary, got '%s'", summary)
	}
	if !strings.Contains(summary, "1 anomaly") {
		t.Errorf("Expected '1 anomaly' in summary, got '%s'", summary)
	}
	if !strings.Contains(summary, "1 ci") {
		t.Errorf("Expected '1 ci' in summary, got '%s'", summary)
	}
}
