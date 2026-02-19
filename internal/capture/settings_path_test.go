package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/state"
)

func TestGetSettingsPathUsesStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	path, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath() error = %v", err)
	}

	want := filepath.Join(stateRoot, "settings", "extension-settings.json")
	if path != want {
		t.Fatalf("getSettingsPath() = %q, want %q", path, want)
	}
}

func TestLoadSettingsFromDiskFallsBackToLegacyPath(t *testing.T) {
	stateRoot := t.TempDir()
	homeDir := t.TempDir()

	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	legacyPath, err := state.LegacySettingsFile()
	if err != nil {
		t.Fatalf("LegacySettingsFile() error = %v", err)
	}

	payload := PersistedSettings{
		AIWebPilotEnabled: boolPtr(true),
		Timestamp:         time.Now(),
		ExtSessionID:      "legacy-session",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", legacyPath, err)
	}

	c := NewCapture()
	c.LoadSettingsFromDisk()

	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.ext.pilotEnabled {
		t.Fatalf("pilotEnabled = false, want true from legacy settings")
	}
}

func TestSaveSettingsToDiskWritesToStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	c := NewCapture()
	now := time.Now().UTC()

	c.mu.Lock()
	c.ext.pilotEnabled = true
	c.ext.pilotUpdatedAt = now
	c.ext.extSessionID = "session-123"
	c.mu.Unlock()

	if err := c.SaveSettingsToDisk(); err != nil {
		t.Fatalf("SaveSettingsToDisk() error = %v", err)
	}

	path, err := getSettingsPath()
	if err != nil {
		t.Fatalf("getSettingsPath() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}

	var persisted PersistedSettings
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if persisted.AIWebPilotEnabled == nil || !*persisted.AIWebPilotEnabled {
		t.Fatalf("AIWebPilotEnabled = %v, want true", persisted.AIWebPilotEnabled)
	}
	if persisted.ExtSessionID != "session-123" {
		t.Fatalf("SessionID = %q, want %q", persisted.ExtSessionID, "session-123")
	}
}

func boolPtr(v bool) *bool {
	return &v
}
