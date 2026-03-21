// usage_counter_test.go — Tests for aggregated tool usage counters.

package telemetry

import (
	"sync"
	"testing"
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
