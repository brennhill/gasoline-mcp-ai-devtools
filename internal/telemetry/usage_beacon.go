// usage_beacon.go — Periodic aggregated usage beacon.

package telemetry

import (
	"context"
	"strconv"
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
// beacon every 10 minutes if there was activity. Respects ctx.Done() for clean shutdown.
func StartUsageBeaconLoop(ctx context.Context, counter *UsageCounter) {
	util.SafeGo(func() {
		startUsageBeaconLoopWithInterval(ctx, counter, usageBeaconInterval)
	})
}

// startUsageBeaconLoopWithInterval runs the beacon loop with a configurable interval.
// Blocks until ctx is cancelled. Used directly in tests with short intervals.
func startUsageBeaconLoopWithInterval(ctx context.Context, counter *UsageCounter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := counter.SwapAndReset()
			if len(snapshot) == 0 {
				callOnTick()
				continue // no activity, skip beacon
			}
			props := make(map[string]string)
			props["window_m"] = strconv.Itoa(int(interval.Minutes()))
			for key, count := range snapshot {
				props[key] = strconv.Itoa(count)
			}
			BeaconEvent("usage_summary", props)
			callOnTick()
		}
	}
}
