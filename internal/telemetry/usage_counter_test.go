// usage_counter_test.go — Tests for aggregated tool usage counters.

package telemetry

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestUsageTracker_Increment(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("observe:errors", 0, false)

	counts := c.Peek()
	if counts["observe:errors"] != 3 {
		t.Fatalf("count = %d, want 3", counts["observe:errors"])
	}
}

func TestUsageTracker_SwapAndReset(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("interact:click", 0, false)

	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if len(snapshot.ToolStats) != 2 {
		t.Fatalf("ToolStats length = %d, want 2", len(snapshot.ToolStats))
	}

	// After swap, should be empty.
	fresh := c.SwapAndReset()
	if fresh != nil {
		t.Fatalf("second SwapAndReset returned %+v, want nil", fresh)
	}
}

func TestUsageTracker_ConcurrentIncrement(t *testing.T) {
	c := NewUsageTracker()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			c.RecordToolCall("concurrent:key", 0, false)
		}()
	}
	wg.Wait()

	counts := c.Peek()
	if counts["concurrent:key"] != goroutines {
		t.Fatalf("count = %d, want %d", counts["concurrent:key"], goroutines)
	}
}

func TestUsageTracker_ConcurrentSwapAndIncrement(t *testing.T) {
	c := NewUsageTracker()
	const incrementors = 100
	const incrementsEach = 50

	var wg sync.WaitGroup
	var swapMu sync.Mutex

	// Start incrementor goroutines.
	wg.Add(incrementors)
	for i := 0; i < incrementors; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsEach; j++ {
				c.RecordToolCall("key", 0, false)
			}
		}()
	}

	// Start a swapper goroutine that runs concurrently with incrementors.
	stopSwapper := make(chan struct{})
	swapperDone := make(chan struct{})
	var snapshots []*UsageSnapshot
	go func() {
		defer close(swapperDone)
		for {
			select {
			case <-stopSwapper:
				return
			default:
				snapshot := c.SwapAndReset()
				if snapshot != nil {
					swapMu.Lock()
					snapshots = append(snapshots, snapshot)
					swapMu.Unlock()
				}
				runtime.Gosched() // yield to avoid burning CPU in tight loop
			}
		}
	}()

	// Wait for all incrementors to finish.
	wg.Wait()

	// Signal the swapper to stop.
	close(stopSwapper)
	<-swapperDone

	// Collect the final snapshot.
	if final := c.SwapAndReset(); final != nil {
		snapshots = append(snapshots, final)
	}

	// Sum all counts across all snapshots.
	total := 0
	for _, snap := range snapshots {
		for _, stat := range snap.ToolStats {
			if stat.Tool == "key" {
				total += stat.Count
			}
		}
	}

	expected := incrementors * incrementsEach
	if total != expected {
		t.Fatalf("total count = %d, want %d (counts were lost)", total, expected)
	}
}

func TestUsageTracker_Peek(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 0, false)
	c.RecordToolCall("observe:page", 0, false)
	c.RecordToolCall("interact:click", 0, false)

	peeked := c.Peek()
	if peeked["observe:page"] != 2 {
		t.Fatalf("peeked observe:page = %d, want 2", peeked["observe:page"])
	}
	if peeked["interact:click"] != 1 {
		t.Fatalf("peeked interact:click = %d, want 1", peeked["interact:click"])
	}

	// Peek should not reset — counts should still be there.
	peeked2 := c.Peek()
	if peeked2["observe:page"] != 2 {
		t.Fatalf("second peek observe:page = %d, want 2 (Peek should not reset)", peeked2["observe:page"])
	}

	// Mutating the returned map should not affect the counter.
	peeked["observe:page"] = 999
	peeked3 := c.Peek()
	if peeked3["observe:page"] != 2 {
		t.Fatalf("peek after mutation = %d, want 2 (returned map should be a copy)", peeked3["observe:page"])
	}
}

func TestUsageTracker_PeekEmpty(t *testing.T) {
	c := NewUsageTracker()
	peeked := c.Peek()
	if len(peeked) != 0 {
		t.Fatalf("Peek on new counter returned %d entries, want 0", len(peeked))
	}
}

func TestUsageTracker_SwapAndResetEmpty(t *testing.T) {
	c := NewUsageTracker()
	snapshot := c.SwapAndReset()
	if snapshot != nil {
		t.Fatalf("SwapAndReset on empty tracker returned %+v, want nil", snapshot)
	}
}

