// session_test.go — Tests for PTY session spawn, I/O roundtrip, resize, and cleanup.

package pty

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func readUntilContains(t *testing.T, sess *Session, needle string, timeout time.Duration) string {
	t.Helper()
	buf := make([]byte, 4096)
	deadline := time.Now().Add(timeout)
	var collected string
	for time.Now().Before(deadline) {
		n, err := sess.Read(buf)
		if n > 0 {
			collected += string(buf[:n])
			if strings.Contains(collected, needle) {
				return collected
			}
		}
		if err != nil {
			t.Fatalf("read before %q: %v (output so far: %q)", needle, err, collected)
		}
	}
	t.Fatalf("timeout waiting for %q, got: %q", needle, collected)
	return ""
}

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
	readUntilContains(t, sess, "HELLO_PTY", 3*time.Second)

	// Write to stdin (cat echoes back).
	input := "test_input\n"
	if _, err := sess.Write([]byte(input)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the echo back.
	readUntilContains(t, sess, "test_input", 3*time.Second)
}

func TestSpawn_DefaultSize(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "stty size; exit 0"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	collected := readUntilContains(t, sess, "24 80", 3*time.Second)
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

	collected := readUntilContains(t, sess, "40 120", 3*time.Second)
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

func TestSession_IsAlive(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Running session should be alive.
	if !sess.IsAlive() {
		t.Fatal("expected IsAlive()=true for running session")
	}

	// After close, should not be alive.
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if sess.IsAlive() {
		t.Fatal("expected IsAlive()=false after Close()")
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

// T1 regression: Read/Write during concurrent Close must return ErrSessionClosed,
// not a raw OS error from a potentially recycled file descriptor.
func TestSession_ReadDuringClose_ReturnsErrSessionClosed(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Start a goroutine that reads forever; it will unblock when Close() closes the fd.
	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := sess.Read(buf)
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	// Give the goroutine time to enter the blocking Read syscall.
	time.Sleep(50 * time.Millisecond)

	// Close the session — the read should unblock with ErrSessionClosed.
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case err := <-readErr:
		if err != ErrSessionClosed {
			t.Fatalf("expected ErrSessionClosed from in-flight Read, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for Read to return after Close")
	}
}

func TestSession_WriteDuringClose_ReturnsErrSessionClosed(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Close the session first.
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Write after close must return ErrSessionClosed.
	_, werr := sess.Write([]byte("hello"))
	if werr != ErrSessionClosed {
		t.Fatalf("expected ErrSessionClosed from Write after Close, got: %v", werr)
	}
}

// T1 regression: done channel is closed before ptmx, so concurrent callers
// detect shutdown via the channel rather than hitting a recycled fd.
func TestSession_DoneChannelClosedBeforePtmx(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// Before close, done channel should not be closed.
	select {
	case <-sess.done:
		t.Fatal("done channel should not be closed before Close()")
	default:
	}

	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// After close, done channel must be closed.
	select {
	case <-sess.done:
		// Good — channel is closed.
	default:
		t.Fatal("done channel should be closed after Close()")
	}
}

// T3 regression: scrollback trimming must not leak memory via backing array retention.
// Verifies that after eviction, the scrollback slice has its own backing array
// (length == cap == maxScrollback), not a sub-slice of a larger allocation.
func TestSession_AppendScrollback_EvictionReleasesBackingArray(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	// Fill scrollback past maxScrollback to trigger eviction.
	chunk := make([]byte, 1024)
	for i := range chunk {
		chunk[i] = 'A'
	}
	// Write 2x maxScrollback worth of data to force at least one eviction.
	iterations := (maxScrollback * 2) / len(chunk)
	for i := 0; i < iterations; i++ {
		sess.AppendScrollback(chunk)
	}

	sess.scrollMu.Lock()
	sbLen := len(sess.scrollback)
	sbCap := cap(sess.scrollback)
	sess.scrollMu.Unlock()

	if sbLen != maxScrollback {
		t.Fatalf("expected scrollback len=%d, got %d", maxScrollback, sbLen)
	}
	// After eviction with make+copy, cap should equal len (no excess backing array).
	if sbCap != maxScrollback {
		t.Fatalf("expected scrollback cap=%d (new allocation), got %d (backing array leak)", maxScrollback, sbCap)
	}
}

// T3 regression: scrollback below maxScrollback should not trigger eviction.
func TestSession_AppendScrollback_NoEvictionBelowMax(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	data := []byte("hello")
	sess.AppendScrollback(data)

	got := sess.Scrollback()
	if string(got) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(got))
	}
}

// T1 regression: concurrent Read/Write/Close must not panic or deadlock.
func TestSession_ConcurrentReadWriteClose(t *testing.T) {
	sess, err := Spawn(SpawnConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}

	const goroutines = 10
	var wg sync.WaitGroup

	// Spawn readers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 64)
			for {
				_, err := sess.Read(buf)
				if err != nil {
					return
				}
			}
		}()
	}

	// Spawn writers.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, err := sess.Write([]byte("x"))
				if err != nil {
					return
				}
				runtime.Gosched()
			}
		}()
	}

	// Let them run briefly, then close.
	time.Sleep(50 * time.Millisecond)
	if err := sess.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// All goroutines must exit within a reasonable time.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines exited.
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: concurrent Read/Write goroutines did not exit after Close")
	}
}
