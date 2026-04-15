// usage_counter.go — Aggregated tool usage counters for periodic beacon.

package telemetry

import (
	"sync"
	"time"
)

// UsageCounter is a thread-safe counter map that tracks tool:action call counts,
// latency totals, error counts, and async command outcomes.
type UsageCounter struct {
	mu          sync.Mutex
	counts      map[string]int
	latencySum  map[string]int64 // key → total milliseconds
	latencyMax  map[string]int64 // key → max milliseconds
	sessionCall        int  // total calls this session (for session depth)
	lastReportedDepth  int  // depth at last SwapAndReset (to avoid re-sending same value)
	everCalled         bool // false until first Increment (for first-use detection)
}

// NewUsageCounter creates a new empty usage counter.
func NewUsageCounter() *UsageCounter {
	return &UsageCounter{
		counts:     make(map[string]int),
		latencySum: make(map[string]int64),
		latencyMax: make(map[string]int64),
	}
}

// Increment adds 1 to the count for the given key (e.g., "observe:errors").
// Also refreshes the telemetry session to keep it alive during activity.
func (u *UsageCounter) Increment(key string) {
	u.mu.Lock()
	u.counts[key]++
	u.sessionCall++
	firstEver := !u.everCalled
	u.everCalled = true
	u.mu.Unlock()
	TouchSession()

	if firstEver {
		BeaconEvent("first_tool_call", map[string]string{"tool": key})
	}
}

// IncrementWithLatency adds 1 to the count and records latency for the key.
func (u *UsageCounter) IncrementWithLatency(key string, elapsed time.Duration) {
	ms := elapsed.Milliseconds()
	u.mu.Lock()
	u.counts[key]++
	u.sessionCall++
	u.latencySum[key] += ms
	if _, exists := u.latencyMax[key]; !exists || ms > u.latencyMax[key] {
		u.latencyMax[key] = ms
	}
	firstEver := !u.everCalled
	u.everCalled = true
	u.mu.Unlock()
	TouchSession()

	if firstEver {
		BeaconEvent("first_tool_call", map[string]string{"tool": key})
	}
}

// IncrementError records a tool error for analytics (separate from call count).
func (u *UsageCounter) IncrementError(key string) {
	u.mu.Lock()
	u.counts["err:"+key]++
	u.mu.Unlock()
}

// RecordAsyncOutcome tracks the terminal status of an async command.
// status is one of: complete, error, timeout, expired, cancelled.
func (u *UsageCounter) RecordAsyncOutcome(status string) {
	u.mu.Lock()
	u.counts["async:"+status]++
	u.mu.Unlock()
}

// SessionDepth returns the total tool calls in the current session.
func (u *UsageCounter) SessionDepth() int {
	u.mu.Lock()
	d := u.sessionCall
	u.mu.Unlock()
	return d
}

// Peek returns a copy of the current counts without resetting.
func (u *UsageCounter) Peek() map[string]int {
	u.mu.Lock()
	cp := make(map[string]int, len(u.counts))
	for k, v := range u.counts {
		cp[k] = v
	}
	u.mu.Unlock()
	return cp
}

// SwapAndReset atomically returns the aggregated snapshot and resets all counters.
// The returned map includes counts, latency (as lat_avg:key and lat_max:key), and session depth.
func (u *UsageCounter) SwapAndReset() map[string]int {
	u.mu.Lock()
	old := u.counts

	// Merge latency into the snapshot as lat_avg:key and lat_max:key (milliseconds).
	for key, total := range u.latencySum {
		count := old[key]
		if count > 0 {
			old["lat_avg:"+key] = int(total / int64(count))
		}
	}
	for key, max := range u.latencyMax {
		old["lat_max:"+key] = int(max)
	}

	// Include session depth only if it increased since last report.
	if u.sessionCall > u.lastReportedDepth {
		old["session_depth"] = u.sessionCall
		u.lastReportedDepth = u.sessionCall
	}

	u.counts = make(map[string]int)
	u.latencySum = make(map[string]int64)
	u.latencyMax = make(map[string]int64)
	// Note: sessionCall is NOT reset — it accumulates across the full session.
	// It resets naturally when the session rotates (30-min inactivity).
	u.mu.Unlock()
	return old
}
