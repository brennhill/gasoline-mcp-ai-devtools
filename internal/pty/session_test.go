// session_test.go — Tests for PTY session spawn, I/O roundtrip, resize, and cleanup.

package pty

import (
	"strings"
	"testing"
	"time"
)

func TestSpawn_RequiresCmd(t *testing.T) {
	_, err := Spawn(SpawnConfig{})
	if err == nil {
		t.Fatal("expected error for empty cmd")
	}
	if !strings.Contains(err.Error(), "cmd is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpawn_InvalidCmd(t *testing.T) {
	_, err := Spawn(SpawnConfig{Cmd: "/nonexistent/binary"})
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
}

func TestSpawn_EchoRoundtrip(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "echo HELLO_PTY; exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	// Read output — should contain "HELLO_PTY".
	buf := make([]byte, 4096)
	deadline := time.After(3 * time.Second)
	var collected string
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for echo output, got: %q", collected)
		default:
		}
		n, err := sess.Read(buf)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		collected += string(buf[:n])
		if strings.Contains(collected, "HELLO_PTY") {
			break
		}
	}

	// Write to stdin (cat echoes back).
	input := "test_input\n"
	if _, err := sess.Write([]byte(input)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the echo back.
	deadline = time.After(3 * time.Second)
	collected = ""
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for echo back, got: %q", collected)
		default:
		}
		n, err := sess.Read(buf)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		collected += string(buf[:n])
		if strings.Contains(collected, "test_input") {
			break
		}
	}
}

func TestSpawn_DefaultSize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd: "/bin/sh",
		Args: []string{"-c", "stty size; exit 0"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	buf := make([]byte, 4096)
	deadline := time.After(3 * time.Second)
	var collected string
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout, got: %q", collected)
		default:
		}
		n, err := sess.Read(buf)
		if n > 0 {
			collected += string(buf[:n])
		}
		if err != nil || strings.Contains(collected, "24 80") {
			break
		}
	}
	if !strings.Contains(collected, "24 80") {
		t.Fatalf("expected '24 80' in output, got: %q", collected)
	}
}

func TestSpawn_CustomSize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "stty size; exit 0"},
		Cols: 120,
		Rows: 40,
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	buf := make([]byte, 4096)
	deadline := time.After(3 * time.Second)
	var collected string
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout, got: %q", collected)
		default:
		}
		n, err := sess.Read(buf)
		if n > 0 {
			collected += string(buf[:n])
		}
		if err != nil || strings.Contains(collected, "40 120") {
			break
		}
	}
	if !strings.Contains(collected, "40 120") {
		t.Fatalf("expected '40 120' in output, got: %q", collected)
	}
}

func TestSession_Resize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	if err := sess.Resize(132, 50); err != nil {
		t.Fatalf("resize: %v", err)
	}
}

func TestSession_Close(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Operations after close should return ErrSessionClosed.
	buf := make([]byte, 64)
	if _, err := sess.Read(buf); err != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed on Read, got: %v", err)
	}
	if _, err := sess.Write([]byte("x")); err != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed on Write, got: %v", err)
	}
	if err := sess.Resize(80, 24); err != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed on Resize, got: %v", err)
	}

	// Double close is safe.
	if err := sess.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestSession_Pid(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	pid := sess.Pid()
	if pid <= 0 {
		t.Fatalf("expected positive PID, got: %d", pid)
	}
}
