package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

func TestBridgeStartupLock_SingleLeaderElection(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7890

	lockA, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		t.Fatalf("tryAcquireBridgeStartupLock() error = %v", err)
	}
	if !acquired || lockA == nil {
		t.Fatal("first lock acquisition should succeed")
	}

	lockB, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		t.Fatalf("second tryAcquireBridgeStartupLock() error = %v", err)
	}
	if acquired || lockB != nil {
		t.Fatal("second lock acquisition should not succeed while first leader holds lock")
	}

	lockA.release()

	lockC, acquired, err := tryAcquireBridgeStartupLock(port)
	if err != nil {
		t.Fatalf("third tryAcquireBridgeStartupLock() error = %v", err)
	}
	if !acquired || lockC == nil {
		t.Fatal("third lock acquisition should succeed after release")
	}
	lockC.release()
}

func TestClearStaleBridgeStartupLock_RemovesDeadOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7891
	path := writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       -1,
		Port:      port,
		CreatedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano),
	})

	if removed := clearStaleBridgeStartupLock(port, daemonStartupLockStaleAfter); !removed {
		t.Fatal("clearStaleBridgeStartupLock() = false, want true for dead owner")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file should be removed, stat err = %v", err)
	}
}

func TestClearStaleBridgeStartupLock_PreservesRecentLiveOwner(t *testing.T) {
	t.Setenv(statecfg.StateDirEnv, t.TempDir())
	port := 7892
	path := writeBridgeStartupLockForTest(t, port, bridgeStartupLockRecord{
		PID:       os.Getpid(),
		Port:      port,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})

	if removed := clearStaleBridgeStartupLock(port, time.Minute); removed {
		t.Fatal("clearStaleBridgeStartupLock() = true, want false for recent live owner")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file should remain, stat err = %v", err)
	}
}

func writeBridgeStartupLockForTest(t *testing.T, port int, record bridgeStartupLockRecord) string {
	t.Helper()
	path, err := bridgeStartupLockPath(port)
	if err != nil {
		t.Fatalf("bridgeStartupLockPath() error = %v", err)
	}
	// #nosec G301 -- test-owned temp directory.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	// #nosec G306 -- test fixture file content.
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}
