// usage_beacon.go — Periodic aggregated usage beacon.
//
// StartUsageBeaconLoop spawns a 5-minute ticker that drains the
// UsageTracker via SwapAndReset and fires a single `usage_summary` event
// summarising the window. The summary contains:
//
//   - tool_stats[]     — per-tool count, error_count, latency_avg_ms,
//                        latency_max_ms.
//   - async_outcomes   — counter map keyed by outcome name.
//   - session_depth    — total RecordToolCall invocations this session.
//
// No per-call beacons fire from this file — those live in
// usage_counter.go::fireStructuredBeacon. This file only emits the
// aggregated rollup.
//
// Wire contract: docs/core/app-metrics.md (`usage_summary` schema).

package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// usageBeaconInterval is the default interval for aggregated usage beacons.
const usageBeaconInterval = 5 * time.Minute

// onTickMu protects the onTick test hook from concurrent access.
var onTickMu sync.Mutex

// onTick is a test hook called after each tick iteration completes.
// When nil (production), no notification is sent.
var onTick func()

// setOnTick sets the test hook (use nil to clear).
func setOnTick(fn func()) {
	onTickMu.Lock()
	onTick = fn
	onTickMu.Unlock()
}

// callOnTick invokes the test hook if set.
func callOnTick() {
	onTickMu.Lock()
	fn := onTick
	onTickMu.Unlock()
	if fn != nil {
		fn()
	}
}

// StartUsageBeaconLoop starts a background goroutine that fires a usage_summary
// beacon every 5 minutes if there was activity. Respects ctx.Done() for clean shutdown.
func StartUsageBeaconLoop(ctx context.Context, tracker *UsageTracker) {
	util.SafeGo(func() {
		startUsageBeaconLoopWithInterval(ctx, tracker, usageBeaconInterval)
	})
}

// startUsageBeaconLoopWithInterval runs the beacon loop with a configurable interval.
// Blocks until ctx is cancelled. Used directly in tests with short intervals.
func startUsageBeaconLoopWithInterval(ctx context.Context, tracker *UsageTracker, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Session ending due to daemon shutdown.
			tracker.EmitSessionEnd("shutdown")
			return
		case <-ticker.C:
			snapshot := tracker.SwapAndReset()
			if snapshot == nil {
				callOnTick()
				continue // no activity, skip beacon
			}
			BeaconUsageSummary(int(interval.Minutes()), snapshot)
			callOnTick()
		}
	}
}
