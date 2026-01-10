// health_test.go — Tests for Health & SLA Metrics (Tier 3.4).
// TDD: Tests written first to define the expected behavior.
package main

import (
	"encoding/json"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================
// HealthMetrics Struct Tests
// ============================================

func TestHealthMetrics_InitialState(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	if hm.startTime.IsZero() {
		t.Error("startTime should be set on creation")
	}

	if hm.requestCounts == nil {
		t.Error("requestCounts map should be initialized")
	}

	if hm.errorCounts == nil {
		t.Error("errorCounts map should be initialized")
	}
}

func TestHealthMetrics_IncrementRequest(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	hm.IncrementRequest("observe")
	hm.IncrementRequest("observe")
	hm.IncrementRequest("analyze")

	if hm.GetRequestCount("observe") != 2 {
		t.Errorf("expected observe count 2, got %d", hm.GetRequestCount("observe"))
	}

	if hm.GetRequestCount("analyze") != 1 {
		t.Errorf("expected analyze count 1, got %d", hm.GetRequestCount("analyze"))
	}

	if hm.GetRequestCount("nonexistent") != 0 {
		t.Errorf("expected nonexistent count 0, got %d", hm.GetRequestCount("nonexistent"))
	}
}

func TestHealthMetrics_IncrementError(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	hm.IncrementError("observe")
	hm.IncrementError("observe")
	hm.IncrementError("query_dom")

	if hm.GetErrorCount("observe") != 2 {
		t.Errorf("expected observe error count 2, got %d", hm.GetErrorCount("observe"))
	}

	if hm.GetErrorCount("query_dom") != 1 {
		t.Errorf("expected query_dom error count 1, got %d", hm.GetErrorCount("query_dom"))
	}
}

func TestHealthMetrics_TotalRequests(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	hm.IncrementRequest("observe")
	hm.IncrementRequest("analyze")
	hm.IncrementRequest("observe")

	if hm.GetTotalRequests() != 3 {
		t.Errorf("expected total requests 3, got %d", hm.GetTotalRequests())
	}
}

func TestHealthMetrics_TotalErrors(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	hm.IncrementError("observe")
	hm.IncrementError("analyze")

	if hm.GetTotalErrors() != 2 {
		t.Errorf("expected total errors 2, got %d", hm.GetTotalErrors())
	}
}

func TestHealthMetrics_Uptime(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	// Uptime should be at least 0
	uptime := hm.GetUptime()
	if uptime < 0 {
		t.Errorf("uptime should be non-negative, got %v", uptime)
	}

	// Sleep briefly and verify uptime increases
	time.Sleep(10 * time.Millisecond)
	newUptime := hm.GetUptime()
	if newUptime <= uptime {
		t.Errorf("uptime should increase over time")
	}
}

func TestHealthMetrics_ThreadSafety(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent request increments
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				hm.IncrementRequest("observe")
			}
		}()
	}

	// Concurrent error increments
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				hm.IncrementError("observe")
			}
		}()
	}

	// Concurrent reads
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = hm.GetRequestCount("observe")
				_ = hm.GetTotalRequests()
				_ = hm.GetUptime()
			}
		}()
	}

	wg.Wait()

	// Verify final counts are correct
	if hm.GetRequestCount("observe") != int64(4*iterations) {
		t.Errorf("expected request count %d, got %d", 4*iterations, hm.GetRequestCount("observe"))
	}

	if hm.GetErrorCount("observe") != int64(4*iterations) {
		t.Errorf("expected error count %d, got %d", 4*iterations, hm.GetErrorCount("observe"))
	}
}

// ============================================
// GetHealth Response Tests
// ============================================

