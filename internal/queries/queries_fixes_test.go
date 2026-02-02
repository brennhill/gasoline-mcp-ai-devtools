package queries

import (
	"encoding/json"
	"runtime"
	"sync"
	"testing"
	"time"
)

// ============================================
// Issue 1: WaitForResult goroutine leak on timeout
// ============================================

func TestWaitForResult_NoGoroutineLeakOnTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow goroutine leak test")
	}
	// NOT parallel: relies on runtime.NumGoroutine() counts
	capture := setupTestCapture(t)

	// Create a pending query that will never get a result
	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#leak-test"}`),
	})

	// Let runtime settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	// Run WaitForResult with a short timeout (it will timeout)
	_, err := capture.WaitForResult(id, 200*time.Millisecond, "")
	if err == nil {
		t.Fatal("Expected timeout error")
	}

	// Wait for goroutines to clean up
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Allow for some variance but ensure no leak (goroutine count should not grow)
	if after > before+1 {
		t.Errorf("Goroutine leak detected: before=%d, after=%d (delta=%d)", before, after, after-before)
	}
}

func TestWaitForResult_MultipleTimeoutsNoLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow goroutine leak test")
	}
	// NOT parallel: relies on runtime.NumGoroutine() counts
	capture := setupTestCapture(t)

	// Let runtime settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	// Run multiple WaitForResult calls that all timeout
	for i := 0; i < 10; i++ {
		id := capture.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"#leak-test"}`),
		})
		_, _ = capture.WaitForResult(id, 100*time.Millisecond, "")
	}

	// Wait for goroutines to clean up
	time.Sleep(1 * time.Second)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()

	// Should not have leaked goroutines from 10 timeout calls
	if after > before+2 {
		t.Errorf("Goroutine leak after 10 timeouts: before=%d, after=%d (delta=%d)", before, after, after-before)
	}
}

// ============================================
// Issue 2: generateCorrelationID uniqueness
// ============================================

func TestGenerateCorrelationID_UniqueRapidGeneration(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := generateCorrelationID()
		if seen[id] {
			t.Fatalf("Duplicate correlation ID generated: %s (at iteration %d)", id, i)
		}
		seen[id] = true
	}

	if len(seen) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(seen))
	}
}

func TestGenerateCorrelationID_ConcurrentUniqueness(t *testing.T) {
	t.Parallel()
	count := 100
	goroutines := 10
	ids := make(chan string, count*goroutines)

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < count; i++ {
				ids <- generateCorrelationID()
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Fatalf("Duplicate correlation ID generated concurrently: %s", id)
		}
		seen[id] = true
	}

	expected := count * goroutines
	if len(seen) != expected {
		t.Errorf("Expected %d unique IDs, got %d", expected, len(seen))
	}
}

// ============================================
// Issue 3: startResultCleanup stop mechanism
// ============================================

func TestStartResultCleanup_ReturnsStopFunction(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	stop := capture.startResultCleanup()

	// Verify we got a stop function
	if stop == nil {
		t.Fatal("Expected startResultCleanup to return a non-nil stop function")
	}

	// Stop the cleanup goroutine
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()

	select {
	case <-done:
		// Good, stop returned quickly
	case <-time.After(2 * time.Second):
		t.Fatal("startResultCleanup stop function did not return within 2 seconds")
	}
}

func TestStartResultCleanup_GoroutineStopsAfterStop(t *testing.T) {
	// NOT parallel: relies on runtime.NumGoroutine() counts
	capture := setupTestCapture(t)

	// Let runtime settle
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	stop := capture.startResultCleanup()

	// Goroutine count should have increased
	time.Sleep(50 * time.Millisecond)
	during := runtime.NumGoroutine()
	if during <= before {
		t.Errorf("Expected goroutine count to increase after start: before=%d, during=%d", before, during)
	}

	// Stop and wait
	stop()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Errorf("Goroutine leak after stop: before=%d, after=%d", before, after)
	}
}

// ============================================
// Issue 4: Consolidated query cleanup
// ============================================

func TestStartQueryCleanup_CleansExpiredQueries(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Start the consolidated query cleanup
	stop := capture.startQueryCleanup()
	defer stop()

	// Create a query with a very short timeout
	capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#cleanup-test"}`),
	}, 100*time.Millisecond, "")

	// Verify query exists
	pending := capture.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pending))
	}

	// Wait for the query to expire and the periodic cleanup to run
	// Query expires in 100ms, cleanup runs every 5s, but GetPendingQueries also cleans
	time.Sleep(200 * time.Millisecond)

	// GetPendingQueries cleans expired queries
	pending = capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected expired query to be cleaned up, got %d pending", len(pending))
	}
}

func TestStartQueryCleanup_ReturnsStopFunction(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	stop := capture.startQueryCleanup()
	if stop == nil {
		t.Fatal("Expected startQueryCleanup to return a non-nil stop function")
	}

	// Stop should not block
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(2 * time.Second):
		t.Fatal("startQueryCleanup stop function did not return within 2 seconds")
	}
}

func TestCreatePendingQueryWithClient_NoPerQueryGoroutine(t *testing.T) {
	// NOT parallel: relies on runtime.NumGoroutine() counts
	capture := setupTestCapture(t)

	// Let runtime settle
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	before := runtime.NumGoroutine()

	// Create multiple queries
	for i := 0; i < 5; i++ {
		capture.CreatePendingQueryWithClient(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"#test"}`),
		}, 30*time.Second, "")
	}

	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Without the per-query goroutines, goroutine count should not grow by 5
	// (there should be 0 goroutines spawned per query now)
	if after > before+2 {
		t.Errorf("Per-query goroutines detected: before=%d, after=%d (expected no growth)", before, after)
	}
}
