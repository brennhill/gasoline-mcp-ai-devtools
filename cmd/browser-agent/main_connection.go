// Purpose: Orchestrates daemon discovery, spawn, health-check, and version-mismatch handling for bridge client connections.
// Why: Handles the complex startup handshake where a bridge client must find or launch a compatible daemon.
//
// Metrics: this file defines (*Server).logLifecycle(event, port, fields)
// — the canonical structured-log helper that EVERY lifecycle/error event
// in the daemon flows through. Each entry is shaped:
//
//   { type: "lifecycle", event: "<name>", pid, port, timestamp, ...fields }
//
// Dashboards consume the `event` string as a metric key. Callers that
// emit a NEW event name MUST list it in their file header so the full set
// is discoverable without grepping. Current emitters (non-exhaustive):
//   - main_runtime_mode.go         (mode_detection, daemon_mode_start, …)
//   - main_connection_mcp.go       (startup, terminal_server_*, …)
//   - main_connection_mcp_bootstrap.go (http_bind_*, stale_pid_*, …)
//   - main_connection_mcp_shutdown.go (shutdown, *_shutdown_error)
//   - main_connection_mcp_upgrade.go (binary_upgrade_*)
//   - daemon_lifecycle.go          (daemon_takeover, daemon_lock_*)
//   - screenshot_cleanup.go        (screenshot_cleanup_*)
//
// Lifecycle events are local logs — they DO NOT fire app-telemetry
// beacons. For wire-bound metrics see internal/telemetry/.

package main

import (
	"os"
	"time"
)

// logLifecycle is a convenience method to emit a structured lifecycle log entry.
func (s *Server) logLifecycle(event string, port int, extra map[string]any) {
	entry := LogEntry{
		"type":      "lifecycle",
		"event":     event,
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if port != 0 {
		entry["port"] = port
	}
	for k, v := range extra {
		entry[k] = v
	}
	s.logs.addEntries([]LogEntry{entry})
}