func TestUsageTracker_RecordToolCallWithLatency(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 50*time.Millisecond, false)
	c.RecordToolCall("observe:page", 150*time.Millisecond, false)
	c.RecordToolCall("interact:click", 30*time.Millisecond, false)

	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	for _, s := range snapshot.ToolStats {
		if s.Tool == "observe:page" {
			if s.Count != 2 {
				t.Fatalf("observe:page count = %d, want 2", s.Count)
			}
			if s.LatencyAvgMs != 100 {
				t.Fatalf("observe:page lat_avg = %d, want 100", s.LatencyAvgMs)
			}
			if s.LatencyMaxMs != 150 {
				t.Fatalf("observe:page lat_max = %d, want 150", s.LatencyMaxMs)
			}
		}
		if s.Tool == "interact:click" {
			if s.LatencyAvgMs != 30 {
				t.Fatalf("interact:click lat_avg = %d, want 30", s.LatencyAvgMs)
			}
		}
	}
}

func TestUsageTracker_RecordToolCallError(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 0, false)
	c.RecordToolCall("observe:page", 0, true) // error

	snapshot := c.Peek()
	if snapshot["observe:page"] != 2 {
		t.Fatalf("observe:page = %d, want 2", snapshot["observe:page"])
	}
	if snapshot["err:observe:page"] != 1 {
		t.Fatalf("err:observe:page = %d, want 1", snapshot["err:observe:page"])
	}
}

func TestUsageTracker_RecordAsyncOutcome(t *testing.T) {
	c := NewUsageTracker()
	c.RecordAsyncOutcome("complete")
	c.RecordAsyncOutcome("complete")
	c.RecordAsyncOutcome("timeout")
	c.RecordAsyncOutcome("expired")

	snapshot := c.Peek()
	if snapshot["async:complete"] != 2 {
		t.Fatalf("async:complete = %d, want 2", snapshot["async:complete"])
	}
	if snapshot["async:timeout"] != 1 {
		t.Fatalf("async:timeout = %d, want 1", snapshot["async:timeout"])
	}
	if snapshot["async:expired"] != 1 {
		t.Fatalf("async:expired = %d, want 1", snapshot["async:expired"])
	}
}

func TestUsageTracker_SessionDepth(t *testing.T) {
	c := NewUsageTracker()
	if c.SessionDepth() != 0 {
		t.Fatalf("initial session depth = %d, want 0", c.SessionDepth())
	}

	c.RecordToolCall("a", 0, false)
	c.RecordToolCall("b", 0, false)
	c.RecordToolCall("c", time.Millisecond, false)

	if c.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", c.SessionDepth())
	}

	// SwapAndReset should include session_depth but NOT reset it.
	snapshot := c.SwapAndReset()
	if snapshot == nil {
		t.Fatal("snapshot is nil")
	}
	if snapshot.SessionDepth != 3 {
		t.Fatalf("session_depth in snapshot = %d, want 3", snapshot.SessionDepth)
	}
	if c.SessionDepth() != 3 {
		t.Fatalf("session depth after swap = %d, want 3 (should not reset)", c.SessionDepth())
	}

	// Further calls add to the running total.
	c.RecordToolCall("d", 0, false)
	if c.SessionDepth() != 4 {
		t.Fatalf("session depth after +1 = %d, want 4", c.SessionDepth())
	}
}

func TestUsageTracker_LatencyNotIncludedWhenNoLatencyRecorded(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:page", 0, false) // no latency variant

	snapshot := c.Peek()
	if _, exists := snapshot["lat_avg:observe:page"]; exists {
		t.Fatal("lat_avg should not exist when no latency was recorded")
	}
}

func TestUsageTracker_MultipleKeys(t *testing.T) {
	c := NewUsageTracker()
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("observe:errors", 0, false)
	c.RecordToolCall("interact:click", 0, false)
	c.RecordToolCall("analyze:performance", 0, false)
	c.RecordToolCall("analyze:performance", 0, false)
	c.RecordToolCall("analyze:performance", 0, false)

	counts := c.Peek()
	if counts["observe:errors"] != 2 {
		t.Fatalf("observe:errors = %d, want 2", counts["observe:errors"])
	}
	if counts["interact:click"] != 1 {
		t.Fatalf("interact:click = %d, want 1", counts["interact:click"])
	}
	if counts["analyze:performance"] != 3 {
		t.Fatalf("analyze:performance = %d, want 3", counts["analyze:performance"])
	}
}
