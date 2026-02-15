// main_connection_stop.go — Server stop and force cleanup operations.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/state"
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

// logCommandInvocation writes a lifecycle log entry for a stop or cleanup command.
func logCommandInvocation(event string, source string, port int) {
	logFile := resolveLogFile()
	entry := map[string]any{
		"type":       "lifecycle",
		"event":      event,
		"port":       port,
		"source":     source,
		"caller_pid": os.Getpid(),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	writeJSONLogEntry(logFile, entry)
}

// resolveLogFile determines the log file path with fallbacks.
func resolveLogFile() string {
	logFile, err := state.DefaultLogFile()
	if err != nil {
		if legacy, legacyErr := state.LegacyDefaultLogFile(); legacyErr == nil {
			return legacy
		}
		return filepath.Join(os.TempDir(), "gasoline.jsonl")
	}
	return logFile
}

// writeJSONLogEntry marshals and appends a JSON entry to the given log file.
func writeJSONLogEntry(logFile string, entry map[string]any) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	_ = os.MkdirAll(filepath.Dir(logFile), 0o750)
	// #nosec G304 -- log file path resolved from trusted runtime state directory
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600) // nosemgrep: go_filesystem_rule-fileread
	if err != nil {
		return
	}
	_, _ = f.Write(data)
	_, _ = f.Write([]byte{'\n'})
	_ = f.Close()
}

// stopViaPIDFile attempts to stop the server using the PID file (fast path).
// Returns true if the server was stopped successfully.
func stopViaPIDFile(port int) bool {
	pid := readPIDFile(port)
	if pid <= 0 || !isProcessAlive(pid) {
		return false
	}

	fmt.Printf("Found server (PID %d) via PID file\n", pid)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		return false
	}
	fmt.Printf("Sent SIGTERM to PID %d\n", pid)

	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isProcessAlive(pid) {
			fmt.Println("Server stopped successfully")
			removePIDFile(port)
			return true
		}
	}

	fmt.Println("Server did not exit within 2 seconds, sending SIGKILL")
	_ = process.Kill()
	removePIDFile(port)
	fmt.Println("Server killed")
	return true
}

// stopViaHTTP attempts to stop the server using the /shutdown HTTP endpoint.
// Returns true if the server acknowledged the shutdown.
func stopViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("POST", shutdownURL, nil)
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		fmt.Println("Server stopped via HTTP endpoint")
		removePIDFile(port)
		return true
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	return false
}

// stopViaProcessLookup finds processes on the port and terminates them.
func stopViaProcessLookup(port int) {
	fmt.Println("Trying process lookup fallback...")
	pids, findErr := findProcessOnPort(port)
	if findErr != nil || len(pids) == 0 {
		fmt.Printf("No server found on port %d\n", port)
		removePIDFile(port)
		return
	}

	for _, pidNum := range pids {
		fmt.Printf("Sending termination signal to PID %d\n", pidNum)
		_ = killProcessByPID(pidNum)
	}

	time.Sleep(500 * time.Millisecond)
	if !isServerRunning(port) {
		fmt.Println("Server stopped successfully")
		removePIDFile(port)
	} else {
		fmt.Printf("Server may still be running, try: %s\n", portKillHintForce(port))
	}
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

// killUnixGasolineProcesses finds and kills gasoline processes on Unix systems
// using lsof and pkill. Returns (killed, failedToKill) counts.
func killUnixGasolineProcesses() (int, int) {
	killed := 0
	failedToKill := 0

	cmd := exec.Command("lsof", "-c", "gasoline")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			pid, err := strconv.Atoi(fields[1])
			if err != nil || pid <= 0 {
				continue
			}
			k, f := terminateProcess(pid)
			killed += k
			failedToKill += f
		}
	}

	// Also try pkill as fallback
	pkillCmd := exec.Command("pkill", "-f", "gasoline.*--daemon")
	_ = pkillCmd.Run()

	return killed, failedToKill
}

// terminateProcess sends SIGTERM then SIGKILL to a process. Returns (killed, failed) counts.
func terminateProcess(pid int) (int, int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, 0
	}
	if err := process.Signal(syscall.SIGTERM); err == nil {
		fmt.Printf("  Sent SIGTERM to PID %d\n", pid)
		time.Sleep(100 * time.Millisecond)
		if !isProcessAlive(pid) {
			return 1, 0
		}
	}
	if err := process.Kill(); err == nil {
		fmt.Printf("  Sent SIGKILL to PID %d\n", pid)
		return 1, 0
	}
	return 0, 1
}

// killWindowsGasolineProcesses kills gasoline processes on Windows using taskkill.
func killWindowsGasolineProcesses() int {
	killed := 0
	cmd := exec.Command("taskkill", "/IM", "gasoline.exe", "/F")
	output, err := cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "SUCCESS") || strings.Contains(line, "terminated") {
				killed++
			}
		}
	}
	return killed
}

// cleanupPIDFiles removes PID files for common port range.
func cleanupPIDFiles() {
	ports := []int{17890}
	for p := 7890; p <= 7910; p++ {
		ports = append(ports, p)
	}
	for _, p := range ports {
		removePIDFile(p)
	}
}

// printForceCleanupSummary outputs the results of the force cleanup operation.
func printForceCleanupSummary(killed, failedToKill int) {
	fmt.Println()
	if killed > 0 {
		fmt.Printf("✓ Successfully killed %d gasoline process(es)\n", killed)
	}
	if failedToKill > 0 {
		fmt.Printf("⚠ Failed to kill %d process(es) (may have already exited)\n", failedToKill)
	}
	if killed == 0 && failedToKill == 0 {
		fmt.Println("✓ No running gasoline processes found")
	}
	fmt.Println()
	fmt.Println("Cleaned up PID files. Safe to proceed with installation.")
}
