// Purpose: Binary-upgrade detection wiring for MCP daemon startup.
// Why: Keeps upgrade monitoring and marker handoff separate from core MCP boot flow.
//
// Metrics emitted from this file (all via logLifecycle):
//   - binary_upgrade_detected   — installer wrote a new binary alongside
//                                 the running daemon; respawn pending.
//   - binary_upgrade_shutdown   — we initiated the supervisor-mediated
//                                 restart.
//   - binary_upgrade_complete   — successor daemon reported healthy.
//
// These are local-only structured logs. Self-update beacons (fired from
// the popup-driven flow) are unrelated and live in
// cmd/browser-agent/server_routes_upgrade.go.

package main

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// binaryUpgradeState tracks whether a binary upgrade has been detected on disk.
// Read by maybeAddUpgradeWarning() and buildUpgradeInfo() in health response builders.
var binaryUpgradeState *BinaryWatcherState

func configureBinaryUpgradeMonitoring(ctx context.Context, server *Server, port int) {
	binaryUpgradeState = startBinaryWatcher(ctx, version,
		func(newVersion string) {
			server.logLifecycle("binary_upgrade_detected", port, map[string]any{
				"current_version": version,
				"new_version":     newVersion,
			})
			server.AddWarning("UPGRADE DETECTED: v" + newVersion + " installed. Auto-restart in ~5s.")
		},
		func() {
			if binaryUpgradeState != nil {
				if _, newVer, _ := binaryUpgradeState.UpgradeInfo(); newVer != "" {
					if markerPath, err := state.UpgradeMarkerFile(); err == nil {
						_ = writeUpgradeMarker(version, newVer, markerPath)
					}
				}
			}
			server.logLifecycle("binary_upgrade_shutdown", port, map[string]any{"version": version})
			p, _ := os.FindProcess(os.Getpid())
			_ = p.Signal(syscall.SIGTERM)
		},
	)

	if markerPath, err := state.UpgradeMarkerFile(); err == nil {
		if marker, err := readAndClearUpgradeMarker(markerPath); err == nil && marker != nil {
			server.AddWarning(fmt.Sprintf("Upgraded from v%s to v%s", marker.FromVersion, marker.ToVersion))
			server.logLifecycle("binary_upgrade_complete", port, map[string]any{
				"from_version": marker.FromVersion,
				"to_version":   marker.ToVersion,
			})
		}
	}
}
