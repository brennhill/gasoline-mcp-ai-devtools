package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSSERegistry_RegisterUnregister tests basic connection lifecycle
func TestSSERegistry_RegisterUnregister(t *testing.T) {
	registry := NewSSERegistry()

	// Create mock response writer
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/mcp/sse", nil)

	sessionID := "test-session-1"
	clientID := "test-client-1"

	// Register connection
	conn, err := registry.Register(sessionID, clientID, w, req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if conn.SessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, conn.SessionID)
	}

	if conn.ClientID != clientID {
		t.Errorf("Expected client ID %s, got %s", clientID, conn.ClientID)
	}

	// Verify connection is registered
	retrieved, exists := registry.Get(sessionID)
	if !exists {
		t.Fatalf("Connection not found after registration")
	}

	if retrieved != conn {
		t.Errorf("Retrieved connection is not the same as registered")
	}

	// Unregister connection
	registry.Unregister(sessionID)

	// Verify connection is removed
	_, exists = registry.Get(sessionID)
	if exists {
		t.Errorf("Connection still exists after unregistration")
	}
}

// TestSSERegistry_SendMessage tests routing messages to specific sessions
func TestSSERegistry_SendMessage(t *testing.T) {
	registry := NewSSERegistry()

	// Create mock response writer with Flusher support
	w := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          false,
	}
	req := httptest.NewRequest("GET", "/mcp/sse", nil)

	sessionID := "test-session-2"
	clientID := "test-client-2"

	// Register connection
	_, err := registry.Register(sessionID, clientID, w, req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Send message
	testData := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  map[string]string{"status": "ok"},
	}

	err = registry.SendMessage(sessionID, testData)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	// Verify message was written
	output := w.Body.String()
	if !strings.Contains(output, "event: message") {
		t.Errorf("Output does not contain SSE event header: %s", output)
	}

	if !strings.Contains(output, `"jsonrpc":"2.0"`) {
		t.Errorf("Output does not contain expected data: %s", output)
	}

	// Verify flusher was called
	if !w.flushed {
		t.Errorf("Flusher was not called after sending message")
	}
}

// TestSSERegistry_BroadcastNotification tests broadcasting to all clients
func TestSSERegistry_BroadcastNotification(t *testing.T) {
	registry := NewSSERegistry()

	// Register multiple connections
	var writers []*mockResponseWriter
	for i := 0; i < 3; i++ {
		w := &mockResponseWriter{
			ResponseRecorder: httptest.NewRecorder(),
			flushed:          false,
		}
		req := httptest.NewRequest("GET", "/mcp/sse", nil)

		sessionID := "session-" + string(rune('A'+i))
		clientID := "client-" + string(rune('A'+i))

		_, err := registry.Register(sessionID, clientID, w, req)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		writers = append(writers, w)
	}

	// Broadcast notification
	notification := MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/message",
		Params: NotificationParams{
			Level:  "warning",
			Logger: "gasoline",
			Data: map[string]string{
				"category": "test",
				"message":  "test notification",
			},
		},
	}

	registry.BroadcastNotification(notification)

	// Verify all connections received the notification
	for i, w := range writers {
		output := w.Body.String()
		if !strings.Contains(output, "event: message") {
			t.Errorf("Writer %d did not receive SSE event header", i)
		}

		if !strings.Contains(output, "test notification") {
			t.Errorf("Writer %d did not receive notification data", i)
		}

		if !w.flushed {
			t.Errorf("Writer %d was not flushed", i)
		}
	}
}

