// Purpose: Implements top-level stop/force commands that orchestrate daemon shutdown strategies.
// Why: Keeps command flow readable while platform/process mechanics live in dedicated helpers.

package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// runStopMode gracefully stops a running server on the specified port.
// Uses hybrid approach: PID file (fast) -> HTTP /shutdown (graceful) -> platform-aware process kill (fallback).
func runStopMode(port int) {
	fmt.Printf("Stopping gasoline server on port %d...\n", port)
	logCommandInvocation("stop_command_invoked", "gasoline --stop", port)

	if stopViaPIDFile(port) {
		return
	}
	if stopViaHTTP(port) {
		return
	}
	stopViaProcessLookup(port)
}

// runForceCleanup kills ALL running gasoline daemons across all ports.
// Used during package install to ensure clean upgrade from older versions.
func runForceCleanup() {
	fmt.Println("Force cleanup: Killing all running gasoline daemons...")

	logFile := resolveLogFile()
	cleanupEntry := map[string]any{
		"type":       "lifecycle",
		"event":      "force_cleanup_invoked",
		"source":     "gasoline --force",
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONLogEntry(logFile, cleanupEntry)

	var killed, failedToKill int
	if runtime.GOOS != "windows" {
		killed, failedToKill = killUnixGasolineProcesses()
	} else {
		killed = killWindowsGasolineProcesses()
	}

	cleanupPIDFiles()
	printForceCleanupSummary(killed, failedToKill)
}
