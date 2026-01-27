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

// HealthResponse is returned by the health endpoint with circuit state
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
// with the batch size (not 1 per request).
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
// This checks the circuit breaker state, memory limits, and per-window rate.
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

// tickRateWindow is called when a window expires. It updates the streak counter
// and evaluates the circuit breaker. Caller must hold the lock.
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

// evaluateCircuit checks whether the circuit should open or close.
// Caller must hold the lock.
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
