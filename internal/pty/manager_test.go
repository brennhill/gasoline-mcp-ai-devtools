// manager_test.go — Tests for PTY session manager lifecycle, auth tokens, and concurrent access.

package pty

import (
	"errors"
	"sync"
	"testing"
)

func TestManager_StartAndGet(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	result, err := m.Start(StartConfig{
		Cmd: "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if result.SessionID != "default" {
		t.Fatalf("expected session ID 'default', got: %s", result.SessionID)
	}
	if result.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if result.Pid <= 0 {
		t.Fatalf("expected positive PID, got: %d", result.Pid)
	}

	// Get by token.
	sess, err := m.GetByToken(result.Token)
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if sess.ID != "default" {
		t.Fatalf("expected session ID 'default', got: %s", sess.ID)
	}

	// Get by ID.
	sess2, err := m.Get("default")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if sess2 != sess {
		t.Fatal("expected same session instance")
	}
}

func TestManager_DuplicateSessionID(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	_, err := m.Start(StartConfig{
		ID:  "test",
		Cmd: "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("first start: %v", err)
	}

	_, err = m.Start(StartConfig{
		ID:  "test",
		Cmd: "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if !errors.Is(err, ErrSessionExists) {
		t.Fatalf("expected ErrSessionExists, got: %v", err)
	}
}

func TestManager_InvalidToken(t *testing.T) {
	m := NewManager()

	_, err := m.GetByToken("bogus")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got: %v", err)
	}
}

func TestManager_StopSession(t *testing.T) {
	m := NewManager()

	result, err := m.Start(StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := m.Stop("default"); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Token should be invalidated.
	_, err = m.GetByToken(result.Token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken after stop, got: %v", err)
	}

	// Session should be gone.
	_, err = m.Get("default")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound after stop, got: %v", err)
	}

	// Stop nonexistent.
	err = m.Stop("nonexistent")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestManager_StopAll(t *testing.T) {
	m := NewManager()

	for _, id := range []string{"a", "b", "c"} {
		_, err := m.Start(StartConfig{
			ID:   id,
			Cmd:  "/bin/sh",
			Args: []string{"-c", "exec cat"},
		})
		if err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}

	if m.Count() != 3 {
		t.Fatalf("expected 3 sessions, got: %d", m.Count())
	}

	m.StopAll()

	if m.Count() != 0 {
		t.Fatalf("expected 0 sessions after StopAll, got: %d", m.Count())
	}
}

func TestManager_List(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	ids := []string{"alpha", "beta"}
	for _, id := range ids {
		_, err := m.Start(StartConfig{
			ID:   id,
			Cmd:  "/bin/sh",
			Args: []string{"-c", "exec cat"},
		})
		if err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}

	listed := m.List()
	if len(listed) != 2 {
		t.Fatalf("expected 2, got: %d", len(listed))
	}

	// Check both IDs are present (order is map-dependent).
	found := make(map[string]bool)
	for _, id := range listed {
		found[id] = true
	}
	for _, id := range ids {
		if !found[id] {
			t.Fatalf("missing session ID: %s", id)
		}
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager()
	defer m.StopAll()

	// Start a session.
	result, err := m.Start(StartConfig{
		Cmd:  "/bin/sh",
		Args: []string{"-c", "exec cat"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.GetByToken(result.Token)
			_ = m.List()
			_ = m.Count()
		}()
	}
	wg.Wait()
}
