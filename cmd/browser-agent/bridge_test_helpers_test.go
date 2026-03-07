// bridge_test_helpers_test.go — Shared test helpers for bridge tests.
// Why: Isolates reusable stdin/stdout redirection helpers used across bridge test files.

package main

import (
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// withTestStdin temporarily replaces os.Stdin with a pipe containing input,
// runs fn, then restores the original stdin.
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

// waitForProcessExit waits for a child process to exit or kills it after timeout.
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