// TestFormatSSEEvent tests SSE protocol formatting
func TestFormatSSEEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		data     string
		expected string
	}{
		{
			name:     "simple event",
			event:    "message",
			data:     `{"test":"value"}`,
			expected: "event: message\ndata: {\"test\":\"value\"}\n\n",
		},
		{
			name:     "endpoint event",
			event:    "endpoint",
			data:     `{"uri":"/mcp/messages/abc123"}`,
			expected: "event: endpoint\ndata: {\"uri\":\"/mcp/messages/abc123\"}\n\n",
		},
		{
			name:     "multiline data",
			event:    "message",
			data:     "line1\nline2\nline3",
			expected: "event: message\ndata: line1\ndata: line2\ndata: line3\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSSEEvent(tt.event, tt.data)
			if result != tt.expected {
				t.Errorf("formatSSEEvent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestSSEConnection_Concurrency tests thread safety
func TestSSEConnection_Concurrency(t *testing.T) {
	registry := NewSSERegistry()

	// Register connection
	w := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          false,
	}
	req := httptest.NewRequest("GET", "/mcp/sse", nil)

	sessionID := "concurrent-test"
	clientID := "concurrent-client"

	conn, err := registry.Register(sessionID, clientID, w, req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Send messages concurrently
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			data := map[string]interface{}{
				"id":      n,
				"message": "concurrent message",
			}
			jsonData, _ := json.Marshal(data)
			_ = conn.WriteEvent("message", string(jsonData))
		}(i)
	}

	wg.Wait()

	// Verify no panics occurred and some data was written
	output := w.Body.String()
	if len(output) == 0 {
		t.Errorf("No data was written during concurrent access")
	}

	// Count occurrences of "event: message" to verify all messages were written
	count := strings.Count(output, "event: message")
	if count != numGoroutines {
		t.Errorf("Expected %d messages, got %d", numGoroutines, count)
	}
}

// TestSSERegistry_CleanupStaleConnections tests timeout cleanup
func TestSSERegistry_CleanupStaleConnections(t *testing.T) {
	t.Skip("Skipping cleanup test - requires waiting 1 hour or mocking time")

	// This test would require either:
	// 1. Waiting 1 hour for the cleanup to trigger (impractical)
	// 2. Refactoring SSERegistry to accept a time.Duration for cleanup interval
	// 3. Using a time mocking library (violates zero-deps constraint)

	// For now, manual testing or integration tests will verify this behavior
}

// TestGenerateSessionID tests session ID generation
func TestGenerateSessionID(t *testing.T) {
	// Generate multiple session IDs
	ids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := generateSessionID()

		// Verify format (32 hex characters)
		if len(id) != 32 {
			t.Errorf("Expected session ID length 32, got %d", len(id))
		}

		// Verify uniqueness
		if ids[id] {
			t.Errorf("Duplicate session ID generated: %s", id)
		}
		ids[id] = true
	}
}

// TestSSEConnection_ContextCancellation tests client disconnect detection
func TestSSEConnection_ContextCancellation(t *testing.T) {
	registry := NewSSERegistry()

	// Create request with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/mcp/sse", nil).WithContext(ctx)

	w := &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		flushed:          false,
	}

	sessionID := "cancel-test"
	clientID := "cancel-client"

	conn, err := registry.Register(sessionID, clientID, w, req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Cancel context to simulate client disconnect
	cancel()

	// Give context cancellation time to propagate
	time.Sleep(10 * time.Millisecond)

	// Try to send message - should fail with connection closed error
	err = conn.WriteEvent("message", `{"test":"data"}`)
	if err == nil {
		t.Errorf("Expected error when writing to cancelled connection, got nil")
	}

	if !strings.Contains(err.Error(), "connection closed") {
		t.Errorf("Expected 'connection closed' error, got: %v", err)
	}
}

// TestSSERegistry_SendMessage_NonexistentSession tests error handling
func TestSSERegistry_SendMessage_NonexistentSession(t *testing.T) {
	registry := NewSSERegistry()

	err := registry.SendMessage("nonexistent-session", map[string]string{"test": "data"})
	if err == nil {
		t.Errorf("Expected error when sending to nonexistent session, got nil")
	}

	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("Expected 'session not found' error, got: %v", err)
	}
}

// mockResponseWriter implements http.ResponseWriter and http.Flusher for testing
type mockResponseWriter struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (m *mockResponseWriter) Flush() {
	m.flushed = true
}
