// rate_limit.go — Request rate limiting and circuit breaker for the HTTP server.
// Protects against runaway extension polling or misbehaving clients.
// Design: Token bucket rate limiter per endpoint category. Circuit breaker
// opens after sustained errors, with exponential backoff before retry.
// Health endpoint bypasses rate limiting for monitoring.
package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// ============================================
// Rate Limiting & Circuit Breaker
// ============================================

// HealthResponse is returned by GET /health endpoint. Includes circuit breaker
// state and current rate metrics for operator monitoring and alerting.
//
// Fields:
//   - CircuitOpen: true if circuit breaker is open (rejecting all requests)
//   - OpenedAt: Timestamp when circuit opened (zero if currently closed)
//   - CurrentRate: Events recorded in last completed 1-second window
//   - MemoryBytes: Total memory across all buffers
//   - Reason: Why circuit opened ("rate_exceeded", "memory_exceeded", or empty)
//
// Operators monitor this endpoint to detect:
//   - Sustained high event rates (CurrentRate > threshold for 5+ seconds)
//   - Memory pressure (MemoryBytes > soft limit 20MB)
//   - Circuit oscillation (repeated open/close cycles)
type HealthResponse struct {
	CircuitOpen bool   `json:"circuit_open"`
	OpenedAt    string `json:"opened_at,omitempty"`
	CurrentRate int    `json:"current_rate"`
	MemoryBytes int64  `json:"memory_bytes"`
	Reason      string `json:"reason,omitempty"`
}

// RateLimitResponse is the 429 response body
type RateLimitResponse struct {
	Error        string `json:"error"`
	Message      string `json:"message"`
	RetryAfterMs int    `json:"retry_after_ms"`
	CircuitOpen  bool   `json:"circuit_open"`
	CurrentRate  int    `json:"current_rate"`
	Threshold    int    `json:"threshold"`
}

// RecordEvents records N events received in the current window.
// This is the primary entry point for rate limiting - called by ingest handlers
// with the batch size (not 1 per request). Hot path: called on every ingest.
//
// Sliding Window Mechanism (1-second window):
//   - On first call in new 1-second window: reset counter, tick previous window
//   - On subsequent calls in same window: accumulate count
//   - Caller must decide window boundaries; RecordEvents just accumulates
//
// State Updated:
//   - windowEventCount += count (accumulate in current window)
//   - rateWindowStart = now (on window expiration)
//   - rateLimitStreak, circuitOpen (updated in tickRateWindow on expiration)
//
// Caller must hold lock. This function is called AFTER enforceMemory() to
// ensure memory limits are checked before accepting new events.
func (c *Capture) RecordEvents(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	// If the window has expired, start a new one
	if now.Sub(c.rateWindowStart) > time.Second {
		// Before resetting, tick the window to update streak
		c.tickRateWindow()
		c.windowEventCount = 0
		c.rateWindowStart = now
	}
	c.windowEventCount += count
}

// CheckRateLimit returns true if the current request should be rejected (429).
// This checks THREE conditions in priority order (short-circuit on first match):
//
// 1. Circuit Breaker (highest priority):
//    - If circuitOpen == true: reject all requests
//    - Recovery: wait for circuit to close (10s below threshold + memory OK)
//
// 2. Memory Hard Limit:
//    - If total memory > 50MB: reject (emergency condition)
//    - Even if circuit closed, memory spike can trigger rejection
//    - Note: This differs from circuit open (may reject without opening circuit)
//
// 3. Rate Limit (windowing):
//    - If window has NOT expired (still in current 1-second window):
//      * If windowEventCount > threshold (1000): reject
//    - If window HAS expired (window age > 1 second):
//      * Assume rate is 0, accept (window will be ticked on next RecordEvents)
//
// Used by HTTP handlers to decide 429 rejection. Called BEFORE RecordEvents()
// so that expensive operations aren't started if rate-limited.
//
// Caller should hold read lock (uses RLock).
func (c *Capture) CheckRateLimit() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Circuit breaker takes precedence
	if c.circuitOpen {
		return true
	}

	// Memory hard limit - immediate rejection
	if c.isMemoryExceeded() {
		return true
	}

	// Check if we're in the current window and over threshold
	now := time.Now()
	if now.Sub(c.rateWindowStart) > time.Second {
		// Window expired, rate is effectively 0
		return false
	}
	return c.windowEventCount > rateLimitThreshold
}

// tickRateWindow is called when a window expires (every 1 second).
// Updates the streak counter and evaluates circuit breaker state.
// Caller must hold the lock.
//
// Streak Counter Logic (rateLimitStreak):
//   - If windowEventCount > threshold (1000 events/sec):
//     * rateLimitStreak++ (consecutive second over threshold)
//     * lastBelowThresholdAt = zero (reset, since we're over)
//   - If windowEventCount <= threshold:
//     * rateLimitStreak = 0 (reset streak)
//     * lastBelowThresholdAt = now (start measuring "below threshold" duration)
//
// Then calls evaluateCircuit() to check opening/closing conditions.
// This is the state machine heartbeat—runs once per second via window expiration.
func (c *Capture) tickRateWindow() {
	if c.windowEventCount > rateLimitThreshold {
		c.rateLimitStreak++
		// Reset the below-threshold timer since we're over
		c.lastBelowThresholdAt = time.Time{}
	} else {
		c.rateLimitStreak = 0
		// Track when we first went below threshold
		if c.lastBelowThresholdAt.IsZero() {
			c.lastBelowThresholdAt = time.Now()
		}
	}
	c.evaluateCircuit()
}

