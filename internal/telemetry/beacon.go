// beacon.go — Anonymous telemetry beacons. Disable with STRUM_TELEMETRY=off.

package telemetry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// Version is set via ldflags at build time. Falls back to "dev" if unset.
var Version = "dev"

// defaultEndpoint is the canonical telemetry ingest URL.
const defaultEndpoint = "https://t.getstrum.dev/v1/event"

// endpoint is the telemetry ingest URL. Overridable for tests.
var endpoint = defaultEndpoint

// sem caps the number of concurrent beacon goroutines to prevent runaway growth.
var sem = make(chan struct{}, 50)

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
	if os.Getenv("STRUM_TELEMETRY") == "off" {
		return
	}

	payload := map[string]any{
		"event": event,
		"v":     Version,
		"os":    runtime.GOOS + "-" + runtime.GOARCH,
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
			resp, err := client.Post(endpoint, "application/json", bytes.NewReader(data))
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
	endpoint = url
}

// resetEndpoint restores the default endpoint after testing.
func resetEndpoint() {
	endpoint = defaultEndpoint
}
