// Purpose: Coordinates bridge startup leadership so only one bridge actively
// spawns the daemon under contention.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	statecfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

type bridgeStartupLockRecord struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	Version   string `json:"version,omitempty"`
	CreatedAt string `json:"created_at"`
}

type bridgeStartupLock struct {
	path string
	pid  int
}

func bridgeStartupLockPath(port int) (string, error) {
	return statecfg.InRoot("run", fmt.Sprintf("bridge-startup-%d.lock.json", port))
}

func tryAcquireBridgeStartupLock(port int) (*bridgeStartupLock, bool, error) {
	path, err := bridgeStartupLockPath(port)
	if err != nil {
		return nil, false, err
	}
	// #nosec G301 -- runtime state directory for bridge coordination.
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, false, err
	}

	record := bridgeStartupLockRecord{
		PID:       os.Getpid(),
		Port:      port,
		Version:   version,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return nil, false, err
	}

	// #nosec G304 -- deterministic lock file path rooted in runtime state dir.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if _, err := f.Write(payload); err != nil {
		_ = f.Close()       //nolint:errcheck // best-effort cleanup on write failure
		_ = os.Remove(path) //nolint:errcheck // best-effort cleanup on write failure
		return nil, false, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path) //nolint:errcheck // best-effort cleanup on close failure
		return nil, false, err
	}
	return &bridgeStartupLock{path: path, pid: os.Getpid()}, true, nil
}

func (l *bridgeStartupLock) release() {
	if l == nil || l.path == "" {
		return
	}
	rec, err := readBridgeStartupLockRecord(l.path)
	if err == nil && rec != nil && rec.PID != l.pid {
		return
	}
	_ = os.Remove(l.path) //nolint:errcheck // best-effort ownership release
}

func readBridgeStartupLockRecord(path string) (*bridgeStartupLockRecord, error) {
	// #nosec G304 -- deterministic lock file path rooted in runtime state dir.
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rec bridgeStartupLockRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func clearStaleBridgeStartupLock(port int, staleAfter time.Duration) bool {
	path, err := bridgeStartupLockPath(port)
	if err != nil {
		return false
	}
	rec, err := readBridgeStartupLockRecord(path)
	if err != nil {
		_ = os.Remove(path) //nolint:errcheck // best-effort stale lock cleanup
		return true
	}
	if rec == nil {
		return false
	}

	if rec.PID <= 0 || !isProcessAlive(rec.PID) {
		_ = os.Remove(path) //nolint:errcheck // best-effort stale lock cleanup
		return true
	}

	createdAt, err := parseBridgeStartupLockTime(rec.CreatedAt)
	if err != nil {
		_ = os.Remove(path) //nolint:errcheck // best-effort stale lock cleanup
		return true
	}
	if staleAfter > 0 && time.Since(createdAt) > staleAfter {
		_ = os.Remove(path) //nolint:errcheck // best-effort stale lock cleanup
		return true
	}
	return false
}

func parseBridgeStartupLockTime(raw string) (time.Time, error) {
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts, nil
	}
	return time.Parse(time.RFC3339, raw)
}
