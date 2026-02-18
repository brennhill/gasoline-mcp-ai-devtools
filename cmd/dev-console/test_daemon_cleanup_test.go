package main

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestMain enforces process hygiene for the full cmd/dev-console test suite.
// Some tests spawn client processes that in turn spawn detached daemons.
// We run cleanup before and after tests to prevent stale daemons from accumulating.
func TestMain(m *testing.M) {
	cleanupGoTestDaemons()
	code := m.Run()
	cleanupGoTestDaemons()
	os.Exit(code)
}

func cleanupGoTestDaemons() {
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/IM", "gasoline-test-binary.exe").Run()
		return
	}

	killPattern("gasoline-test-binary --daemon --port")
	killPattern("gasoline-test-binary --port")

	// Clean known test PID file ranges used by shell and regression tests.
	cleanupPIDFiles()
	for port := 17890; port <= 17999; port++ {
		removePIDFile(port)
	}
}

func killPattern(pattern string) {
	_ = exec.Command("pkill", "-TERM", "-f", pattern).Run()
	time.Sleep(200 * time.Millisecond)
	_ = exec.Command("pkill", "-KILL", "-f", pattern).Run()
}
