// Purpose: Handles daemon recycle and zombie-process recovery during bridge startup.
// Why: Isolates recovery mechanics from connection orchestration and retry logic.

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/util"
)

func recoverVersionMismatchServer(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	server.logLifecycle("version_mismatch_recycle_start", port, nil)
	if !stopServerForUpgrade(port) {
		server.logLifecycle("version_mismatch_recycle_failed", port, nil)
		return false
	}
	return respawnDaemon(server, port, apiKey, mcpEndpoint)
}

func stopServerForUpgrade(port int) bool {
	_ = tryShutdownViaHTTP(port)
	if waitForPortRelease(port, 500*time.Millisecond) {
		removePIDFile(port)
		return true
	}

	pid := readPIDFile(port)
	if pid > 0 && pid != os.Getpid() {
		terminatePIDQuiet(pid, false)
	}

	pids, err := findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, false)
		}
	}

	if waitForPortRelease(port, 1500*time.Millisecond) {
		removePIDFile(port)
		return true
	}

	pids, err = findProcessOnPort(port)
	if err == nil {
		for _, pid := range pids {
			if pid == os.Getpid() {
				continue
			}
			terminatePIDQuiet(pid, true)
		}
	}

	released := waitForPortRelease(port, 1500*time.Millisecond)
	if released {
		removePIDFile(port)
	}
	return released
}

func tryShutdownViaHTTP(port int) bool {
	shutdownURL := fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	req, _ := http.NewRequest(http.MethodPost, shutdownURL, nil)
	resp, err := client.Do(req) // #nosec G704 -- shutdownURL is localhost-only from trusted port
	if err != nil {
		return false
	}
	_ = resp.Body.Close() // lint:body-close-ok one-shot shutdown probe
	return resp.StatusCode == http.StatusOK
}

func waitForPortRelease(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isServerRunning(port) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !isServerRunning(port)
}

func terminatePIDQuiet(pid int, force bool) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if force {
		_ = process.Kill()
		return
	}

	if runtime.GOOS == "windows" {
		_ = process.Kill()
		return
	}

	_ = process.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	if isProcessAlive(pid) {
		_ = process.Kill()
	}
}

// recoverZombieServer attempts to detect and kill a zombie server process,
// then respawn a fresh one. Returns true if recovery succeeded.
func recoverZombieServer(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	zombiePID := readPIDFile(port)
	if zombiePID <= 0 {
		return false
	}
	if !killZombieProcess(server, port, zombiePID) {
		return false
	}
	return respawnDaemon(server, port, apiKey, mcpEndpoint)
}

// killZombieProcess sends SIGTERM then SIGKILL to a zombie server process.
// Returns true if the process was found alive and terminated.
func killZombieProcess(server *Server, port int, zombiePID int) bool {
	zombieProcess, err := os.FindProcess(zombiePID)
	if err != nil {
		return false
	}
	if zombieProcess.Signal(syscall.Signal(0)) != nil {
		return false
	}

	server.logLifecycle("zombie_sigterm", port, map[string]any{"zombie_pid": zombiePID})
	_ = zombieProcess.Signal(syscall.SIGTERM)
	time.Sleep(2 * time.Second)

	if zombieProcess.Signal(syscall.Signal(0)) != nil {
		removePIDFile(port)
		return true
	}

	server.logLifecycle("zombie_sigkill", port, map[string]any{"zombie_pid": zombiePID})
	_ = zombieProcess.Signal(syscall.SIGKILL)
	time.Sleep(500 * time.Millisecond)
	removePIDFile(port)
	return true
}

// respawnDaemon starts a fresh daemon server and bridges stdin/stdout if successful.
func respawnDaemon(server *Server, port int, apiKey string, mcpEndpoint string) bool {
	server.logLifecycle("zombie_recovery_respawn", port, nil)
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to resolve executable path for respawn: %v\n", err)
		return false
	}
	args := []string{"--daemon", "--port", fmt.Sprintf("%d", port)}
	if stateDir := os.Getenv(state.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}
	if apiKey != "" {
		args = append(args, "--api-key", apiKey)
	}

	cmd := exec.Command(exe, args...) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags
	cmd.Args[0] = daemonProcessArgv0(exe)
	cmd.Stdout = nil
	cmd.Stderr = nil
	util.SetDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		sendStartupError("Failed to respawn after zombie recovery: " + err.Error())
		os.Exit(1)
	}

	if waitForServer(port, 10*time.Second) {
		server.logLifecycle("zombie_recovery_success", port, nil)
		bridgeStdioToHTTP(mcpEndpoint)
		return true
	}
	return false
}
