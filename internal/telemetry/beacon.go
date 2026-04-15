// beacon.go — Anonymous telemetry beacons. Disable with KABOOM_TELEMETRY=off.

package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
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

// beaconClient is a shared HTTP client for all beacons. Reuses connections.
var beaconClient = &http.Client{Timeout: 2 * time.Second}

// buildEnvelope returns the base fields included in every beacon.
func buildEnvelope(event string) map[string]any {
	beaconMu.RLock()
	llm := llmName
	beaconMu.RUnlock()

	env := map[string]any{
		"event": event,
		"v":     Version,
		"os":    runtime.GOOS + "-" + runtime.GOARCH,
		"iid":   GetInstallID(),
		"sid":   GetSessionID(),
	}
	if llm != "" {
		env["llm"] = llm
	}
	return env
}

// BeaconError fires an anonymous error event to the telemetry endpoint.
// Fire-and-forget: backgrounded, 2s timeout, never blocks caller, never panics.
func BeaconError(event string, props map[string]string) {
	sendBeacon(event, props)
}

// BeaconEvent fires an anonymous lifecycle event.
// Fire-and-forget: backgrounded, 2s timeout, never blocks caller, never panics.
func BeaconEvent(event string, props map[string]string) {
	sendBeacon(event, props)
}

// BeaconUsageSummary fires a structured usage_summary beacon.
func BeaconUsageSummary(windowMinutes int, snapshot *UsageSnapshot) {
	if snapshot == nil {
		return
	}
	payload := buildEnvelope("usage_summary")
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	payload["channel"] = Channel
	payload["window_m"] = windowMinutes
	payload["tool_stats"] = snapshot.ToolStats
	payload["async_outcomes"] = snapshot.AsyncOutcomes
	if snapshot.SessionDepth > 0 {
		payload["session_depth"] = snapshot.SessionDepth
	}
	fireBeacon(payload)
}

// BuildUsageSummaryPayload builds the beacon payload without sending it.
// Used by debug endpoints to inspect what would be sent.
func BuildUsageSummaryPayload(windowMinutes int, snapshot *UsageSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	payload := buildEnvelope("usage_summary")
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	payload["channel"] = Channel
	payload["window_m"] = windowMinutes
	payload["tool_stats"] = snapshot.ToolStats
	payload["async_outcomes"] = snapshot.AsyncOutcomes
	if snapshot.SessionDepth > 0 {
		payload["session_depth"] = snapshot.SessionDepth
	}
	return payload
}

func sendBeacon(event string, props map[string]string) {
	payload := buildEnvelope(event)
	if props != nil {
		payload["props"] = props
	}

	fireBeacon(payload)
}

// telemetryOptedOut returns true if the user has disabled telemetry.
// Accepts KABOOM_TELEMETRY=off (case-insensitive).
func telemetryOptedOut() bool {
	return strings.EqualFold(os.Getenv("KABOOM_TELEMETRY"), "off")
}

// onFireBeaconMu protects the onFireBeacon test hook from concurrent access.
var onFireBeaconMu sync.Mutex

// onFireBeacon is a test hook called after fireBeacon decides to send or drop.
// When nil (production), no notification is sent. The bool arg is true if sent, false if dropped.
var onFireBeacon func(sent bool)

// setOnFireBeacon sets the test hook (use nil to clear).
func setOnFireBeacon(fn func(sent bool)) {
	onFireBeaconMu.Lock()
	onFireBeacon = fn
	onFireBeaconMu.Unlock()
}

func callOnFireBeacon(sent bool) {
	onFireBeaconMu.Lock()
	fn := onFireBeacon
	onFireBeaconMu.Unlock()
	if fn != nil {
		fn(sent)
	}
}

func fireBeacon(payload map[string]any) {
	if telemetryOptedOut() {
		callOnFireBeacon(false)
		return
	}

	beaconMu.RLock()
	ep := endpoint
	beaconMu.RUnlock()

	data, err := json.Marshal(payload)
	if err != nil {
		return // best-effort
	}

	select {
	case sem <- struct{}{}:
		util.SafeGo(func() {
			defer func() { <-sem }()

			resp, err := beaconClient.Post(ep, "application/json", bytes.NewReader(data))
			if err != nil {
				callOnFireBeacon(false)
				return // best-effort
			}
			// Drain body before close so the HTTP transport can reuse the connection.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			callOnFireBeacon(true)
		})
	default:
		// At capacity, drop this beacon silently
		callOnFireBeacon(false)
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
