// Purpose: Force-cleanup process termination helpers for cross-port daemon cleanup.
// Why: Keeps broad process sweep and summary reporting separate from graceful single-port stop strategies.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// terminateSignalSettleDelay is the pause after SIGTERM before checking if the
	// process exited, allowing it time to shut down gracefully.
	terminateSignalSettleDelay = 100 * time.Millisecond
)

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

	// Also try pkill as fallback.
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
		time.Sleep(terminateSignalSettleDelay)
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

func killUnixGasolineProcessesQuietly() (int, int) {
	cmd := exec.Command("pkill", "-f", "gasoline.*--daemon")
	_ = cmd.Run() //nolint:errcheck // best-effort process cleanup; exit code irrelevant
	return 0, 0
}

func killWindowsGasolineProcessesQuietly() int {
	cmd := exec.Command("taskkill", "/IM", "gasoline.exe", "/F")
	_ = cmd.Run() //nolint:errcheck // best-effort process cleanup; exit code irrelevant
	return 0
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

func runForceCleanupQuietly() error {
	if runtime.GOOS != "windows" {
		_, _ = killUnixGasolineProcessesQuietly()
	} else {
		_ = killWindowsGasolineProcessesQuietly()
	}
	cleanupPIDFiles()
	return nil
}
