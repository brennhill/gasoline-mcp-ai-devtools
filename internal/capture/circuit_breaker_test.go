// circuit_breaker_test.go — Tests for the CircuitBreaker sub-struct.
// Verifies state machine transitions, streak counting, and concurrent RecordEvents safety.
package capture

import (
	"sync"
	"testing"
	"time"
)

func newTestCircuitBreaker() *CircuitBreaker {
	return NewCircuitBreaker(
		func(event string, data map[string]any) {}, // no-op lifecycle
	)
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	if cb.IsOpen() {
		t.Fatal("Circuit should be closed initially")
	}
	status := cb.GetHealthStatus()
	if status.CircuitOpen {
		t.Fatal("Health status should show circuit closed")
	}
	if status.CurrentRate != 0 {
		t.Fatalf("Expected rate 0, got %d", status.CurrentRate)
	}
}

func TestCircuitBreaker_RecordEvents(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	cb.RecordEvents(500)
	status := cb.GetHealthStatus()
	if status.CurrentRate != 500 {
		t.Fatalf("Expected rate 500, got %d", status.CurrentRate)
	}
}

func TestCircuitBreaker_CheckRateLimit_BelowThreshold(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	cb.RecordEvents(100)
	if cb.CheckRateLimit() {
		t.Fatal("Should not be rate limited at 100 events")
	}
}

func TestCircuitBreaker_CheckRateLimit_AboveThreshold(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	cb.RecordEvents(RateLimitThreshold + 1)
	if !cb.CheckRateLimit() {
		t.Fatal("Should be rate limited above threshold")
	}
}

func TestCircuitBreaker_CheckRateLimit_CircuitOpen(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	cb.ForceOpen("test")
	if !cb.CheckRateLimit() {
		t.Fatal("Should reject when circuit is open")
	}
}

func TestCircuitBreaker_StreakOpensCircuit(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()

	// Simulate 5 consecutive seconds over threshold
	for i := 0; i < circuitOpenStreakCount; i++ {
		cb.mu.Lock()
		cb.windowEventCount = RateLimitThreshold + 1
		cb.tickRateWindow()
		cb.windowEventCount = 0
		cb.mu.Unlock()
	}

	if !cb.IsOpen() {
		t.Fatal("Circuit should open after 5 consecutive seconds over threshold")
	}
}

func TestCircuitBreaker_CircuitCloses(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()

	cb.ForceOpen("rate_exceeded")
	if !cb.IsOpen() {
		t.Fatal("Circuit should be open")
	}

	// Set conditions for close: streak=0, below threshold for >10s
	cb.mu.Lock()
	cb.rateLimitStreak = 0
	cb.lastBelowThresholdAt = time.Now().Add(-11 * time.Second) // 11s ago
	cb.windowEventCount = 0
	cb.evaluateCircuit()
	cb.mu.Unlock()

	if cb.IsOpen() {
		t.Fatal("Circuit should close when rate below threshold for 10s")
	}
}

func TestCircuitBreaker_WindowExpiration(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	cb.RecordEvents(500)

	// After window expires, rate should not cause rejection
	cb.mu.Lock()
	cb.rateWindowStart = time.Now().Add(-2 * time.Second) // expired
	cb.mu.Unlock()

	if cb.CheckRateLimit() {
		t.Fatal("Should not rate limit after window expires")
	}
}

func TestCircuitBreaker_ConcurrentRecordEvents(t *testing.T) {
	t.Parallel()
	cb := newTestCircuitBreaker()
	var wg sync.WaitGroup

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				cb.RecordEvents(1)
				cb.CheckRateLimit()
			}
		}()
	}

	wg.Wait()
	// Should not panic — verifies concurrent safety
}

func TestCircuitBreaker_LifecycleCallback(t *testing.T) {
	t.Parallel()
	var events []string
	var mu sync.Mutex

	cb := NewCircuitBreaker(
		func(event string, data map[string]any) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	)

	// Force open circuit to trigger callback
	cb.mu.Lock()
	for i := 0; i < circuitOpenStreakCount; i++ {
		cb.windowEventCount = RateLimitThreshold + 1
		cb.tickRateWindow()
		cb.windowEventCount = 0
	}
	cb.mu.Unlock()

	// Wait briefly for goroutine-based callbacks
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(events) == 0 {
		t.Fatal("Expected lifecycle callback for circuit_opened")
	}
	if events[0] != "circuit_opened" {
		t.Fatalf("Expected circuit_opened event, got %s", events[0])
	}
}