func TestGetHealth_ResponseStructure(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	// Simulate some activity
	hm.IncrementRequest("observe")
	hm.IncrementRequest("analyze")
	hm.IncrementError("query_dom")

	// Create a mock capture for buffer data
	capture := NewCapture()

	response := hm.GetHealth(capture, nil, version)

	// Verify server info
	if response.Server.Version == "" {
		t.Error("server version should be set")
	}
	if response.Server.PID == 0 {
		t.Error("server PID should be set")
	}
	if response.Server.Platform == "" {
		t.Error("server platform should be set")
	}
	if response.Server.GoVersion == "" {
		t.Error("server go version should be set")
	}
	if response.Server.UptimeSeconds < 0 {
		t.Error("uptime should be non-negative")
	}

	// Verify memory info
	if response.Memory.CurrentMB < 0 {
		t.Error("current memory should be non-negative")
	}
	if response.Memory.HardLimitMB <= 0 {
		t.Error("hard limit should be positive")
	}

	// Verify buffers info
	if response.Buffers.Console.Capacity <= 0 {
		t.Error("console capacity should be positive")
	}
	if response.Buffers.Network.Capacity <= 0 {
		t.Error("network capacity should be positive")
	}
	if response.Buffers.WebSocket.Capacity <= 0 {
		t.Error("websocket capacity should be positive")
	}
	if response.Buffers.Actions.Capacity <= 0 {
		t.Error("actions capacity should be positive")
	}

	// Verify audit info
	// We incremented 2 requests (observe, analyze) and 1 error (query_dom)
	if response.Audit.TotalCalls != 2 {
		t.Errorf("expected total calls 2, got %d", response.Audit.TotalCalls)
	}
	if response.Audit.TotalErrors != 1 {
		t.Errorf("expected total errors 1, got %d", response.Audit.TotalErrors)
	}
	if response.Audit.CallsPerTool == nil {
		t.Error("calls per tool should be set")
	}
	if response.Audit.CallsPerTool["observe"] != 1 {
		t.Errorf("expected observe calls 1, got %d", response.Audit.CallsPerTool["observe"])
	}
}

func TestGetHealth_MemoryStats(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()
	capture := NewCapture()

	response := hm.GetHealth(capture, nil, version)

	// Memory should be populated from runtime.MemStats
	if response.Memory.CurrentMB == 0 && response.Memory.AllocMB == 0 {
		t.Error("memory stats should have some non-zero values")
	}

	// Hard limit should match the constant
	expectedHardLimitMB := float64(memoryHardLimit) / (1024 * 1024)
	if response.Memory.HardLimitMB != expectedHardLimitMB {
		t.Errorf("expected hard limit %.2f MB, got %.2f MB", expectedHardLimitMB, response.Memory.HardLimitMB)
	}
}

func TestGetHealth_BufferUtilization(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()
	capture := NewCapture()

	// Add some data to buffers
	capture.AddWebSocketEvents([]WebSocketEvent{
		{ID: "test1", Event: "message"},
		{ID: "test2", Event: "message"},
	})

	response := hm.GetHealth(capture, nil, version)

	// WebSocket buffer should show 2 entries
	if response.Buffers.WebSocket.Entries != 2 {
		t.Errorf("expected 2 websocket entries, got %d", response.Buffers.WebSocket.Entries)
	}

	// Utilization should be (2/maxWSEvents)*100
	expectedUtil := float64(2) / float64(maxWSEvents) * 100
	if response.Buffers.WebSocket.UtilizationPct != expectedUtil {
		t.Errorf("expected utilization %.2f%%, got %.2f%%", expectedUtil, response.Buffers.WebSocket.UtilizationPct)
	}
}

func TestGetHealth_RateLimiting(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()
	capture := NewCapture()

	response := hm.GetHealth(capture, nil, version)

	// Rate limiting info should be present
	if response.RateLimiting.Threshold <= 0 {
		t.Error("rate limit threshold should be positive")
	}
	// CircuitOpen is a boolean, so just verify it exists
	// (no error if false is the default)
}

