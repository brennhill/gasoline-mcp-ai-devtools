// beacon.go — Anonymous telemetry beacons. Disable with KABOOM_TELEMETRY=off.
//
// Defines the metric-emission infrastructure that every other emitter in
// the codebase calls into:
//   - BeaconEvent(name, fields)       — fire-and-forget single event.
//   - AppError(category, kind, ...)   — structured error telemetry.
//   - Warm()                          — pre-load install ID + session ID
//                                        off the hot path.
//   - buildEnvelope(event)            — stamps the contract envelope
//                                        (event/v/os/iid/sid/llm) onto
//                                        every payload; drops the beacon
//                                        when no install ID exists.
//
// Wire contract: docs/core/app-metrics.md (POST /v1/event).
// Producer audit (every emission site in the codebase): see the bridge
// classifier in this file's classifyAppErrorEvent and grep for
// `telemetry.BeaconEvent` / `telemetry.AppError`.
//
// Adding a new event name here REQUIRES updating docs/core/app-metrics.md
// (the contract dashboards consume) and the calling file's header.

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
// Only includes fields defined in the Counterscale contract shared envelope.
// Beacons are dropped entirely when a stable install ID is unavailable.
func buildEnvelope(event string) (map[string]any, bool) {
	beaconMu.RLock()
	llm := llmName
	beaconMu.RUnlock()

	installID := GetInstallID()
	if installID == "" {
		return nil, false
	}

	env := map[string]any{
		"event": event,
		"v":     Version,
		"os":    runtime.GOOS + "-" + runtime.GOARCH,
		"iid":   installID,
		"sid":   GetSessionID(),
	}
	if llm != "" {
		env["llm"] = llm
	}
	return env, true
}

// AppError fires a structured app_error event.
// Props are merged first so contract fields (error_kind, severity, etc.) always win.
func AppError(category string, props map[string]string) {
	errorKind, severity, source, retryable := classifyAppError(category)

	// Apply caller props first so contract fields cannot be overwritten.
	fields := map[string]any{}
	for k, v := range props {
		fields[k] = v
	}

	// Contract fields applied last — always authoritative.
	fields["event"] = "app_error"
	fields["error_kind"] = errorKind
	fields["error_code"] = normalizeAppErrorCode(category)
	fields["severity"] = severity
	fields["source"] = source
	if retryable {
		fields["retryable"] = true
	}
	fireStructuredBeacon(fields)
}

func classifyAppError(category string) (errorKind string, severity string, source string, retryable bool) {
	switch category {
	case "daemon_panic":
		return "internal", "fatal", "daemon", false
	case "daemon_start_failed":
		return "internal", "fatal", "startup", false
	case "tool_rate_limited":
		return "integration", "warning", "daemon", true
	case "bridge_connection_error":
		return "integration", "error", "bridge", true
	case "bridge_port_blocked":
		return "integration", "error", "bridge", false
	case "bridge_spawn_build_error", "bridge_spawn_start_error":
		return "internal", "fatal", "bridge", false
	case "bridge_spawn_timeout":
		return "internal", "error", "bridge", true
	case "bridge_exit_error":
		// Retained for backward compat with historical data; no longer emitted.
		// New emissions use bridge_parse_error, bridge_method_not_found, or bridge_stdin_error.
		return "internal", "error", "bridge", false
	case "bridge_parse_error":
		// Bridge received malformed JSON-RPC from the MCP client. Caller-side
		// defect — classify as integration/warning so dashboards don't page
		// us for client bugs we can't fix.
		return "integration", "warning", "bridge", false
	case "bridge_method_not_found":
		// Bridge received a JSON-RPC method it does not implement. Client bug or version skew.
		return "integration", "warning", "bridge", false
	case "bridge_stdin_error":
		// Stdin read failed mid-stream (pipe broken, process killed, fd issue). Env/transport.
		return "internal", "error", "bridge", false
	case "extension_disconnect":
		return "integration", "warning", "extension", false
	case "install_config_error":
		return "internal", "error", "installer", false
	case "install_id_migrated":
		// Identity-lineage marker: the persisted install ID differs from the
		// deterministic derivation (hostname/uid/machine_id changed). Not an
		// error — informational so analytics can stitch a single install
		// across the change. integration/warning so dashboards don't page.
		return "integration", "warning", "daemon", false
	default:
		return "unknown", "error", "daemon", false
	}
}

func normalizeAppErrorCode(category string) string {
	replacer := strings.NewReplacer("-", "_", " ", "_")
	return strings.ToUpper(replacer.Replace(strings.TrimSpace(category)))
}

// BeaconEvent fires an anonymous lifecycle event.
// Fire-and-forget: backgrounded, 2s timeout, never blocks caller, never panics.
func BeaconEvent(event string, props map[string]string) {
	sendBeacon(event, props)
}

// BeaconUsageSummary fires a structured usage_summary beacon.
func BeaconUsageSummary(windowMinutes int, snapshot *UsageSnapshot) {
	payload := BuildUsageSummaryPayload(windowMinutes, snapshot)
	if payload == nil {
		return
	}
	fireBeacon(payload)
}

// BuildUsageSummaryPayload builds the beacon payload without sending it.
// Used by debug endpoints to inspect what would be sent.
func BuildUsageSummaryPayload(windowMinutes int, snapshot *UsageSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	payload, ok := buildEnvelope("usage_summary")
	if !ok {
		return nil
	}
	payload["ts"] = time.Now().UTC().Format(time.RFC3339)
	payload["channel"] = Channel
	payload["window_m"] = windowMinutes
	payload["tool_stats"] = snapshot.ToolStats
	if len(snapshot.AsyncOutcomes) > 0 {
		payload["async_outcomes"] = snapshot.AsyncOutcomes
	}
	return payload
}

func sendBeacon(event string, props map[string]string) {
	payload, ok := buildEnvelope(event)
	if !ok {
		callOnFireBeacon(false)
		return
	}
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

// setOnFireBeacon test helper lives in helpers_test.go.

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

// overrideEndpoint / resetEndpoint test helpers live in helpers_test.go.
