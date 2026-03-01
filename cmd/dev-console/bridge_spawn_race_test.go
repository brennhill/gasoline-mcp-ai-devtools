// bridge_spawn_race_test.go — Tests for tryConnectToExisting and waitForPeerDaemon helpers.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- tryConnectToExisting tests ---

func TestTryConnectToExisting_NoServer(t *testing.T) {
	t.Parallel()
	state := &daemonState{
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	// Use an ephemeral port that nothing is listening on.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	got := tryConnectToExisting(state, port)
	if got {
		t.Fatal("tryConnectToExisting() = true, want false when no server running")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		t.Fatal("state.ready should be false")
	}
	if state.failed {
		t.Fatal("state.failed should be false")
	}
}

func TestTryConnectToExisting_CompatibleServer(t *testing.T) {
	t.Parallel()
	ln, port := startHealthServer(t, http.StatusOK, healthJSON(version, "gasoline"))
	defer ln.Close()

	state := &daemonState{
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	got := tryConnectToExisting(state, port)
	if !got {
		t.Fatal("tryConnectToExisting() = false, want true for compatible server")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.ready {
		t.Fatal("state.ready should be true")
	}
}

func TestTryConnectToExisting_NonGasolineService(t *testing.T) {
	t.Parallel()
	ln, port := startHealthServer(t, http.StatusOK, healthJSON("1.0.0", "some-other-service"))
	defer ln.Close()

	state := &daemonState{
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	got := tryConnectToExisting(state, port)
	if !got {
		t.Fatal("tryConnectToExisting() = false, want true (fatally blocked)")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.ready {
		t.Fatal("state.ready should be false when non-gasoline service occupies port")
	}
	if !state.failed {
		t.Fatal("state.failed should be true when non-gasoline service occupies port")
	}
	if !strings.Contains(state.err, "non-gasoline") {
		t.Fatalf("state.err = %q, want mention of non-gasoline", state.err)
	}
}

// --- waitForPeerDaemon tests ---

func TestWaitForPeerDaemon_ServerAppearsOnFirstRetry(t *testing.T) {
	t.Parallel()
	// Start server after a short delay (< 500ms) so the first retry finds it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// Launch server after 200ms (before 500ms first retry completes).
	go func() {
		time.Sleep(200 * time.Millisecond)
		startHealthServerOnPort(t, port, http.StatusOK, healthJSON(version, "gasoline"))
	}()

	state := &daemonState{
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	start := time.Now()
	got := waitForPeerDaemon(state, port)
	elapsed := time.Since(start)

	if !got {
		t.Fatal("waitForPeerDaemon() = false, want true when server appears during retry")
	}
	// Should have waited at least 500ms (the first sleep)
	if elapsed < 400*time.Millisecond {
		t.Fatalf("elapsed = %v, want >= 400ms (first retry sleep)", elapsed)
	}
	// Should NOT have needed the second 2s wait
	if elapsed > 2*time.Second {
		t.Fatalf("elapsed = %v, want < 2s (should find on first retry)", elapsed)
	}
}

func TestWaitForPeerDaemon_NoServerReturnsQuickly(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	state := &daemonState{
		readyCh:  make(chan struct{}),
		failedCh: make(chan struct{}),
	}
	start := time.Now()
	got := waitForPeerDaemon(state, port)
	elapsed := time.Since(start)

	if got {
		t.Fatal("waitForPeerDaemon() = true, want false when no server ever appears")
	}
	// Should have taken ~2.5s total (500ms + 2s retries)
	if elapsed < 2*time.Second {
		t.Fatalf("elapsed = %v, want >= 2s (both retries)", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("elapsed = %v, want < 5s", elapsed)
	}
}

// --- runBridgeMode integration: existing test still passes (no regression) ---

func TestRunBridgeModeWithExistingServer_StillWorks(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(:0) error = %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`+"\n")
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	output := captureStdout(t, func() {
		withTestStdin(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`+"\n", func() {
			runBridgeMode(port, "", 0)
		})
	})
	if !strings.Contains(output, `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("runBridgeMode output missing initialize response: %q", output)
	}
}

// --- helpers ---

func healthJSON(ver, service string) string {
	m := map[string]any{
		"status":       "ok",
		"version":      ver,
		"service-name": service,
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func startHealthServer(t *testing.T, status int, body string) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln, port
}

func startHealthServerOnPort(t *testing.T, port, status int, body string) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		// Port may already be in use; return nil.
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return ln
}