func TestGetHealth_ErrorRate(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()
	capture := NewCapture()

	// 10 requests, 2 errors = 20% error rate
	for i := 0; i < 10; i++ {
		hm.IncrementRequest("observe")
	}
	hm.IncrementError("observe")
	hm.IncrementError("observe")

	response := hm.GetHealth(capture, nil, version)

	expectedRate := 20.0 // 2/10 * 100
	if response.Audit.ErrorRatePct != expectedRate {
		t.Errorf("expected error rate %.2f%%, got %.2f%%", expectedRate, response.Audit.ErrorRatePct)
	}
}

func TestGetHealth_ZeroDivision(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()
	capture := NewCapture()

	// No requests yet - error rate should be 0, not NaN or panic
	response := hm.GetHealth(capture, nil, version)

	if response.Audit.ErrorRatePct != 0 {
		t.Errorf("expected 0%% error rate with no requests, got %.2f%%", response.Audit.ErrorRatePct)
	}
}

func TestGetHealth_JSONSerialization(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()
	capture := NewCapture()

	response := hm.GetHealth(capture, nil, version)

	// Verify it serializes to valid JSON
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal health response: %v", err)
	}

	// Verify it deserializes back
	var decoded MCPHealthResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}

	// Spot check some fields
	if decoded.Server.GoVersion != runtime.Version() {
		t.Errorf("expected go version %s, got %s", runtime.Version(), decoded.Server.GoVersion)
	}
}

// ============================================
// Integration with ToolHandler
// ============================================

func TestToolHandler_GetHealthTool(t *testing.T) {
	t.Parallel()
	// Create a minimal server setup
	server := &Server{
		maxEntries: defaultMaxEntries,
		entries:    make([]LogEntry, 0),
	}
	capture := NewCapture()

	// Create the tool handler
	mcpHandler := &MCPHandler{server: server}
	handler := &ToolHandler{
		MCPHandler:    mcpHandler,
		capture:       capture,
		noise:         NewNoiseConfig(),
		healthMetrics: NewHealthMetrics(),
	}

	// Call the get_health tool
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
	}

	resp := handler.toolGetHealth(req)

	// Should not be an error
	if resp.Error != nil {
		t.Fatalf("get_health returned error: %v", resp.Error)
	}

	// Parse the result
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// Parse the health response from the text content (strip summary line)
	text := result.Content[0].Text
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) < 2 {
		t.Fatalf("expected summary + JSON, got: %s", text)
	}
	if lines[0] != "Server health" {
		t.Errorf("expected summary 'Server health', got %q", lines[0])
	}
	var health MCPHealthResponse
	if err := json.Unmarshal([]byte(lines[1]), &health); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}

	// Verify basic fields
	if health.Server.Version == "" {
		t.Error("version should be set")
	}
}

// ============================================
// Edge Cases
// ============================================

func TestHealthMetrics_RequestCountOverflow(t *testing.T) {
	t.Parallel()
	// This tests that we handle large counts without issues
	// In practice, int64 won't overflow during normal operation
	hm := NewHealthMetrics()

	// Simulate many requests
	for i := 0; i < 10000; i++ {
		hm.IncrementRequest("stress_test")
	}

	if hm.GetRequestCount("stress_test") != 10000 {
		t.Errorf("expected 10000 requests, got %d", hm.GetRequestCount("stress_test"))
	}
}

func TestHealthMetrics_ManyTools(t *testing.T) {
	t.Parallel()
	hm := NewHealthMetrics()

	tools := []string{
		"observe", "generate", "configure", "interact",
	}

	for _, tool := range tools {
		hm.IncrementRequest(tool)
	}

	if hm.GetTotalRequests() != int64(len(tools)) {
		t.Errorf("expected %d total requests, got %d", len(tools), hm.GetTotalRequests())
	}

	// Verify each tool has count 1
	for _, tool := range tools {
		if hm.GetRequestCount(tool) != 1 {
			t.Errorf("expected 1 request for %s, got %d", tool, hm.GetRequestCount(tool))
		}
	}
}
