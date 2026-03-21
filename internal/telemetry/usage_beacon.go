// usage_beacon.go — Periodic aggregated usage beacon.

package telemetry

import (
	"context"
	"strconv"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// usageBeaconInterval is the default interval for aggregated usage beacons.
const usageBeaconInterval = 10 * time.Minute

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
				continue // no activity, skip beacon
			}
			props := make(map[string]string)
			props["window_m"] = "10"
			for key, count := range snapshot {
				props[key] = strconv.Itoa(count)
			}
			BeaconEvent("usage_summary", props)
		}
	}
}
