// beacon.go — Anonymous telemetry beacons. Disable with Kaboom_TELEMETRY=off.

package telemetry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Version is set via ldflags at build time. Falls back to "dev" if unset.
var Version = "dev"

// beaconMu protects endpoint and llmName from concurrent access.
var beaconMu sync.RWMutex

// llmName is the MCP client name (e.g. "claude-code", "cursor").
var llmName string

// SetLLMName records which LLM client connected. Included in all subsequent beacons.
func SetLLMName(name string) {
	beaconMu.Lock()
	llmName = name
	beaconMu.Unlock()
}

// defaultEndpoint is the canonical telemetry ingest URL.
const defaultEndpoint = "https://t.gokaboom.dev/v1/event"

// endpoint is the telemetry ingest URL. Overridable for tests.
var endpoint = defaultEndpoint

// maxConcurrentBeacons caps in-flight beacon goroutines. Chosen to allow burst
// traffic (startup + first tool calls) without unbounded goroutine growth.
// A dropped beacon is harmless — telemetry is best-effort.
const maxConcurrentBeacons = 50

// sem caps the number of concurrent beacon goroutines to prevent runaway growth.
var sem = make(chan struct{}, maxConcurrentBeacons)

// BeaconError fires an anonymous error event to the telemetry endpoint.
// Fire-and-forget: backgrounded, 2s timeout, never blocks caller, never panics.
func BeaconError(event string, props map[string]string) {
	beacon(event, props)
}

// BeaconEvent fires an anonymous lifecycle event.
// Fire-and-forget: backgrounded, 2s timeout, never blocks caller, never panics.
func BeaconEvent(event string, props map[string]string) {
	beacon(event, props)
}

func beacon(event string, props map[string]string) {
	if os.Getenv("Kaboom_TELEMETRY") == "off" {
		return
	}

	// Snapshot mutable state under lock before spawning goroutine.
	beaconMu.RLock()
	ep := endpoint
	llm := llmName
	beaconMu.RUnlock()

	payload := map[string]any{
		"event": event,
		"v":     Version,
		"os":    runtime.GOOS + "-" + runtime.GOARCH,
		"iid":   GetInstallID(),
	}
	if llm != "" {
		payload["llm"] = llm
	}
	if props != nil {
		payload["props"] = props
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return // best-effort
	}

	select {
	case sem <- struct{}{}:
		util.SafeGo(func() {
			defer func() { <-sem }()

			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Post(ep, "application/json", bytes.NewReader(data))
			if err != nil {
				return // best-effort
			}
			_ = resp.Body.Close()
		})
	default:
		// At capacity, drop this beacon silently
	}
}

// overrideEndpoint sets a custom endpoint for testing.
func overrideEndpoint(url string) {
	beaconMu.Lock()
	endpoint = url
	beaconMu.Unlock()
}

// resetEndpoint restores the default endpoint after testing.
func resetEndpoint() {
	beaconMu.Lock()
	endpoint = defaultEndpoint
	beaconMu.Unlock()
}
