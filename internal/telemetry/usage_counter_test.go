// usage_counter_test.go — Tests for aggregated tool usage counters.

package telemetry

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestUsageCounter_Increment(t *testing.T) {
	c := NewUsageCounter()
	c.Increment("observe:errors")
	c.Increment("observe:errors")
	c.Increment("observe:errors")

	counts := c.SwapAndReset()
	if counts["observe:errors"] != 3 {
		t.Fatalf("count = %d, want 3", counts["observe:errors"])
	}
}

func TestUsageCounter_SwapAndReset(t *testing.T) {
	c := NewUsageCounter()
	c.Increment("observe:errors")
	c.Increment("interact:click")

	old := c.SwapAndReset()
	if old["observe:errors"] != 1 {
		t.Fatalf("old[observe:errors] = %d, want 1", old["observe:errors"])
	}
	if old["interact:click"] != 1 {
		t.Fatalf("old[interact:click] = %d, want 1", old["interact:click"])
	}

	// After swap, new map should be empty.
	fresh := c.SwapAndReset()
	if len(fresh) != 0 {
		t.Fatalf("fresh map has %d entries, want 0", len(fresh))
	}
}

func TestUsageCounter_ConcurrentIncrement(t *testing.T) {
	c := NewUsageCounter()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			c.Increment("concurrent:key")
		}()
	}
	wg.Wait()

	counts := c.SwapAndReset()
	if counts["concurrent:key"] != goroutines {
		t.Fatalf("count = %d, want %d", counts["concurrent:key"], goroutines)
	}
}

func TestUsageCounter_ConcurrentSwapAndIncrement(t *testing.T) {
	c := NewUsageCounter()
	const incrementors = 100
	const incrementsEach = 50

	var wg sync.WaitGroup
	var swapResults []map[string]int
	var swapMu sync.Mutex

	// Start incrementor goroutines.
	wg.Add(incrementors)
	for i := 0; i < incrementors; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < incrementsEach; j++ {
				c.Increment("key")
			}
		}()
	}

	// Start a swapper goroutine that runs concurrently with incrementors.
	stopSwapper := make(chan struct{})
	swapperDone := make(chan struct{})
	go func() {
		defer close(swapperDone)
		for {
			select {
			case <-stopSwapper:
				return
			default:
				snapshot := c.SwapAndReset()
				if len(snapshot) > 0 {
					swapMu.Lock()
					swapResults = append(swapResults, snapshot)
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
	finalSnapshot := c.SwapAndReset()
	if len(finalSnapshot) > 0 {
		swapResults = append(swapResults, finalSnapshot)
	}

	// Sum all counts across all swap results.
	total := 0
	for _, snapshot := range swapResults {
		total += snapshot["key"]
	}

	expected := incrementors * incrementsEach
	if total != expected {
		t.Fatalf("total count = %d, want %d (counts were lost)", total, expected)
	}
}

func TestUsageCounter_Peek(t *testing.T) {
	c := NewUsageCounter()
	c.Increment("observe:page")
	c.Increment("observe:page")
	c.Increment("interact:click")

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

func TestUsageCounter_PeekEmpty(t *testing.T) {
	c := NewUsageCounter()
	peeked := c.Peek()
	if len(peeked) != 0 {
		t.Fatalf("Peek on new counter returned %d entries, want 0", len(peeked))
	}
}

func TestUsageCounter_SwapAndResetEmpty(t *testing.T) {
	c := NewUsageCounter()
	snapshot := c.SwapAndReset()
	if len(snapshot) != 0 {
		t.Fatalf("SwapAndReset on new counter returned %d entries, want 0", len(snapshot))
	}
}

func TestUsageCounter_IncrementWithLatency(t *testing.T) {
	c := NewUsageCounter()
	c.IncrementWithLatency("observe:page", 50*time.Millisecond)
	c.IncrementWithLatency("observe:page", 150*time.Millisecond)
	c.IncrementWithLatency("interact:click", 30*time.Millisecond)

	snapshot := c.SwapAndReset()
	if snapshot["observe:page"] != 2 {
		t.Fatalf("observe:page count = %d, want 2", snapshot["observe:page"])
	}
	if snapshot["lat_avg:observe:page"] != 100 {
		t.Fatalf("lat_avg:observe:page = %d, want 100", snapshot["lat_avg:observe:page"])
	}
	if snapshot["lat_max:observe:page"] != 150 {
		t.Fatalf("lat_max:observe:page = %d, want 150", snapshot["lat_max:observe:page"])
	}
	if snapshot["lat_avg:interact:click"] != 30 {
		t.Fatalf("lat_avg:interact:click = %d, want 30", snapshot["lat_avg:interact:click"])
	}
}

func TestUsageCounter_IncrementError(t *testing.T) {
	c := NewUsageCounter()
	c.Increment("observe:page")
	c.Increment("observe:page")
	c.IncrementError("observe:page")

	snapshot := c.SwapAndReset()
	if snapshot["observe:page"] != 2 {
		t.Fatalf("observe:page = %d, want 2", snapshot["observe:page"])
	}
	if snapshot["err:observe:page"] != 1 {
		t.Fatalf("err:observe:page = %d, want 1", snapshot["err:observe:page"])
	}
}

func TestUsageCounter_RecordAsyncOutcome(t *testing.T) {
	c := NewUsageCounter()
	c.RecordAsyncOutcome("complete")
	c.RecordAsyncOutcome("complete")
	c.RecordAsyncOutcome("timeout")
	c.RecordAsyncOutcome("expired")

	snapshot := c.SwapAndReset()
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

func TestUsageCounter_SessionDepth(t *testing.T) {
	c := NewUsageCounter()
	if c.SessionDepth() != 0 {
		t.Fatalf("initial session depth = %d, want 0", c.SessionDepth())
	}

	c.Increment("a")
	c.Increment("b")
	c.IncrementWithLatency("c", time.Millisecond)

	if c.SessionDepth() != 3 {
		t.Fatalf("session depth = %d, want 3", c.SessionDepth())
	}

	// SwapAndReset should include session_depth but NOT reset it.
	snapshot := c.SwapAndReset()
	if snapshot["session_depth"] != 3 {
		t.Fatalf("session_depth in snapshot = %d, want 3", snapshot["session_depth"])
	}
	if c.SessionDepth() != 3 {
		t.Fatalf("session depth after swap = %d, want 3 (should not reset)", c.SessionDepth())
	}

	// Further calls add to the running total.
	c.Increment("d")
	if c.SessionDepth() != 4 {
		t.Fatalf("session depth after +1 = %d, want 4", c.SessionDepth())
	}
}

func TestUsageCounter_LatencyNotIncludedWhenNoLatencyRecorded(t *testing.T) {
	c := NewUsageCounter()
	c.Increment("observe:page") // no latency variant

	snapshot := c.SwapAndReset()
	if _, exists := snapshot["lat_avg:observe:page"]; exists {
		t.Fatal("lat_avg should not exist when no latency was recorded")
	}
}

func TestUsageCounter_MultipleKeys(t *testing.T) {
	c := NewUsageCounter()
	c.Increment("observe:errors")
	c.Increment("observe:errors")
	c.Increment("interact:click")
	c.Increment("analyze:performance")
	c.Increment("analyze:performance")
	c.Increment("analyze:performance")

	counts := c.SwapAndReset()
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
