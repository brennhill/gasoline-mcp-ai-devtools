package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestPortKillHint(t *testing.T) {
	t.Parallel()

	hint := portKillHint(7890)
	if hint == "" {
		t.Fatal("portKillHint returned empty string")
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(hint, "netstat") {
			t.Errorf("Windows hint should contain netstat, got: %s", hint)
		}
		if !strings.Contains(hint, "taskkill") {
			t.Errorf("Windows hint should contain taskkill, got: %s", hint)
		}
		if !strings.Contains(hint, "7890") {
			t.Errorf("Windows hint should contain port number, got: %s", hint)
		}
	default:
		if !strings.Contains(hint, "lsof") {
			t.Errorf("Unix hint should contain lsof, got: %s", hint)
		}
		if !strings.Contains(hint, "7890") {
			t.Errorf("Unix hint should contain port number, got: %s", hint)
		}
	}
}

func TestPortKillHintForce(t *testing.T) {
	t.Parallel()

	hint := portKillHintForce(7890)
	if hint == "" {
		t.Fatal("portKillHintForce returned empty string")
	}

	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(hint, "taskkill") {
			t.Errorf("Windows force hint should contain taskkill, got: %s", hint)
		}
	default:
		if !strings.Contains(hint, "kill -9") {
			t.Errorf("Unix force hint should contain kill -9, got: %s", hint)
		}
		if !strings.Contains(hint, "7890") {
			t.Errorf("Unix force hint should contain port number, got: %s", hint)
		}
	}
}

func TestFindProcessOnPort(t *testing.T) {
	t.Parallel()

	// findProcessOnPort should not panic on any platform
	pids, err := findProcessOnPort(0)
	// Port 0 is unlikely to have a process; we just verify no panic
	if err != nil {
		t.Logf("findProcessOnPort(0) returned error (expected): %v", err)
	}
	_ = pids // may be empty
}

func TestGetProcessCommand(t *testing.T) {
	t.Parallel()

	// getProcessCommand should not panic for an invalid PID
	cmd := getProcessCommand(999999)
	// Should return empty or some value, but not panic
	_ = cmd
}

func TestKillProcessByPID(t *testing.T) {
	t.Parallel()

	// killProcessByPID should not panic for an invalid PID
	// It should gracefully handle the error
	err := killProcessByPID(999999)
	// We expect an error since process doesn't exist
	if err == nil {
		t.Log("killProcessByPID(999999) returned nil (process may exist)")
	}
}
