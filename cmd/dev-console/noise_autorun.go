// noise_autorun.go — Automatic noise detection after page navigation.
// Runs noise auto-detect in a debounced background goroutine, triggered when
// the extension reports a navigation action. Prevents stale errors from
// dominating observe() results after page changes.
package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dev-console/dev-console/internal/ai"
	"github.com/dev-console/dev-console/internal/util"
)

// noiseAutoDetectInterval is the minimum time between automatic noise detection runs.
// Navigation events closer together than this are coalesced into a single run.
const noiseAutoDetectInterval = 30 * time.Second

// noiseAutoDetectEnvVar gates navigation-triggered noise auto-detection.
// Default is off; set to 1/true/on/yes to enable.
const noiseAutoDetectEnvVar = "GASOLINE_NOISE_AUTORUN"

// noiseAutoRunner debounces automatic noise detection. Multiple rapid navigation
// events (e.g., SPA route changes) are coalesced: only one auto-detect runs per
// debounce window. Thread-safe.
type noiseAutoRunner struct {
	mu       sync.Mutex
	fn       func()
	interval time.Duration
	lastRun  time.Time
	pending  bool
}

// newNoiseAutoRunner creates a debounced runner for the given function.
// The interval is the minimum time between runs.
func newNoiseAutoRunner(fn func(), interval time.Duration) *noiseAutoRunner {
	return &noiseAutoRunner{
		fn:       fn,
		interval: interval,
	}
}

// schedule requests that the function run after the debounce interval elapses.
// If a run is already scheduled, this is a no-op (coalesced).
// Thread-safe: may be called from any goroutine.
func (r *noiseAutoRunner) schedule() {
	if r.fn == nil {
		return
	}

	r.mu.Lock()
	if r.pending {
		r.mu.Unlock()
		return
	}

	elapsed := time.Since(r.lastRun)
	if elapsed >= r.interval {
		// Enough time has passed — run immediately in background
		r.pending = true
		r.mu.Unlock()
		util.SafeGo(r.run)
		return
	}

	// Schedule for after the remaining debounce period
	r.pending = true
	delay := r.interval - elapsed
	r.mu.Unlock()

	util.SafeGo(func() {
		time.Sleep(delay)
		r.run()
	})
}

// run executes the function and resets the debounce state.
func (r *noiseAutoRunner) run() {
	r.fn()

	r.mu.Lock()
	r.lastRun = time.Now()
	r.pending = false
	r.mu.Unlock()
}

// wireNoiseAutoDetect connects automatic noise detection to navigation events.
// Called once during NewToolHandler initialization.
func wireNoiseAutoDetect(h *ToolHandler) {
	if !noiseAutoDetectEnabled() {
		return
	}
	if h.capture == nil || h.noiseConfig == nil {
		return
	}

	runner := newNoiseAutoRunner(func() {
		h.runNoiseAutoDetect()
	}, noiseAutoDetectInterval)

	h.capture.SetNavigationCallback(func() {
		runner.schedule()
	})

	fmt.Fprintf(os.Stderr, "[gasoline] noise auto-detect enabled (triggers after navigation, debounce=%s)\n", noiseAutoDetectInterval)
}

// runNoiseAutoDetectOnFirstConnection schedules exactly one noise auto-detect run
// for the process lifetime after the first MCP initialize call.
func (h *ToolHandler) runNoiseAutoDetectOnFirstConnection() {
	if h == nil || h.capture == nil || h.noiseConfig == nil {
		return
	}
	if !atomic.CompareAndSwapUint32(&h.noiseInitTriggered, 0, 1) {
		return
	}
	atomic.AddUint32(&h.noiseAutoInitRuns, 1)
	util.SafeGo(func() {
		h.runNoiseAutoDetect()
	})
}

func noiseAutoDetectEnabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(noiseAutoDetectEnvVar)))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

// IsConsoleNoise delegates to the noise config to check if a log entry is noise.
// Satisfies mcp.NoiseFilterer. Returns false if noise config is nil.
func (h *ToolHandler) IsConsoleNoise(entry map[string]any) bool {
	if h.noiseConfig == nil {
		return false
	}
	return h.noiseConfig.IsConsoleNoise(entry)
}

// runNoiseAutoDetect collects current buffer data and runs noise auto-detection.
// This is the same logic as noiseActionAutoDetect() but designed for background use.
func (h *ToolHandler) runNoiseAutoDetect() {
	h.server.mu.RLock()
	consoleEntries := make([]ai.LogEntry, len(h.server.entries))
	for i, e := range h.server.entries {
		consoleEntries[i] = ai.LogEntry(e)
	}
	h.server.mu.RUnlock()

	networkBodies := h.capture.GetNetworkBodies()
	wsEvents := h.capture.GetAllWebSocketEvents()

	proposals := h.noiseConfig.AutoDetect(consoleEntries, networkBodies, wsEvents)
	if len(proposals) > 0 {
		var toApply []ai.NoiseRule
		for _, p := range proposals {
			if p.Confidence >= 0.9 {
				toApply = append(toApply, p.Rule)
			}
		}
		if len(toApply) > 0 {
			_ = h.noiseConfig.AddRules(toApply)
		}
		fmt.Fprintf(os.Stderr, "[gasoline] noise auto-detect: %d proposals, %d auto-applied\n", len(proposals), len(toApply))
	}
}
