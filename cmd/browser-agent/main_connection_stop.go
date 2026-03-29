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
	fmt.Printf("Stopping kaboom server on port %d...\n", port)
	logCommandInvocation("stop_command_invoked", "kaboom --stop", port)

	if stopViaPIDFile(port) {
		return
	}
	if stopViaHTTP(port) {
		return
	}
	stopViaProcessLookup(port)
}

// runForceCleanup kills ALL running kaboom daemons across all ports.
// Used during package install to ensure clean upgrade from older versions.
func runForceCleanup() {
	fmt.Println("Force cleanup: Killing all running kaboom daemons...")

	logFile := resolveLogFile()
	cleanupEntry := map[string]any{
		"type":       "lifecycle",
		"event":      "force_cleanup_invoked",
		"source":     "kaboom --force",
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONLogEntry(logFile, cleanupEntry)

	var killed, failedToKill int
	if runtime.GOOS != "windows" {
		killed, failedToKill = killUnixKaboomProcesses()
	} else {
		killed = killWindowsKaboomProcesses()
	}

	cleanupPIDFiles()
	printForceCleanupSummary(killed, failedToKill)
}
