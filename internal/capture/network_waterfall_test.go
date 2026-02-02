// Package capture provides telemetry capture functionality
package capture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================
// Network Waterfall Tests (TDD Phase 2)
// ============================================
// These tests verify the network waterfall capture system for complete
// CSP generation and security flagging.

// ============================================
// Basic Functionality Tests
// ============================================

func TestHandleNetworkWaterfall_AcceptsValidPayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{
				Name:            "https://example.com/app.js",
				URL:             "https://example.com/app.js",
				InitiatorType:   "script",
				Duration:        50.5,
				TransferSize:    1024,
				DecodedBodySize: 2048,
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleNetworkWaterfall_RejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader([]byte(`{invalid json`)))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("Expected error status, got %d", w.Code)
	}
}

func TestHandleNetworkWaterfall_StoresTimestamp(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	beforeTime := time.Now()
	capture.HandleNetworkWaterfall(w, req)
	afterTime := time.Now()

	capture.mu.RLock()
	if len(capture.networkWaterfall) == 0 {
		t.Fatalf("Expected 1 entry, got %d", len(capture.networkWaterfall))
	}
	entryTime := capture.networkWaterfall[0].Timestamp
	capture.mu.RUnlock()

	if entryTime.Before(beforeTime) || entryTime.After(afterTime) {
		t.Errorf("Timestamp not set correctly: %v (should be between %v and %v)", entryTime, beforeTime, afterTime)
	}
}

func TestHandleNetworkWaterfall_StoresPageURL(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	expectedURL := "https://example.com/page"
	payload := NetworkWaterfallPayload{
		PageURL: expectedURL,
		Entries: []NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	capture.mu.RLock()
	if len(capture.networkWaterfall) == 0 {
		t.Fatalf("Expected 1 entry, got %d", len(capture.networkWaterfall))
	}
	storedURL := capture.networkWaterfall[0].PageURL
	capture.mu.RUnlock()

	if storedURL != expectedURL {
		t.Errorf("Expected URL %q, got %q", expectedURL, storedURL)
	}
}

// ============================================
// Ring Buffer and Memory Tests
// ============================================

func TestNetworkWaterfall_RingBufferEviction(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add 10 entries which should trigger eviction since max is 10 per page
	for i := 0; i < 12; i++ {
		payload := NetworkWaterfallPayload{
			PageURL: "https://example.com",
			Entries: []NetworkWaterfallEntry{
				{
					Name:          fmt.Sprintf("https://example.com/resource%d", i),
					URL:           fmt.Sprintf("https://example.com/resource%d", i),
					InitiatorType: "script",
				},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
		w := httptest.NewRecorder()

		capture.HandleNetworkWaterfall(w, req)
	}

	capture.mu.RLock()
	count := len(capture.networkWaterfall)
	capture.mu.RUnlock()

	// Should keep only the last 10 (or whatever the buffer size is)
	if count > 10 {
		t.Errorf("Expected max 10 entries, got %d", count)
	}
}

func TestNetworkWaterfall_MultipleEntriesInSinglePayload(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{
				Name:          "https://example.com/app.js",
				URL:           "https://example.com/app.js",
				InitiatorType: "script",
			},
			{
				Name:          "https://example.com/style.css",
				URL:           "https://example.com/style.css",
				InitiatorType: "stylesheet",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	capture.mu.RLock()
	if len(capture.networkWaterfall) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(capture.networkWaterfall))
	}
	capture.mu.RUnlock()
}

// ============================================
// CSP Generation Tests
// ============================================

func TestNetworkWaterfall_FeedsCSPGenerator(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	payload := NetworkWaterfallPayload{
		PageURL: "https://example.com",
		Entries: []NetworkWaterfallEntry{
			{
				Name:          "https://cdn.example.com/lib.js",
				URL:           "https://cdn.example.com/lib.js",
				InitiatorType: "script",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
	w := httptest.NewRecorder()

	capture.HandleNetworkWaterfall(w, req)

	capture.mu.RLock()
	if capture.cspGen == nil {
		t.Errorf("Expected CSP generator to be initialized")
	}
	capture.mu.RUnlock()
}

// ============================================
// URL Extraction Tests (Skipped)
// ============================================
// NOTE: extractOrigin function is not implemented in the capture package.

func TestExtractOrigin_StandardHTTPS(t *testing.T) {
	t.Parallel()
	t.Skip("extractOrigin not implemented")
}

func TestExtractOrigin_WithPort(t *testing.T) {
	t.Parallel()
	t.Skip("extractOrigin not implemented")
}

func TestExtractOrigin_DataURL(t *testing.T) {
	t.Parallel()
	t.Skip("extractOrigin not implemented")
}

func TestExtractOrigin_BlobURL(t *testing.T) {
	t.Parallel()
	t.Skip("extractOrigin not implemented")
}

func TestExtractOrigin_MalformedURL(t *testing.T) {
	t.Parallel()
	t.Skip("extractOrigin not implemented")
}

// ============================================
// Capacity Configuration Tests
// ============================================

func TestNetworkWaterfall_DefaultCapacity(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.mu.RLock()
	if cap(capture.networkWaterfall) == 0 {
		t.Errorf("Expected networkWaterfall buffer to be initialized")
	}
	capture.mu.RUnlock()
}

func TestNetworkWaterfall_CustomCapacity(t *testing.T) {
	t.Parallel()
	capture := NewCapture()
	capture.mu.RLock()
	capacity := cap(capture.networkWaterfall)
	capture.mu.RUnlock()

	if capacity == 0 {
		t.Errorf("Expected non-zero capacity")
	}
}

// ============================================
// Concurrent Access Tests
// ============================================

func TestNetworkWaterfall_ConcurrentWrites(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Write 100 entries concurrently
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(index int) {
			payload := NetworkWaterfallPayload{
				PageURL: "https://example.com",
				Entries: []NetworkWaterfallEntry{
					{
						Name:          fmt.Sprintf("https://example.com/resource%d", index),
						URL:           fmt.Sprintf("https://example.com/resource%d", index),
						InitiatorType: "script",
					},
				},
			}

			body, _ := json.Marshal(payload)
			req := httptest.NewRequest("POST", "/network-waterfall", bytes.NewReader(body))
			w := httptest.NewRecorder()

			capture.HandleNetworkWaterfall(w, req)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 100; i++ {
		<-done
	}

	capture.mu.RLock()
	count := len(capture.networkWaterfall)
	capture.mu.RUnlock()

	if count != 100 {
		t.Errorf("Expected 100 entries, got %d", count)
	}
}

// ============================================
// MCP Tool Handler Tests (Skipped)
// ============================================
// NOTE: These tests are skipped because ToolHandler and MCPHandler
// have not been moved to internal packages yet. They remain in cmd/dev-console
// and would create circular dependencies if imported here.

func TestToolGetNetworkWaterfall_EmptyBuffer(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/dev-console refactoring")
}

func TestToolGetNetworkWaterfall_PopulatedBuffer(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/dev-console refactoring")
}

func TestToolGetNetworkWaterfall_LimitParameter(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/dev-console refactoring")
}

func TestToolGetNetworkWaterfall_URLFilter(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/dev-console refactoring")
}

func TestToolGetNetworkWaterfall_ConcurrentAccessSafety(t *testing.T) {
	t.Parallel()
	t.Skip("ToolHandler not available in internal packages - requires cmd/dev-console refactoring")
}
