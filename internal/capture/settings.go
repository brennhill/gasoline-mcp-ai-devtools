// Purpose: Owns settings.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// settings.go — Settings disk persistence for fast daemon restart.
package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

// PersistedSettings is the disk format for extension-settings.json.
type PersistedSettings struct {
	AIWebPilotEnabled *bool     `json:"ai_web_pilot_enabled,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	ExtSessionID      string    `json:"ext_session_id"`
}

// getSettingsPath returns the path to the settings cache file
func getSettingsPath() (string, error) {
	return state.SettingsFile()
}

func getLegacySettingsPath() (string, error) {
	return state.LegacySettingsFile()
}

// readSettingsData reads settings from the primary path, falling back to legacy.
func readSettingsData() ([]byte, error) {
	path, err := getSettingsPath()
	if err != nil {
		return nil, fmt.Errorf("could not determine settings path: %w", err)
	}

	// #nosec G304 -- path is resolved from trusted runtime state directory, not user input
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not read settings file: %w", err)
	}

	// Compatibility fallback for older locations.
	legacyPath, legacyErr := getLegacySettingsPath()
	if legacyErr != nil {
		return nil, nil // no legacy path available, not an error
	}
	// #nosec G304 -- legacy path is deterministic, not user input
	legacyData, readErr := os.ReadFile(legacyPath)
	if readErr != nil {
		if !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("could not read legacy settings file: %w", readErr)
		}
		return nil, nil
	}
	return legacyData, nil
}

// LoadSettingsFromDisk loads cached settings from runtime state storage.
func (c *Capture) LoadSettingsFromDisk() {
	data, err := readSettingsData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] %v\n", err)
		return
	}
	if data == nil {
		return
	}

	var settings PersistedSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Could not parse settings file: %v\n", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(settings.Timestamp) > 5*time.Second {
		return
	}
	if settings.AIWebPilotEnabled != nil {
		c.ext.pilotEnabled = *settings.AIWebPilotEnabled
		c.ext.pilotUpdatedAt = settings.Timestamp
	}
}

// SaveSettingsToDisk persists current settings to runtime state storage.
func (c *Capture) SaveSettingsToDisk() error {
	path, err := getSettingsPath()
	if err != nil {
		return err
	}

	c.mu.RLock()
	// Copy bool by value — &c.ext.pilotEnabled would escape the lock scope
	pilotEnabled := c.ext.pilotEnabled
	settings := PersistedSettings{
		AIWebPilotEnabled: &pilotEnabled,
		Timestamp:         c.ext.pilotUpdatedAt,
		ExtSessionID:      c.ext.extSessionID,
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	// Write atomically via temp file
	// #nosec G306 -- settings file is owner-only readable (0600) for privacy
	// #nosec G301 -- runtime state directory should be user-readable for diagnostics
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
