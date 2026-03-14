// Purpose: Binary-upgrade detection wiring for MCP daemon startup.
// Why: Keeps upgrade monitoring and marker handoff separate from core MCP boot flow.

package main

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

// binaryUpgradeState tracks whether a binary upgrade has been detected on disk.
// Read by maybeAddUpgradeWarning() and buildUpgradeInfo() in health response builders.
var binaryUpgradeState *BinaryWatcherState

// persistUpgradeMarker writes a marker file so the next process knows it was upgraded.
func persistUpgradeMarker() {
	if binaryUpgradeState == nil {
		return
	}
	_, newVer, _ := binaryUpgradeState.UpgradeInfo()
	if newVer == "" {
		return
	}
	if markerPath, err := state.UpgradeMarkerFile(); err == nil {
		_ = writeUpgradeMarker(version, newVer, markerPath)
	}
}

// selfTerminate sends SIGTERM to the current process to trigger graceful shutdown.
func selfTerminate() {
	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)
}

// announceCompletedUpgrade checks for a previous upgrade marker and logs/warns if found.
func announceCompletedUpgrade(server *Server, port int) {
	markerPath, err := state.UpgradeMarkerFile()
	if err != nil {
		return
	}
	marker, err := readAndClearUpgradeMarker(markerPath)
	if err != nil || marker == nil {
		return
	}
	server.AddWarning(fmt.Sprintf("Upgraded from v%s to v%s", marker.FromVersion, marker.ToVersion))
	server.logLifecycle("binary_upgrade_complete", port, map[string]any{
		"from_version": marker.FromVersion,
		"to_version":   marker.ToVersion,
	})
}

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
			persistUpgradeMarker()
			server.logLifecycle("binary_upgrade_shutdown", port, map[string]any{"version": version})
			selfTerminate()
		},
	)

	announceCompletedUpgrade(server, port)
}
