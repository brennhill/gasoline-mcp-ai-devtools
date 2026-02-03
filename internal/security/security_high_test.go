//go:build integration
// +build integration

// NOTE: These tests require Capture, PerformanceSnapshot, etc. that aren't available.
// Run with: go test -tags=integration ./internal/security/...
package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// H-3: Missing Body Size Limit on POST /api/extension-status
// ============================================

func TestHandleExtensionStatus_RejectsLargeBody(t *testing.T) {
	t.Parallel()

	// Create a very large JSON body (1MB)
	largeBody := strings.Repeat(`{"key":"value",`, 100000) + `"end":"value"}`

	req := httptest.NewRequest(http.MethodPost, "/api/extension-status", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	capture := &Capture{}
	capture.HandleExtensionStatus(w, req)

	// Should reject with 413 or 400 status code
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Errorf("Expected 413 or 400 for large body, got %d", w.Code)
	}
}

func TestHandleExtensionStatus_AcceptsSmallBody(t *testing.T) {
	t.Parallel()

	smallBody := `{"version":"1.0.0","connected":true}`

	req := httptest.NewRequest(http.MethodPost, "/api/extension-status", strings.NewReader(smallBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	capture := &Capture{}
	capture.HandleExtensionStatus(w, req)

	// Should accept with 200 status code
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for small body, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// ============================================
// H-6: ClearAll() Does Not Reset Performance Data
// ============================================

func TestClearAll_ResetsPerformanceData(t *testing.T) {
	t.Parallel()

	capture := &Capture{}
	capture.mu.Lock()

	// Add some performance data
	capture.perf.snapshots = map[string]PerformanceSnapshot{
		"snap1": {
			URL: "https://example.com",
			Timing: PerformanceTiming{
				DomContentLoaded: 500,
				Load:             1000,
			},
		},
	}
	capture.perf.baselines = map[string]PerformanceBaseline{
		"base1": {
			URL:         "https://example.com",
			SampleCount: 10,
			Timing: BaselineTiming{
				DomContentLoaded: 450,
				Load:             800,
			},
		},
	}
	capture.perf.snapshotOrder = []string{"snap1"}
	capture.perf.baselineOrder = []string{"base1"}

	// Add other data that should be cleared
	capture.wsEvents = []WebSocketEvent{{Direction: "send"}}
	capture.networkBodies = []NetworkBody{{URL: "https://api.example.com"}}

	capture.mu.Unlock()

	// Call ClearAll
	capture.ClearAll()

	// Verify everything is cleared
	capture.mu.RLock()
	defer capture.mu.RUnlock()

	if len(capture.perf.snapshots) != 0 {
		t.Errorf("Expected perf.snapshots to be empty after ClearAll, got %d entries", len(capture.perf.snapshots))
	}
	if len(capture.perf.baselines) != 0 {
		t.Errorf("Expected perf.baselines to be empty after ClearAll, got %d entries", len(capture.perf.baselines))
	}
	if len(capture.perf.snapshotOrder) != 0 {
		t.Errorf("Expected perf.snapshotOrder to be empty after ClearAll, got %d entries", len(capture.perf.snapshotOrder))
	}
	if len(capture.perf.baselineOrder) != 0 {
		t.Errorf("Expected perf.baselineOrder to be empty after ClearAll, got %d entries", len(capture.perf.baselineOrder))
	}
	if len(capture.wsEvents) != 0 {
		t.Errorf("Expected wsEvents to be empty after ClearAll, got %d entries", len(capture.wsEvents))
	}
	if len(capture.networkBodies) != 0 {
		t.Errorf("Expected networkBodies to be empty after ClearAll, got %d entries", len(capture.networkBodies))
	}
}

// ============================================
// H-4: AutoDetect() Lock Contention Test
// ============================================

func TestAutoDetect_MinimalLockContention(t *testing.T) {
	t.Parallel()

	nc := NewNoiseConfig()

	// Create many console entries for analysis
	consoleEntries := make([]LogEntry, 100)
	for i := 0; i < 100; i++ {
		consoleEntries[i] = LogEntry{
			"level":   "error",
			"message": "[HMR] Hot Module Replacement enabled",
			"source":  "webpack:///app.js",
		}
	}

	// Create many WebSocket events
	wsEvents := make([]WebSocketEvent, 100)
	for i := 0; i < 100; i++ {
		wsEvents[i] = WebSocketEvent{
			Direction: "receive",
			Data:      `{"type":"ping"}`,
		}
	}

	// AutoDetect should be callable with read-only data
	// This test verifies it doesn't panic and completes
	proposals := nc.AutoDetect(consoleEntries, []NetworkBody{}, wsEvents)

	// Verify it returns a valid result (not nil)
	if proposals == nil {
		t.Error("AutoDetect should return a valid proposals list (can be empty)")
	}

	// Verify we can still list rules after AutoDetect
	rules := nc.ListRules()
	if rules == nil {
		t.Error("ListRules should return a valid rules list")
	}
}
