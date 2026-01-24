package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================
// Test Scenario 1: Under 1000 events/sec -> all requests accepted (200)
// ============================================

func TestRateLimitUnderThreshold(t *testing.T) {
	c := setupTestCapture(t)

	// Send 500 events (under 1000 threshold)
	for i := 0; i < 500; i++ {
		c.RecordEvents(1)
	}

	if c.CheckRateLimit() {
		t.Error("expected requests under threshold to be accepted, but got rate limited")
	}
}

// ============================================
// Test Scenario 2: Exactly 1001 events in one second -> 1001st returns 429
// ============================================

func TestRateLimitAtThreshold(t *testing.T) {
	c := setupTestCapture(t)

	// Send exactly 1000 events - should still be fine
	c.RecordEvents(1000)
	if c.CheckRateLimit() {
		t.Error("expected 1000 events to be accepted, but got rate limited")
	}

	// The 1001st event should trigger rate limiting
	c.RecordEvents(1)
	if !c.CheckRateLimit() {
		t.Error("expected 1001st event to be rate limited, but was accepted")
	}
}

// ============================================
// Test Scenario 3: Rate resets after 1 second -> requests accepted again
// ============================================

func TestRateLimitResetsAfterOneSecond(t *testing.T) {
	c := setupTestCapture(t)

	// Exceed threshold
	c.RecordEvents(1500)
	if !c.CheckRateLimit() {
		t.Error("expected rate limit after 1500 events")
	}

	// Simulate time passing beyond the 1-second window
	c.mu.Lock()
	c.rateWindowStart = time.Now().Add(-2 * time.Second)
	c.mu.Unlock()

	// Now should be accepted (new window)
	if c.CheckRateLimit() {
		t.Error("expected rate limit to reset after 1 second window expires")
	}
}

// ============================================
// Test Scenario 4: 5 consecutive seconds over threshold -> circuit opens
// ============================================

func TestCircuitOpensAfterFiveSeconds(t *testing.T) {
	c := setupTestCapture(t)

	// Simulate 5 consecutive seconds over threshold
	for i := 0; i < 5; i++ {
		c.mu.Lock()
		c.rateWindowStart = time.Now()
		c.windowEventCount = rateLimitThreshold + 1
		c.rateLimitStreak++
		c.mu.Unlock()
	}

	// Check if circuit is open
	c.mu.Lock()
	c.evaluateCircuit()
	isOpen := c.circuitOpen
	c.mu.Unlock()

	if !isOpen {
		t.Error("expected circuit to open after 5 consecutive seconds over threshold")
	}
}

// ============================================
// Test Scenario 5: Circuit open -> all ingest endpoints return 429 immediately
// ============================================

