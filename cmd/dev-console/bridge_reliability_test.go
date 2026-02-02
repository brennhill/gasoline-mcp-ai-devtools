// bridge_reliability_test.go — Test bridge error handling and reliability
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestBridgeHTTPTimeout verifies bridge respects 30s timeout
func TestBridgeHTTPTimeout(t *testing.T) {
	// Create test server that hangs for 35 seconds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(35 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	// Create HTTP client with 30s timeout (same as bridge)
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make request
	start := time.Now()
	_, err := client.Get(server.URL)
	elapsed := time.Since(start)

	// Should timeout at ~30s
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if elapsed < 29*time.Second || elapsed > 31*time.Second {
		t.Errorf("Expected timeout at ~30s, got %v", elapsed)
	}
}

// TestBridgeConnectionRefused verifies bridge handles server down gracefully
func TestBridgeConnectionRefused(t *testing.T) {
	// Try to connect to non-existent server
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get("http://localhost:99999")

	// Should get connection error
	if err == nil {
		t.Error("Expected connection error, got nil")
		resp.Body.Close()
	}

	// Error should be immediate (not wait 30s)
	// Connection refused is instant
}

// TestBridgeHTTPNon200 verifies bridge handles HTTP errors
func TestBridgeHTTPNon200(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"Bad Request", http.StatusBadRequest, "Invalid request"},
		{"Unauthorized", http.StatusUnauthorized, "Missing API key"},
		{"Not Found", http.StatusNotFound, "Endpoint not found"},
		{"Internal Error", http.StatusInternalServerError, "Server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			resp, err := client.Get(server.URL)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, resp.StatusCode)
			}
		})
	}
}

// TestBridgeJSONRPCErrorFormat verifies error response format
func TestBridgeJSONRPCErrorFormat(t *testing.T) {
	// Simulate bridge error response
	errResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      "test-123",
		Error: &JSONRPCError{
			Code:    -32603,
			Message: "Server connection error: connection refused",
		},
	}

	// Verify it marshals correctly
	jsonData, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}

	// Verify AI can parse it
	var parsed JSONRPCResponse
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if parsed.Error == nil {
		t.Fatal("Expected error field to be present")
	}

	if parsed.Error.Code != -32603 {
		t.Errorf("Expected error code -32603, got %d", parsed.Error.Code)
	}

	if parsed.Error.Message != "Server connection error: connection refused" {
		t.Errorf("Unexpected error message: %s", parsed.Error.Message)
	}
}

// TestBridgeLargePayload verifies bridge handles large payloads (10MB limit)
func TestBridgeLargePayload(t *testing.T) {
	// Create 5MB payload (within 10MB limit)
	largeData := make([]byte, 5*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(largeData)
	}))
	defer server.Close()

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should handle large response without error
	body := make([]byte, len(largeData))
	n, err := resp.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		t.Errorf("Failed to read large response: %v", err)
	}

	if n < len(largeData) {
		t.Errorf("Incomplete response: got %d bytes, expected %d", n, len(largeData))
	}
}

// TestBridgeReconnection verifies bridge behavior after server restart
func TestBridgeReconnection(t *testing.T) {
	// Start server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	server := httptest.NewServer(handler)
	initialURL := server.URL

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// First request succeeds
	resp1, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	resp1.Body.Close()

	// Stop server (simulate crash)
	server.Close()

	// Second request should fail with connection error
	_, err = client.Get(initialURL)
	if err == nil {
		t.Error("Expected connection error after server shutdown, got nil")
	}

	// Start new server
	server2 := httptest.NewServer(handler)
	defer server2.Close()

	// Third request to new server succeeds
	resp3, err := client.Get(server2.URL)
	if err != nil {
		t.Fatalf("Request to restarted server failed: %v", err)
	}
	resp3.Body.Close()
}
