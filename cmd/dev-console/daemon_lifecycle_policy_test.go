package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

func writeDaemonPIDFileForTest(t *testing.T, port int, pid int) {
	t.Helper()
	path := pidFilePath(port)
	if path == "" {
		t.Fatal("pidFilePath returned empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readLifecycleEventsFromLogFile(t *testing.T, logFile string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logFile, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("json.Unmarshal(log line) error = %v, line=%q", err, line)
		}
		if event, _ := entry["event"].(string); event != "" {
			events = append(events, entry)
		}
	}
	return events
}

func TestEnforceDaemonStartupPolicy_DefaultTakeover(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	const existingPID = 42424
	const existingPort = 7890
	const requestedPort = 7891

	if err := writeDaemonLockFile(daemonLockRecord{
		PID:      existingPID,
		Port:     existingPort,
		StateDir: stateRoot,
		Version:  "0.7.7",
	}); err != nil {
		t.Fatalf("writeDaemonLockFile() error = %v", err)
	}
	writeDaemonPIDFileForTest(t, existingPort, existingPID)

	logFile := filepath.Join(t.TempDir(), "daemon-policy.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	oldIsAlive := daemonIsProcessAlive
	oldTryShutdown := daemonTryShutdown
	oldWaitRelease := daemonWaitForPortRelease
	oldTerminate := daemonTerminatePID
	defer func() {
		daemonIsProcessAlive = oldIsAlive
		daemonTryShutdown = oldTryShutdown
		daemonWaitForPortRelease = oldWaitRelease
		daemonTerminatePID = oldTerminate
	}()

	daemonIsProcessAlive = func(pid int) bool { return pid == existingPID }
	daemonTryShutdown = func(port int) bool { return port == existingPort }
	waitCalls := 0
	daemonWaitForPortRelease = func(port int, _ time.Duration) bool {
		if port != existingPort {
			return false
		}
		waitCalls++
		return waitCalls >= 2
	}
	terminatedPIDs := make([]int, 0, 2)
	daemonTerminatePID = func(pid int, _ bool) {
		terminatedPIDs = append(terminatedPIDs, pid)
	}

	if err := enforceDaemonStartupPolicy(server, requestedPort, daemonLaunchOptions{}); err != nil {
		t.Fatalf("enforceDaemonStartupPolicy() error = %v", err)
	}

	if len(terminatedPIDs) != 1 || terminatedPIDs[0] != existingPID {
		t.Fatalf("terminate calls = %v, want [%d]", terminatedPIDs, existingPID)
	}

	lockAfter, err := readDaemonLockFile()
	if err != nil {
		t.Fatalf("readDaemonLockFile() error = %v", err)
	}
	if lockAfter != nil {
		t.Fatalf("daemon lock should be removed after takeover, got %+v", *lockAfter)
	}

	if _, err := os.Stat(pidFilePath(existingPort)); !os.IsNotExist(err) {
		t.Fatalf("pid file for existing port should be removed, stat err = %v", err)
	}

	server.shutdownAsyncLogger(2 * time.Second)
	events := readLifecycleEventsFromLogFile(t, logFile)
	var takeover map[string]any
	for _, evt := range events {
		if evtName, _ := evt["event"].(string); evtName == "daemon_takeover" {
			takeover = evt
			break
		}
	}
	if takeover == nil {
		t.Fatal("expected daemon_takeover lifecycle event")
	}
	if got, _ := takeover["existing_pid"].(float64); int(got) != existingPID {
		t.Fatalf("daemon_takeover existing_pid = %v, want %d", takeover["existing_pid"], existingPID)
	}
	if got, _ := takeover["existing_port"].(float64); int(got) != existingPort {
		t.Fatalf("daemon_takeover existing_port = %v, want %d", takeover["existing_port"], existingPort)
	}
	if got, _ := takeover["new_pid"].(float64); int(got) != os.Getpid() {
		t.Fatalf("daemon_takeover new_pid = %v, want %d", takeover["new_pid"], os.Getpid())
	}
	if takeoverFlag, _ := takeover["takeover"].(bool); !takeoverFlag {
		t.Fatalf("daemon_takeover takeover = %v, want true", takeover["takeover"])
	}
	if stateDir, _ := takeover["state_dir"].(string); stateDir != stateRoot {
		t.Fatalf("daemon_takeover state_dir = %q, want %q", stateDir, stateRoot)
	}
}

func TestEnforceDaemonStartupPolicy_SafetyGuardRejectsPIDMismatch(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	const existingPID = 51515
	const existingPort = 7900

	if err := writeDaemonLockFile(daemonLockRecord{
		PID:      existingPID,
		Port:     existingPort,
		StateDir: stateRoot,
		Version:  "0.7.7",
	}); err != nil {
		t.Fatalf("writeDaemonLockFile() error = %v", err)
	}

	writeDaemonPIDFileForTest(t, existingPort, existingPID+1)

	logFile := filepath.Join(t.TempDir(), "daemon-policy-mismatch.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.shutdownAsyncLogger(2 * time.Second)

	oldIsAlive := daemonIsProcessAlive
	oldTerminate := daemonTerminatePID
	defer func() {
		daemonIsProcessAlive = oldIsAlive
		daemonTerminatePID = oldTerminate
	}()
	daemonIsProcessAlive = func(pid int) bool { return pid == existingPID }
	terminated := false
	daemonTerminatePID = func(_ int, _ bool) { terminated = true }

	err = enforceDaemonStartupPolicy(server, 7901, daemonLaunchOptions{})
	if err == nil {
		t.Fatal("enforceDaemonStartupPolicy() error = nil, want ownership mismatch error")
	}
	if !strings.Contains(err.Error(), "ownership mismatch") {
		t.Fatalf("error = %q, want ownership mismatch guidance", err.Error())
	}
	if terminated {
		t.Fatal("safety guard should not terminate process on PID mismatch")
	}
}

func TestEnforceDaemonStartupPolicy_ParallelRequiresIsolatedStateDir(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	if err := writeDaemonLockFile(daemonLockRecord{
		PID:      30303,
		Port:     7920,
		StateDir: stateRoot,
		Version:  "0.7.7",
	}); err != nil {
		t.Fatalf("writeDaemonLockFile() error = %v", err)
	}

	logFile := filepath.Join(t.TempDir(), "daemon-policy-parallel.log")
	server, err := NewServer(logFile, 200)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.shutdownAsyncLogger(2 * time.Second)

	oldIsAlive := daemonIsProcessAlive
	oldTerminate := daemonTerminatePID
	oldTryShutdown := daemonTryShutdown
	defer func() {
		daemonIsProcessAlive = oldIsAlive
		daemonTerminatePID = oldTerminate
		daemonTryShutdown = oldTryShutdown
	}()
	daemonIsProcessAlive = func(pid int) bool { return pid == 30303 }
	terminated := false
	shutdownCalled := false
	daemonTerminatePID = func(_ int, _ bool) { terminated = true }
	daemonTryShutdown = func(_ int) bool {
		shutdownCalled = true
		return false
	}

	err = enforceDaemonStartupPolicy(server, 7921, daemonLaunchOptions{Parallel: true})
	if err == nil {
		t.Fatal("enforceDaemonStartupPolicy() error = nil, want isolated state-dir error")
	}
	if !strings.Contains(err.Error(), "isolated --state-dir") {
		t.Fatalf("error = %q, want isolated state-dir guidance", err.Error())
	}
	if terminated || shutdownCalled {
		t.Fatalf("parallel mode should not takeover/kill existing daemon; terminated=%v shutdownCalled=%v", terminated, shutdownCalled)
	}
}