func TestCircuitOpenRejects(t *testing.T) {
	c := setupTestCapture(t)

	// Manually open the circuit
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now()
	c.mu.Unlock()

	// All ingest checks should fail
	if !c.CheckRateLimit() {
		t.Error("expected circuit open to reject requests")
	}

	// Test via HTTP - WebSocket events endpoint
	payload := `{"events":[{"event":"message","id":"ws1","direction":"incoming","data":"test"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	c.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 when circuit open, got %d", rec.Code)
	}

	// Test network bodies endpoint
	nbPayload := `{"bodies":[{"method":"GET","url":"http://example.com","status":200}]}`
	req2 := httptest.NewRequest("POST", "/network-bodies", strings.NewReader(nbPayload))
	rec2 := httptest.NewRecorder()
	c.HandleNetworkBodies(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for network-bodies when circuit open, got %d", rec2.Code)
	}

	// Test enhanced actions endpoint
	actPayload := `{"actions":[{"type":"click","timestamp":1234567890}]}`
	req3 := httptest.NewRequest("POST", "/enhanced-actions", strings.NewReader(actPayload))
	rec3 := httptest.NewRecorder()
	c.HandleEnhancedActions(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for enhanced-actions when circuit open, got %d", rec3.Code)
	}
}

// ============================================
// Test Scenario 6: Rate below threshold for 10s + memory below 30MB -> circuit closes
// ============================================

func TestCircuitClosesAfterRecovery(t *testing.T) {
	c := setupTestCapture(t)

	// Open the circuit
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now().Add(-15 * time.Second) // opened 15s ago
	c.rateLimitStreak = 0                                 // rate is fine now
	c.windowEventCount = 0                                // no events
	c.rateWindowStart = time.Now()
	c.lastBelowThresholdAt = time.Now().Add(-11 * time.Second) // below threshold for 11s
	c.mem.simulatedMemory = 20 * 1024 * 1024                   // 20MB - well under 30MB
	c.mu.Unlock()

	// Evaluate circuit - should close
	c.mu.Lock()
	c.evaluateCircuit()
	isOpen := c.circuitOpen
	c.mu.Unlock()

	if isOpen {
		t.Error("expected circuit to close after 10s below threshold with memory under 30MB")
	}
}

func TestCircuitStaysOpenIfMemoryHigh(t *testing.T) {
	c := setupTestCapture(t)

	// Open the circuit
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now().Add(-15 * time.Second)
	c.rateLimitStreak = 0
	c.windowEventCount = 0
	c.rateWindowStart = time.Now()
	c.lastBelowThresholdAt = time.Now().Add(-11 * time.Second)
	c.mem.simulatedMemory = 35 * 1024 * 1024 // 35MB - over 30MB threshold
	c.mu.Unlock()

	c.mu.Lock()
	c.evaluateCircuit()
	isOpen := c.circuitOpen
	c.mu.Unlock()

	if !isOpen {
		t.Error("expected circuit to stay open when memory is above 30MB")
	}
}

// ============================================
// Test Scenario 7: 429 response contains correct JSON body and Retry-After header
// ============================================

func TestRateLimitResponseFormat(t *testing.T) {
	c := setupTestCapture(t)

	// Exceed rate limit
	c.RecordEvents(1100)

	// Make a request that should be rate-limited
	payload := `{"events":[{"event":"message","id":"ws1","direction":"incoming","data":"test"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	c.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	// Check Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header to be set")
	}
	if retryAfter != "1" {
		t.Errorf("expected Retry-After to be '1', got '%s'", retryAfter)
	}

	// Check JSON body
	var body struct {
		Error       string `json:"error"`
		Message     string `json:"message"`
		RetryAfter  int    `json:"retry_after_ms"`
		CircuitOpen bool   `json:"circuit_open"`
		CurrentRate int    `json:"current_rate"`
		Threshold   int    `json:"threshold"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Error != "rate_limited" {
		t.Errorf("expected error='rate_limited', got '%s'", body.Error)
	}
	if body.RetryAfter != 1000 {
		t.Errorf("expected retry_after_ms=1000, got %d", body.RetryAfter)
	}
	if body.Threshold != rateLimitThreshold {
		t.Errorf("expected threshold=%d, got %d", rateLimitThreshold, body.Threshold)
	}
	if body.CurrentRate < rateLimitThreshold {
		t.Errorf("expected current_rate >= %d, got %d", rateLimitThreshold, body.CurrentRate)
	}
}

// ============================================
// Test Scenario 8: Event count increments by batch size, not request count
// ============================================

func TestEventCountIncrementsByBatchSize(t *testing.T) {
	c := setupTestCapture(t)

	// Single request with 500 events
	c.RecordEvents(500)

	c.mu.RLock()
	count := c.windowEventCount
	c.mu.RUnlock()

	if count != 500 {
		t.Errorf("expected event count 500 after batch of 500, got %d", count)
	}

	// Another request with 400 events
	c.RecordEvents(400)

	c.mu.RLock()
	count = c.windowEventCount
	c.mu.RUnlock()

	if count != 900 {
		t.Errorf("expected event count 900 after second batch of 400, got %d", count)
	}

	// Should not be rate limited yet
	if c.CheckRateLimit() {
		t.Error("expected 900 events to be under threshold")
	}

	// One more batch of 200 pushes over threshold
	c.RecordEvents(200)

	if !c.CheckRateLimit() {
		t.Error("expected 1100 events to exceed threshold")
	}
}

// ============================================
// Test Scenario 9: Health endpoint returns circuit state accurately
// ============================================

func TestHealthEndpointCircuitState(t *testing.T) {
	c := setupTestCapture(t)

	// Test when circuit is closed
	health := c.GetHealthStatus()

	if health.CircuitOpen {
		t.Error("expected circuit_open=false initially")
	}
	if health.CurrentRate != 0 {
		t.Errorf("expected current_rate=0, got %d", health.CurrentRate)
	}

	// Open the circuit
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now()
	c.rateLimitStreak = 5
	c.windowEventCount = 2400
	c.circuitReason = "rate_exceeded"
	c.mu.Unlock()

	health = c.GetHealthStatus()

	if !health.CircuitOpen {
		t.Error("expected circuit_open=true after opening circuit")
	}
	if health.Reason != "rate_exceeded" {
		t.Errorf("expected reason='rate_exceeded', got '%s'", health.Reason)
	}
	if health.CurrentRate != 2400 {
		t.Errorf("expected current_rate=2400, got %d", health.CurrentRate)
	}
	if health.OpenedAt == "" {
		t.Error("expected opened_at to be set when circuit is open")
	}
}

func TestHealthEndpointHTTP(t *testing.T) {
	c := setupTestCapture(t)

	// Open circuit for test
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now()
	c.circuitReason = "rate_exceeded"
	c.windowEventCount = 1500
	c.mu.Unlock()

	req := httptest.NewRequest("GET", "/v4/health", nil)
	rec := httptest.NewRecorder()
	c.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.CircuitOpen {
		t.Error("expected circuit_open=true in health response")
	}
	if resp.Reason != "rate_exceeded" {
		t.Errorf("expected reason='rate_exceeded', got '%s'", resp.Reason)
	}
}

// ============================================
// Test Scenario 10: Circuit doesn't open on a single spike (< 5 consecutive seconds)
// ============================================

func TestCircuitDoesNotOpenOnSingleSpike(t *testing.T) {
	c := setupTestCapture(t)

	// Simulate only 3 consecutive seconds over threshold (not enough to open)
	c.mu.Lock()
	c.rateLimitStreak = 3
	c.mu.Unlock()

	c.mu.Lock()
	c.evaluateCircuit()
	isOpen := c.circuitOpen
	c.mu.Unlock()

	if isOpen {
		t.Error("expected circuit to stay closed with only 3 consecutive seconds over threshold")
	}

	// 4 is still not enough
	c.mu.Lock()
	c.rateLimitStreak = 4
	c.mu.Unlock()

	c.mu.Lock()
	c.evaluateCircuit()
	isOpen = c.circuitOpen
	c.mu.Unlock()

	if isOpen {
		t.Error("expected circuit to stay closed with only 4 consecutive seconds over threshold")
	}
}

// ============================================
// Test Scenario 11: Rate limit applies to all three ingest endpoints combined
// ============================================

func TestRateLimitGlobalAcrossEndpoints(t *testing.T) {
	c := setupTestCapture(t)

	// Send 400 events via WebSocket endpoint
	c.RecordEvents(400)

	// Send 400 events via network bodies endpoint
	c.RecordEvents(400)

	// Send 300 events via enhanced actions endpoint (total: 1100)
	c.RecordEvents(300)

	// Should now be rate limited globally
	if !c.CheckRateLimit() {
		t.Error("expected global rate limit after 1100 combined events across endpoints")
	}

	// Verify via HTTP that all three endpoints are affected
	// WebSocket events
	wsPayload := `{"events":[{"event":"message","id":"ws1","direction":"incoming","data":"x"}]}`
	req1 := httptest.NewRequest("POST", "/websocket-events", strings.NewReader(wsPayload))
	rec1 := httptest.NewRecorder()
	c.HandleWebSocketEvents(rec1, req1)
	if rec1.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for websocket-events, got %d", rec1.Code)
	}

	// Network bodies
	nbPayload := `{"bodies":[{"method":"GET","url":"http://example.com","status":200}]}`
	req2 := httptest.NewRequest("POST", "/network-bodies", strings.NewReader(nbPayload))
	rec2 := httptest.NewRecorder()
	c.HandleNetworkBodies(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for network-bodies, got %d", rec2.Code)
	}

	// Enhanced actions
	actPayload := `{"actions":[{"type":"click","timestamp":1234567890}]}`
	req3 := httptest.NewRequest("POST", "/enhanced-actions", strings.NewReader(actPayload))
	rec3 := httptest.NewRecorder()
	c.HandleEnhancedActions(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for enhanced-actions, got %d", rec3.Code)
	}
}

// ============================================
// Test Scenario 12: Non-ingest endpoints (GET queries, MCP tools) are never rate-limited
// ============================================

func TestNonIngestEndpointsNotRateLimited(t *testing.T) {
	c := setupTestCapture(t)

	// Exceed rate limit
	c.RecordEvents(2000)
	if !c.CheckRateLimit() {
		t.Fatal("expected rate limit to be active")
	}

	// Also open the circuit
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now()
	c.mu.Unlock()

	// GET on websocket-events should still work (it's a read/query)
	req1 := httptest.NewRequest("GET", "/websocket-events", nil)
	rec1 := httptest.NewRecorder()
	c.HandleWebSocketEvents(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("expected GET /websocket-events to return 200 even when rate limited, got %d", rec1.Code)
	}

	// Health endpoint should still work
	req2 := httptest.NewRequest("GET", "/v4/health", nil)
	rec2 := httptest.NewRecorder()
	c.HandleHealth(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("expected GET /v4/health to return 200 even when rate limited, got %d", rec2.Code)
	}
}

// ============================================
// Additional edge case tests
// ============================================

func TestCircuitOpensOnMemoryExceeded(t *testing.T) {
	c := setupTestCapture(t)

	// Set memory over the hard limit (50MB)
	c.mu.Lock()
	c.mem.simulatedMemory = 55 * 1024 * 1024 // 55MB
	c.mu.Unlock()

	c.mu.Lock()
	c.evaluateCircuit()
	isOpen := c.circuitOpen
	c.mu.Unlock()

	if !isOpen {
		t.Error("expected circuit to open when memory exceeds 50MB hard limit")
	}
}

func TestCircuitReasonMemoryExceeded(t *testing.T) {
	c := setupTestCapture(t)

	c.mu.Lock()
	c.mem.simulatedMemory = 55 * 1024 * 1024
	c.evaluateCircuit()
	reason := c.circuitReason
	c.mu.Unlock()

	if reason != "memory_exceeded" {
		t.Errorf("expected reason='memory_exceeded', got '%s'", reason)
	}
}

func TestCircuitReasonRateExceeded(t *testing.T) {
	c := setupTestCapture(t)

	c.mu.Lock()
	c.rateLimitStreak = 5
	c.evaluateCircuit()
	reason := c.circuitReason
	c.mu.Unlock()

	if reason != "rate_exceeded" {
		t.Errorf("expected reason='rate_exceeded', got '%s'", reason)
	}
}

func TestRateLimitStreakIncrementsCorrectly(t *testing.T) {
	c := setupTestCapture(t)

	// First window: exceed threshold
	c.RecordEvents(1200)

	// Tick the rate limiter (simulates 1 second passing)
	c.mu.Lock()
	c.tickRateWindow()
	streak := c.rateLimitStreak
	c.mu.Unlock()

	if streak != 1 {
		t.Errorf("expected streak=1 after first second over threshold, got %d", streak)
	}

	// Second window: exceed again
	c.RecordEvents(1100)
	c.mu.Lock()
	c.tickRateWindow()
	streak = c.rateLimitStreak
	c.mu.Unlock()

	if streak != 2 {
		t.Errorf("expected streak=2 after second second over threshold, got %d", streak)
	}
}

func TestRateLimitStreakResetsOnBelowThreshold(t *testing.T) {
	c := setupTestCapture(t)

	// Build up a streak
	c.mu.Lock()
	c.rateLimitStreak = 3
	c.windowEventCount = 500 // under threshold
	c.tickRateWindow()
	streak := c.rateLimitStreak
	c.mu.Unlock()

	if streak != 0 {
		t.Errorf("expected streak to reset to 0 when under threshold, got %d", streak)
	}
}

func TestRecordEventsWithBatchSize(t *testing.T) {
	c := setupTestCapture(t)

	// Record a batch of 50
	c.RecordEvents(50)

	c.mu.RLock()
	count := c.windowEventCount
	c.mu.RUnlock()

	if count != 50 {
		t.Errorf("expected windowEventCount=50, got %d", count)
	}

	// Record another batch of 100
	c.RecordEvents(100)

	c.mu.RLock()
	count = c.windowEventCount
	c.mu.RUnlock()

	if count != 150 {
		t.Errorf("expected windowEventCount=150, got %d", count)
	}
}

func TestRateLimitResponseWhenCircuitOpen(t *testing.T) {
	c := setupTestCapture(t)

	// Open the circuit
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now()
	c.windowEventCount = 2500
	c.mu.Unlock()

	payload := `{"events":[{"event":"message","id":"ws1","direction":"incoming","data":"test"}]}`
	req := httptest.NewRequest("POST", "/websocket-events", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	c.HandleWebSocketEvents(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	// Should have circuit_open=true in the response
	var body struct {
		CircuitOpen bool `json:"circuit_open"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !body.CircuitOpen {
		t.Error("expected circuit_open=true in 429 response when circuit is open")
	}
}

func TestLastBelowThresholdTracking(t *testing.T) {
	c := setupTestCapture(t)

	// Start with circuit open
	c.mu.Lock()
	c.circuitOpen = true
	c.circuitOpenedAt = time.Now().Add(-15 * time.Second)
	c.rateLimitStreak = 0
	c.windowEventCount = 500                                  // under threshold
	c.lastBelowThresholdAt = time.Now().Add(-5 * time.Second) // only 5 seconds below
	c.mem.simulatedMemory = 20 * 1024 * 1024                  // under 30MB
	c.mu.Unlock()

	// Circuit should stay open (need 10s below threshold)
	c.mu.Lock()
	c.evaluateCircuit()
	isOpen := c.circuitOpen
	c.mu.Unlock()

	if !isOpen {
		t.Error("expected circuit to stay open when below threshold for only 5 seconds")
	}
}
