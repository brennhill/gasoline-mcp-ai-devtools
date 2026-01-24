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
func (v *Capture) RecordEvents(count int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	// If the window has expired, start a new one
	if now.Sub(v.rateWindowStart) > time.Second {
		// Before resetting, tick the window to update streak
		v.tickRateWindow()
		v.windowEventCount = 0
		v.rateWindowStart = now
	}
	v.windowEventCount += count
}

// CheckRateLimit returns true if the current request should be rejected (429).
// This checks the circuit breaker state, memory limits, and per-window rate.
func (v *Capture) CheckRateLimit() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Circuit breaker takes precedence
	if v.circuitOpen {
		return true
	}

	// Memory hard limit - immediate rejection
	if v.isMemoryExceeded() {
		return true
	}

	// Check if we're in the current window and over threshold
	now := time.Now()
	if now.Sub(v.rateWindowStart) > time.Second {
		// Window expired, rate is effectively 0
		return false
	}
	return v.windowEventCount > rateLimitThreshold
}

// tickRateWindow is called when a window expires. It updates the streak counter
// and evaluates the circuit breaker. Caller must hold the lock.
func (v *Capture) tickRateWindow() {
	if v.windowEventCount > rateLimitThreshold {
		v.rateLimitStreak++
		// Reset the below-threshold timer since we're over
		v.lastBelowThresholdAt = time.Time{}
	} else {
		v.rateLimitStreak = 0
		// Track when we first went below threshold
		if v.lastBelowThresholdAt.IsZero() {
			v.lastBelowThresholdAt = time.Now()
		}
	}
	v.evaluateCircuit()
}

// evaluateCircuit checks whether the circuit should open or close.
// Caller must hold the lock.
func (v *Capture) evaluateCircuit() {
	// Check if circuit should OPEN
	if !v.circuitOpen {
		// Rate-based opening: 5 consecutive seconds over threshold
		if v.rateLimitStreak >= circuitOpenStreakCount {
			v.circuitOpen = true
			v.circuitOpenedAt = time.Now()
			v.circuitReason = "rate_exceeded"
			return
		}
		// Memory-based opening: buffer memory exceeds hard limit
		if v.getMemoryForCircuit() > memoryHardLimit {
			v.circuitOpen = true
			v.circuitOpenedAt = time.Now()
			v.circuitReason = "memory_exceeded"
			return
		}
		return
	}

	// Check if circuit should CLOSE
	// Requirements: rate below threshold for 10s AND memory below 30MB
	if v.rateLimitStreak > 0 {
		// Still over threshold, can't close
		return
	}

	// Check that we've been below threshold long enough
	if v.lastBelowThresholdAt.IsZero() {
		// Haven't been below threshold yet
		return
	}
	belowDuration := time.Since(v.lastBelowThresholdAt)
	if belowDuration < time.Duration(circuitCloseSeconds)*time.Second {
		// Not long enough below threshold
		return
	}

	// Check memory
	if v.getMemoryForCircuit() > circuitCloseMemoryLimit {
		// Memory still too high
		return
	}

	// All conditions met - close the circuit
	v.circuitOpen = false
	v.circuitReason = ""
	v.rateLimitStreak = 0
}

// getMemoryForCircuit returns the memory to use for circuit evaluation.
// Uses simulated memory if set, otherwise real buffer memory. Caller must hold lock.
func (v *Capture) getMemoryForCircuit() int64 {
	if v.mem.simulatedMemory > 0 {
		return v.mem.simulatedMemory
	}
	return v.calcTotalMemory()
}

// GetHealthStatus returns the current health/circuit state
func (v *Capture) GetHealthStatus() HealthResponse {
	v.mu.RLock()
	defer v.mu.RUnlock()

	resp := HealthResponse{
		CircuitOpen: v.circuitOpen,
		CurrentRate: v.windowEventCount,
		MemoryBytes: v.getMemoryForCircuit(),
		Reason:      v.circuitReason,
	}

	if v.circuitOpen {
		resp.OpenedAt = v.circuitOpenedAt.Format(time.RFC3339)
	}

	return resp
}

// WriteRateLimitResponse writes a 429 response with the proper JSON body and headers
func (v *Capture) WriteRateLimitResponse(w http.ResponseWriter) {
	v.mu.RLock()
	currentRate := v.windowEventCount
	isCircuitOpen := v.circuitOpen
	v.mu.RUnlock()

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

// HandleHealth handles GET /v4/health returning circuit breaker state
func (v *Capture) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	health := v.GetHealthStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(health)
}
