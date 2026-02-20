// binary_watcher.go — Watches the daemon binary on disk for upgrades and triggers auto-restart.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// Tunable intervals — overridden in tests.
var (
	binaryWatchInterval  = 30 * time.Second
	upgradeGracePeriod   = 5 * time.Second
	versionVerifyTimeout = 5 * time.Second
)

// getExecutablePath returns the path to the current binary. Overridable for tests.
var getExecutablePath = os.Executable

// BinaryWatcherState tracks the on-disk binary state for upgrade detection.
type BinaryWatcherState struct {
	mu              sync.Mutex
	execPath        string
	lastModTime     time.Time
	lastSize        int64
	upgradePending  bool
	detectedVersion string
	detectedAt      time.Time
}

// UpgradeInfo returns the current upgrade detection state (thread-safe).
func (s *BinaryWatcherState) UpgradeInfo() (pending bool, version string, detectedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upgradePending, s.detectedVersion, s.detectedAt
}

// binaryChanged checks if the binary at execPath has changed since the last check.
// The first call always returns false and caches the initial file state.
func (s *BinaryWatcherState) binaryChanged() (bool, error) {
	fi, err := os.Stat(s.execPath)
	if err != nil {
		return false, fmt.Errorf("stat binary: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	modTime := fi.ModTime()
	size := fi.Size()

	if s.lastModTime.IsZero() {
		// First call: cache initial state
		s.lastModTime = modTime
		s.lastSize = size
		return false, nil
	}

	if modTime != s.lastModTime || size != s.lastSize {
		s.lastModTime = modTime
		s.lastSize = size
		return true, nil
	}
	return false, nil
}

// checkForUpgrade verifies whether the binary reports a newer version than current.
// Returns true if an upgrade is detected and sets the upgrade-pending state.
func (s *BinaryWatcherState) checkForUpgrade(currentVersion string) bool {
	newVer, err := verifyBinaryVersion(s.execPath)
	if err != nil {
		return false
	}

	if !isNewerVersion(newVer, currentVersion) {
		return false
	}

	s.mu.Lock()
	s.upgradePending = true
	s.detectedVersion = newVer
	s.detectedAt = time.Now()
	s.mu.Unlock()
	return true
}

// verifyBinaryVersion executes the binary with --version and parses the output.
// Expects output like "gasoline v0.8.0" or just "0.8.0".
func verifyBinaryVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionVerifyTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = append(os.Environ(), "GASOLINE_VERSION_CHECK=1")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("exec --version: %w", err)
	}

	return parseVersionOutput(strings.TrimSpace(string(out)))
}

// parseVersionOutput extracts a version string from --version output.
// Handles "gasoline v0.8.0", "gasoline 0.8.0", "v0.8.0", and "0.8.0".
func parseVersionOutput(output string) (string, error) {
	// Try "gasoline v0.8.0" or "gasoline 0.8.0"
	if strings.HasPrefix(output, "gasoline ") {
		output = strings.TrimPrefix(output, "gasoline ")
	}
	output = strings.TrimPrefix(output, "v")

	parts := parseVersionParts(output)
	if parts == nil {
		return "", fmt.Errorf("invalid version output: %q", output)
	}
	return output, nil
}

// upgradeMarker is persisted to disk so the new daemon can report the completed upgrade.
type upgradeMarker struct {
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	Timestamp   string `json:"timestamp"`
}

// writeUpgradeMarker writes the upgrade marker file.
func writeUpgradeMarker(fromVersion, toVersion, path string) error {
	marker := upgradeMarker{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("marshal upgrade marker: %w", err)
	}
	if err := os.MkdirAll(parentDir(path), 0o755); err != nil {
		return fmt.Errorf("create marker dir: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// readAndClearUpgradeMarker reads the marker file and removes it.
// Returns nil if the file doesn't exist or contains invalid JSON.
func readAndClearUpgradeMarker(path string) (*upgradeMarker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read upgrade marker: %w", err)
	}

	// Always remove the file, even if JSON is invalid
	_ = os.Remove(path)

	var marker upgradeMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, nil // invalid JSON, treat as no marker
	}
	if marker.FromVersion == "" || marker.ToVersion == "" {
		return nil, nil
	}
	return &marker, nil
}

// parentDir returns the parent directory of a path.
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// startBinaryWatcher starts a background goroutine that watches the daemon binary for changes.
// Returns nil if auto-upgrade is disabled via GASOLINE_NO_AUTO_UPGRADE=1.
//
// Detection loop (every binaryWatchInterval):
//  1. Stat the executable, compare modtime+size
//  2. If changed: run --version, parse output
//  3. If newer: set upgrade_pending, call onUpgrade
//  4. After grace period: call triggerShutdown
func startBinaryWatcher(ctx context.Context, currentVersion string, onUpgrade func(string), triggerShutdown func()) *BinaryWatcherState {
	if os.Getenv("GASOLINE_NO_AUTO_UPGRADE") == "1" {
		return nil
	}

	execPath, err := getExecutablePath()
	if err != nil {
		return nil
	}

	state := &BinaryWatcherState{execPath: execPath}

	util.SafeGo(func() {
		// Cache initial binary state
		if _, err := state.binaryChanged(); err != nil {
			return
		}

		ticker := time.NewTicker(binaryWatchInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				changed, err := state.binaryChanged()
				if err != nil || !changed {
					continue
				}

				if !state.checkForUpgrade(currentVersion) {
					continue
				}

				_, newVer, _ := state.UpgradeInfo()
				onUpgrade(newVer)

				// Grace period before shutdown
				select {
				case <-time.After(upgradeGracePeriod):
					triggerShutdown()
					return
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	})

	return state
}