// evaluateCircuit implements the complete circuit breaker state machine.
// Called every 1 second when a rate window expires. Caller must hold lock.
//
// CIRCUIT BREAKER FSM:
// =====================
//
// State: circuitOpen (bool) + circuitReason (string)
// Transitions: CLOSED → OPEN → CLOSED
//
// CLOSED → OPEN Conditions (OR logic):
//   1. Rate-based: rateLimitStreak >= 5 consecutive seconds
//      - Means 5+ consecutive seconds with > 1000 events/sec
//      - Set circuitReason = "rate_exceeded"
//   2. Memory-based: total memory > 50MB (memoryHardLimit)
//      - Set circuitReason = "memory_exceeded"
//
// OPEN → CLOSED Conditions (AND logic, must BOTH be true):
//   1. Rate condition: rateLimitStreak == 0
//      - Means current window AND all previous windows below threshold
//   2. Time condition: now - lastBelowThresholdAt >= 10 seconds
//      - Must have been below threshold for 10+ consecutive seconds
//   3. Memory condition: getMemoryForCircuit() <= 30MB
//      - Must recover to low memory state (circuitCloseMemoryLimit)
//
// IMPORTANT: Circuit stays OPEN until ALL close conditions are true.
// If rate spikes again before 10 seconds, streak resets but duration counter
// continues from lastBelowThresholdAt (doesn't reset, prevents flapping).
//
// Example Timeline:
//   T=0s:  5s over threshold → circuitOpen=true, reason="rate_exceeded"
//   T=5s:  Rate drops below threshold
//          → rateLimitStreak=0, lastBelowThresholdAt=now
//          → circuitOpen stays true (need to wait 10s)
//   T=7s:  Brief rate spike, back to normal
//          → rateLimitStreak=0 (no change, stays 0)
//          → lastBelowThresholdAt unchanged (spans the spike)
//   T=15s: now - lastBelowThresholdAt = 10s, memory OK
//          → Close circuit, reset streak/lastBelowThresholdAt
//          → circuitOpen=false, circuitReason=""
//
// State Variables Modified:
//   - circuitOpen: true if any open condition met; false if all close conditions met
//   - circuitOpenedAt: Set when opening; cleared when closing (for display)
//   - circuitReason: "rate_exceeded" or "memory_exceeded" (cleared on close)
//   - rateLimitStreak: Reset to 0 on close
//   - lastBelowThresholdAt: Reset to zero Time on close
func (c *Capture) evaluateCircuit() {
	// Check if circuit should OPEN
	if !c.circuitOpen {
		// Rate-based opening: 5 consecutive seconds over threshold
		if c.rateLimitStreak >= circuitOpenStreakCount {
			c.circuitOpen = true
			c.circuitOpenedAt = time.Now()
			c.circuitReason = "rate_exceeded"
			return
		}
		// Memory-based opening: buffer memory exceeds hard limit
		if c.getMemoryForCircuit() > memoryHardLimit {
			c.circuitOpen = true
			c.circuitOpenedAt = time.Now()
			c.circuitReason = "memory_exceeded"
			return
		}
		return
	}

	// Check if circuit should CLOSE
	// Requirements: rate below threshold for 10s AND memory below 30MB
	if c.rateLimitStreak > 0 {
		// Still over threshold, can't close
		return
	}

	// Check that we've been below threshold long enough
	if c.lastBelowThresholdAt.IsZero() {
		// Haven't been below threshold yet
		return
	}
	belowDuration := time.Since(c.lastBelowThresholdAt)
	if belowDuration < time.Duration(circuitCloseSeconds)*time.Second {
		// Not long enough below threshold
		return
	}

	// Check memory
	if c.getMemoryForCircuit() > circuitCloseMemoryLimit {
		// Memory still too high
		return
	}

	// All conditions met - close the circuit
	c.circuitOpen = false
	c.circuitReason = ""
	c.rateLimitStreak = 0
}

// getMemoryForCircuit returns the memory to use for circuit evaluation.
// Uses simulated memory if set, otherwise real buffer memory. Caller must hold lock.
func (c *Capture) getMemoryForCircuit() int64 {
	if c.mem.simulatedMemory > 0 {
		return c.mem.simulatedMemory
	}
	return c.calcTotalMemory()
}

// GetHealthStatus returns the current health/circuit state
func (c *Capture) GetHealthStatus() HealthResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	resp := HealthResponse{
		CircuitOpen: c.circuitOpen,
		CurrentRate: c.windowEventCount,
		MemoryBytes: c.getMemoryForCircuit(),
		Reason:      c.circuitReason,
	}

	if c.circuitOpen {
		resp.OpenedAt = c.circuitOpenedAt.Format(time.RFC3339)
	}

	return resp
}

// WriteRateLimitResponse writes a 429 response with the proper JSON body and headers
func (c *Capture) WriteRateLimitResponse(w http.ResponseWriter) {
	c.mu.RLock()
	currentRate := c.windowEventCount
	isCircuitOpen := c.circuitOpen
	c.mu.RUnlock()

	resp := RateLimitResponse{
		Error:        "rate_limited",
		Message:      "Server receiving >1000 events/sec. Retry after backoff.",
		RetryAfterMs: 1000,
		CircuitOpen:  isCircuitOpen,
		CurrentRate:  currentRate,
		Threshold:    rateLimitThreshold,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleHealth returns circuit breaker state as a JSON response (used by /health)
func (c *Capture) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	health := c.GetHealthStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(health)
}
