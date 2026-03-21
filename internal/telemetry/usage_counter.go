// usage_counter.go — Aggregated tool usage counters for periodic beacon.

package telemetry

import "sync"

// UsageCounter is a thread-safe counter map that tracks tool:action call counts.
type UsageCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

// NewUsageCounter creates a new empty usage counter.
func NewUsageCounter() *UsageCounter {
	return &UsageCounter{
		counts: make(map[string]int),
	}
}

// Increment adds 1 to the count for the given key (e.g., "observe:errors").
func (u *UsageCounter) Increment(key string) {
	u.mu.Lock()
	u.counts[key]++
	u.mu.Unlock()
}

// SwapAndReset atomically returns the current counts and replaces with an empty map.
func (u *UsageCounter) SwapAndReset() map[string]int {
	u.mu.Lock()
	old := u.counts
	u.counts = make(map[string]int)
	u.mu.Unlock()
	return old
}
