// circuit_breaker.go — Rate limiting and circuit breaker state machine.
// Extracted from the Capture god object. Owns its own sync.RWMutex,
// independent of Capture.mu. Cross-cutting dependencies (memory, lifecycle)
// are injected as function callbacks.
package capture

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// CircuitBreaker implements a rate limiter with circuit breaker pattern.
// Uses a 1-second sliding window for event counting and a streak-based
// state machine for circuit open/close transitions.
type CircuitBreaker struct {
	mu                   sync.RWMutex
	windowEventCount     int
	rateWindowStart      time.Time
	rateLimitStreak      int
	lastBelowThresholdAt time.Time
	circuitOpen          bool
	circuitOpenedAt      time.Time
	circuitReason        string

	// Injected: returns current buffer memory in bytes (acquires Capture.mu.RLock)
	getMemory func() int64
	// Injected: emits lifecycle events (circuit_opened, circuit_closed)
	emitEvent func(event string, data map[string]any)
}

// NewCircuitBreaker creates a CircuitBreaker with injected dependencies.
func NewCircuitBreaker(getMemory func() int64, emitEvent func(string, map[string]any)) *CircuitBreaker {
	now := time.Now()
	return &CircuitBreaker{
		rateWindowStart:      now,
		lastBelowThresholdAt: now,
		getMemory:            getMemory,
		emitEvent:            emitEvent,
	}
}

// IsOpen returns whether the circuit breaker is currently open (rejecting all requests).
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.circuitOpen
}

// ForceOpen opens the circuit breaker for testing purposes.
func (cb *CircuitBreaker) ForceOpen(reason string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.circuitOpen = true
	cb.circuitOpenedAt = time.Now()
	cb.circuitReason = reason
}

// SetWindowState sets the rate window state for testing.
func (cb *CircuitBreaker) SetWindowState(start time.Time, count int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.rateWindowStart = start
	cb.windowEventCount = count
}

// RecordEvents records N events received in the current 1-second window.
// Called by ingest handlers with batch sizes.
func (cb *CircuitBreaker) RecordEvents(count int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	if now.Sub(cb.rateWindowStart) > time.Second {
		cb.tickRateWindow()
		cb.windowEventCount = 0
		cb.rateWindowStart = now
	}
	cb.windowEventCount += count
}

// CheckRateLimit returns true if the request should be rejected (429).
// Checks: 1) circuit open, 2) memory hard limit, 3) window rate.
func (cb *CircuitBreaker) CheckRateLimit() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.circuitOpen {
		return true
	}

	// Memory hard limit — immediate rejection
	if cb.getMemory() > MemoryHardLimit {
		return true
	}

	now := time.Now()
	if now.Sub(cb.rateWindowStart) > time.Second {
		return false // window expired, rate is effectively 0
	}
	return cb.windowEventCount > RateLimitThreshold
}

// tickRateWindow is called when a 1-second window expires.
// Updates streak counter and evaluates circuit state. Caller must hold lock.
func (cb *CircuitBreaker) tickRateWindow() {
	if cb.windowEventCount > RateLimitThreshold {
		cb.rateLimitStreak++
		cb.lastBelowThresholdAt = time.Time{}
	} else {
		cb.rateLimitStreak = 0
		if cb.lastBelowThresholdAt.IsZero() {
			cb.lastBelowThresholdAt = time.Now()
		}
	}
	cb.evaluateCircuit()
}

// evaluateCircuit implements the circuit breaker FSM.
// CLOSED→OPEN: streak>=5 OR memory>50MB. OPEN→CLOSED: streak=0 AND below for 10s AND memory<30MB.
// Caller must hold lock.
func (cb *CircuitBreaker) evaluateCircuit() {
	if !cb.circuitOpen {
		// Rate-based opening
		if cb.rateLimitStreak >= circuitOpenStreakCount {
			cb.circuitOpen = true
			cb.circuitOpenedAt = time.Now()
			cb.circuitReason = "rate_exceeded"
			go cb.emitEvent("circuit_opened", map[string]any{
				"reason":       "rate_exceeded",
				"streak":       cb.rateLimitStreak,
				"rate":         cb.windowEventCount,
				"threshold":    RateLimitThreshold,
				"memory_bytes": cb.getMemory(),
			})
			return
		}
		// Memory-based opening
		if cb.getMemory() > MemoryHardLimit {
			cb.circuitOpen = true
			cb.circuitOpenedAt = time.Now()
			cb.circuitReason = "memory_exceeded"
			go cb.emitEvent("circuit_opened", map[string]any{
				"reason":       "memory_exceeded",
				"memory_bytes": cb.getMemory(),
				"hard_limit":   MemoryHardLimit,
				"rate":         cb.windowEventCount,
			})
			return
		}
		return
	}

	// Check if circuit should close
	if cb.rateLimitStreak > 0 {
		return
	}
	if cb.lastBelowThresholdAt.IsZero() {
		return
	}
	if time.Since(cb.lastBelowThresholdAt) < time.Duration(circuitCloseSeconds)*time.Second {
		return
	}
	if cb.getMemory() > circuitCloseMemoryLimit {
		return
	}

	// All conditions met — close
	openDuration := time.Since(cb.circuitOpenedAt)
	prevReason := cb.circuitReason
	cb.circuitOpen = false
	cb.circuitReason = ""
	cb.rateLimitStreak = 0

	go cb.emitEvent("circuit_closed", map[string]any{
		"previous_reason":    prevReason,
		"open_duration_secs": openDuration.Seconds(),
		"memory_bytes":       cb.getMemory(),
		"rate":               cb.windowEventCount,
	})
}

// GetHealthStatus returns the current health/circuit state.
func (cb *CircuitBreaker) GetHealthStatus() HealthResponse {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	resp := HealthResponse{
		CircuitOpen: cb.circuitOpen,
		CurrentRate: cb.windowEventCount,
		MemoryBytes: cb.getMemory(),
		Reason:      cb.circuitReason,
	}
	if cb.circuitOpen {
		resp.OpenedAt = cb.circuitOpenedAt.Format(time.RFC3339)
	}
	return resp
}

// GetState returns circuit breaker state fields for external snapshot consumers.
// Used by Capture.GetHealthSnapshot() to avoid reentrant locking.
func (cb *CircuitBreaker) GetState() (open bool, reason string, openedAt time.Time, eventCount int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.circuitOpen, cb.circuitReason, cb.circuitOpenedAt, cb.windowEventCount
}

// WriteRateLimitResponse writes a 429 response with JSON body.
func (cb *CircuitBreaker) WriteRateLimitResponse(w http.ResponseWriter) {
	cb.mu.RLock()
	currentRate := cb.windowEventCount
	isOpen := cb.circuitOpen
	cb.mu.RUnlock()

	resp := RateLimitResponse{
		Error:        "rate_limited",
		Message:      "Server receiving >1000 events/sec. Retry after backoff.",
		RetryAfterMs: 1000,
		CircuitOpen:  isOpen,
		CurrentRate:  currentRate,
		Threshold:    RateLimitThreshold,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(resp)
}
