// Purpose: Persists and reads daemon binary upgrade markers across restarts.
// Why: Isolates marker file I/O from watcher state and version-check logic.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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

	// Always remove the file, even if JSON is invalid.
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
