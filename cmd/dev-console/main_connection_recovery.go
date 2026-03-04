// Purpose: Handles daemon recycle and zombie-process recovery during bridge startup.
// Why: Isolates recovery mechanics from connection orchestration and retry logic.

package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"
)

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
