package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

func writeExecutableScript(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func withTestStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdin) error = %v", err)
	}
	if _, err := io.WriteString(w, input); err != nil {
		t.Fatalf("WriteString(stdin) error = %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()
	fn()
}

func waitForProcessExit(t *testing.T, cmd *exec.Cmd, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		t.Fatalf("process %d did not exit within %s", cmd.Process.Pid, timeout)
	}
}

func TestHandleMCPConnectionConnectExistingSuccess(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "connection.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.shutdownAsyncLogger(2 * time.Second)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","service-name":"gasoline","version":"`+version+`"}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`+"\n")
	})
	httpSrv := &http.Server{Handler: mux}
	go func() { _ = httpSrv.Serve(ln) }()
	defer func() { _ = httpSrv.Close() }()

	output := captureStdout(t, func() {
		withTestStdin(t, `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n", func() {
			handleMCPConnection(server, port, "")
		})
	})
	if !strings.Contains(output, `"jsonrpc":"2.0"`) || !strings.Contains(output, `"id":1`) {
		t.Fatalf("bridge output missing JSON-RPC response: %q", output)
	}
}

func TestConnectWithRetriesRejectsVersionMismatch(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "connection-version-mismatch.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.shutdownAsyncLogger(2 * time.Second)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","service-name":"gasoline","version":"0.0.1"}`)
	})
	httpSrv := &http.Server{Handler: mux}
	go func() { _ = httpSrv.Serve(ln) }()
	defer func() { _ = httpSrv.Close() }()

	healthURL := "http://127.0.0.1:" + strconv.Itoa(port) + "/health"
	err = connectWithRetries(server, healthURL, "http://127.0.0.1:1/mcp", &debugWriter{port: port})
	if err == nil {
		t.Fatal("connectWithRetries() error = nil, want version mismatch error")
	}
	var mismatchErr *serverVersionMismatchError
	if !errors.As(err, &mismatchErr) {
		t.Fatalf("connectWithRetries() error = %v, want serverVersionMismatchError", err)
	}
}

func TestConnectWithRetriesRejectsNonGasolineService(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "connection-non-gasoline.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.shutdownAsyncLogger(2 * time.Second)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok","service-name":"other-service","version":"`+version+`"}`)
	})
	httpSrv := &http.Server{Handler: mux}
	go func() { _ = httpSrv.Serve(ln) }()
	defer func() { _ = httpSrv.Close() }()

	healthURL := "http://127.0.0.1:" + strconv.Itoa(port) + "/health"
	err = connectWithRetries(server, healthURL, "http://127.0.0.1:1/mcp", &debugWriter{port: port})
	if err == nil {
		t.Fatal("connectWithRetries() error = nil, want non-gasoline service error")
	}
	var nonGasolineErr *nonGasolineServiceError
	if !errors.As(err, &nonGasolineErr) {
		t.Fatalf("connectWithRetries() error = %v, want nonGasolineServiceError", err)
	}
}

func TestRunMCPModePortConflictRemovesStalePID(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	oldGitHubURL := getGitHubAPIURL()
	setGitHubAPIURL("http://127.0.0.1:1/releases/latest")
	defer setGitHubAPIURL(oldGitHubURL)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	pidPath := pidFilePath(port)
	if pidPath == "" {
		t.Fatal("pidFilePath returned empty path")
	}
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(pidPath), err)
	}
	if err := os.WriteFile(pidPath, []byte("999999"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", pidPath, err)
	}

	logFile := filepath.Join(t.TempDir(), "mcp-mode.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	defer server.shutdownAsyncLogger(2 * time.Second)

	err = runMCPMode(server, port, "")
	if err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("runMCPMode() error = %v, want port conflict", err)
	}

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid file to be removed, stat err = %v", err)
	}
}

func TestRunMCPModeGracefulSignalShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal-based shutdown path under test")
	}

	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	oldGitHubURL := getGitHubAPIURL()
	setGitHubAPIURL("http://127.0.0.1:1/releases/latest")
	defer setGitHubAPIURL(oldGitHubURL)

	port := freePortForTest(t)
	logFile := filepath.Join(t.TempDir(), "mcp-signal.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- runMCPMode(server, port, "")
	}()

	deadline := time.Now().Add(5 * time.Second)
	for !isServerRunning(port) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if !isServerRunning(port) {
		t.Fatal("runMCPMode server did not become ready")
	}

	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess(self) error = %v", err)
	}
	if err := self.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("self.Signal(SIGTERM) error = %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runMCPMode() returned error on graceful shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runMCPMode did not return after SIGTERM")
	}
}

func TestRunStopModePIDFastPathKillsProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal semantics differ on Windows")
	}

	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	port := freePortForTest(t)

	sleepCmd := exec.Command("sh", "-c", "sleep 30")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("sleep process start error = %v", err)
	}
	t.Cleanup(func() {
		if sleepCmd.Process != nil {
			_ = sleepCmd.Process.Kill()
		}
	})

	pidPath := pidFilePath(port)
	if err := os.MkdirAll(filepath.Dir(pidPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(pidPath), err)
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(sleepCmd.Process.Pid)), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", pidPath, err)
	}

	out := captureStdout(t, func() {
		runStopMode(port)
	})

	waitForProcessExit(t, sleepCmd, 3*time.Second)
	if !strings.Contains(out, "Found server (PID") {
		t.Fatalf("runStopMode output missing PID fast-path message: %q", out)
	}
	if !strings.Contains(out, "Server stopped successfully") && !strings.Contains(out, "Server killed") {
		t.Fatalf("runStopMode output missing stop confirmation: %q", out)
	}
}

func TestRunForceCleanupKillsListedProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process discovery path under test")
	}

	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	sleepCmd := exec.Command("sh", "-c", "sleep 30")
	if err := sleepCmd.Start(); err != nil {
		t.Fatalf("sleep process start error = %v", err)
	}
	t.Cleanup(func() {
		if sleepCmd.Process != nil {
			_ = sleepCmd.Process.Kill()
		}
	})

	fakeBin := t.TempDir()
	writeExecutableScript(t, fakeBin, "lsof", fmt.Sprintf(`
if [ "$1" = "-c" ] && [ "$2" = "gasoline" ]; then
  echo "COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME"
  echo "gasoline %d user 0u IPv4 0t0 TCP *:7890 (LISTEN)"
  exit 0
fi
exit 1
`, sleepCmd.Process.Pid))
	writeExecutableScript(t, fakeBin, "pkill", "exit 0")

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	out := captureStdout(t, runForceCleanup)
	waitForProcessExit(t, sleepCmd, 3*time.Second)

	if !strings.Contains(out, "Force cleanup: Killing all running gasoline daemons") {
		t.Fatalf("runForceCleanup output missing header: %q", out)
	}
	if !strings.Contains(out, "Successfully killed") {
		t.Fatalf("runForceCleanup output missing kill summary: %q", out)
	}
}

func TestGatherConnectionDiagnosticsConflictProcess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "healthy")
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, "not ready")
	})
	httpSrv := &http.Server{Handler: mux}
	go func() { _ = httpSrv.Serve(ln) }()
	defer func() { _ = httpSrv.Close() }()

	fakeBin := t.TempDir()
	writeExecutableScript(t, fakeBin, "lsof", fmt.Sprintf(`echo "%d"`, port+1000))
	writeExecutableScript(t, fakeBin, "ps", `echo "/usr/bin/python3 app.py"`)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	serverURL := "http://127.0.0.1:" + strconv.Itoa(port)
	diag := gatherConnectionDiagnostics(port, serverURL, serverURL+"/health")

	if got, _ := diag["process_type"].(string); got != "NOT gasoline (conflict)" {
		t.Fatalf("process_type = %q, want conflict", got)
	}
	if got, _ := diag["diagnosis"].(string); got != "Port occupied by different service" {
		t.Fatalf("diagnosis = %q, want port conflict diagnosis", got)
	}
	if got, _ := diag["health_response"].(string); got != "unexpected response format" {
		t.Fatalf("health_response = %q, want unexpected response format", got)
	}
}

func TestGatherConnectionDiagnosticsHealthFailureForGasoline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	fakeBin := t.TempDir()
	writeExecutableScript(t, fakeBin, "lsof", `echo "98765"`)
	writeExecutableScript(t, fakeBin, "ps", `echo "gasoline --daemon --port 7890"`)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	serverURL := "http://127.0.0.1:" + strconv.Itoa(port)
	diag := gatherConnectionDiagnostics(port, serverURL, "http://127.0.0.1:1/health")

	if got, _ := diag["port_status"].(string); got != "listening" {
		t.Fatalf("port_status = %q, want listening", got)
	}
	if got, _ := diag["health_check"].(string); got != "failed" {
		t.Fatalf("health_check = %q, want failed", got)
	}
	if got, _ := diag["diagnosis"].(string); got != "Gasoline process exists but not responding" {
		t.Fatalf("diagnosis = %q, want hung gasoline diagnosis", got)
	}
}
