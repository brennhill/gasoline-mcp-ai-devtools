// ring_buffer_slo_test.go — Performance SLO tests for ring buffer operations.

package buffers

import (
	"testing"
	"time"
)

// TestSLOWriteOne validates that WriteOne completes in < 500ns average.
// This SLO supports the WebSocket < 0.1ms requirement from CLAUDE.md.
func TestSLOWriteOne(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("SLO test skipped under race detector (significantly slower execution)")
	}

	const iterations = 10000
	const maxAvgDuration = 500 * time.Nanosecond

	rb := NewRingBuffer[int](1000)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		rb.WriteOne(i)
	}
	elapsed := time.Since(start)

	avgDuration := elapsed / iterations
	if avgDuration > maxAvgDuration {
		t.Errorf("WriteOne SLO violation: avg %v > %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	} else {
		t.Logf("WriteOne SLO met: avg %v < %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	}
}

// TestSLOReadAll validates that ReadAll on a 1000-entry buffer completes in < 100μs average.
// This ensures low-latency reads for observe tool responses.
func TestSLOReadAll(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("SLO test skipped under race detector (significantly slower execution)")
	}

	const iterations = 1000
	const bufferSize = 1000
	const maxAvgDuration = 100 * time.Microsecond

	rb := NewRingBuffer[int](bufferSize)

	// Fill buffer with 1000 entries
	for i := 0; i < bufferSize; i++ {
		rb.WriteOne(i)
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		items := rb.ReadAll()
		if len(items) != bufferSize {
			t.Fatalf("ReadAll returned %d items, expected %d", len(items), bufferSize)
		}
	}
	elapsed := time.Since(start)

	avgDuration := elapsed / iterations
	if avgDuration > maxAvgDuration {
		t.Errorf("ReadAll SLO violation: avg %v > %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	} else {
		t.Logf("ReadAll SLO met: avg %v < %v (total %v for %d iterations)",
			avgDuration, maxAvgDuration, elapsed, iterations)
	}
}
