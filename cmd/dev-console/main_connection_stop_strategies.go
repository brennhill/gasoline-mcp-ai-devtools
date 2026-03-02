// Purpose: Concrete shutdown strategies for stop/force-cleanup commands.
// Why: Separates platform/process termination mechanics from top-level command flow.

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
		_ = resp.Body.Close() // lint:body-close-ok immediate close on success path
		fmt.Println("Server stopped via HTTP endpoint")
		removePIDFile(port)
		return true
	}
	if resp != nil {
		_ = resp.Body.Close() // lint:body-close-ok immediate close before fallback path
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

func killUnixGasolineProcessesQuietly() (int, int) {
	cmd := exec.Command("pkill", "-f", "gasoline.*--daemon")
	_ = cmd.Run()
	return 0, 0
}

func killWindowsGasolineProcessesQuietly() int {
	cmd := exec.Command("taskkill", "/IM", "gasoline.exe", "/F")
	_ = cmd.Run()
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
