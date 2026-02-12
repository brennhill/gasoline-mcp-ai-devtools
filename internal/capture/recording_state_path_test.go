package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	recordingtypes "github.com/dev-console/dev-console/internal/recording"
	"github.com/dev-console/dev-console/internal/state"
)

func TestListRecordingsReadsLegacyDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	recordingID := "legacy-recording-123"
	writeLegacyRecording(t, recordingID)

	manager := NewRecordingManager()
	recordings, err := manager.ListRecordings(10)
	if err != nil {
		t.Fatalf("ListRecordings() error = %v", err)
	}
	if len(recordings) == 0 {
		t.Fatalf("ListRecordings() returned no recordings; expected legacy recording")
	}

	found := false
	for _, rec := range recordings {
		if rec.ID == recordingID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ListRecordings() did not include legacy recording %q", recordingID)
	}
}

func TestPersistRecordingWritesToStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	manager := NewRecordingManager()
	recordingID, err := manager.StartRecording("state-storage", "https://example.com", false)
	if err != nil {
		t.Fatalf("StartRecording() error = %v", err)
	}

	if _, _, err := manager.StopRecording(recordingID); err != nil {
		t.Fatalf("StopRecording() error = %v", err)
	}

	stateDir, err := state.RecordingsDir()
	if err != nil {
		t.Fatalf("state.RecordingsDir() error = %v", err)
	}
	stateMetadata := filepath.Join(stateDir, recordingID, "metadata.json")
	if _, err := os.Stat(stateMetadata); err != nil {
		t.Fatalf("expected metadata in state directory at %q: %v", stateMetadata, err)
	}

	legacyDir, err := state.LegacyRecordingsDir()
	if err != nil {
		t.Fatalf("state.LegacyRecordingsDir() error = %v", err)
	}
	legacyMetadata := filepath.Join(legacyDir, recordingID, "metadata.json")
	if _, err := os.Stat(legacyMetadata); err == nil {
		t.Fatalf("expected no legacy metadata at %q, but file exists", legacyMetadata)
	}
}

func writeLegacyRecording(t *testing.T, id string) {
	t.Helper()

	legacyDir, err := state.LegacyRecordingsDir()
	if err != nil {
		t.Fatalf("state.LegacyRecordingsDir() error = %v", err)
	}
	recordingDir := filepath.Join(legacyDir, id)
	if err := os.MkdirAll(recordingDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", recordingDir, err)
	}

	meta := recordingtypes.RecordingMetadata{
		ID:          id,
		Name:        "legacy",
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		StartURL:    "https://example.com",
		Duration:    10,
		ActionCount: 1,
		Actions: []recordingtypes.RecordingAction{
			{Type: "click", Selector: "#btn", TimestampMs: time.Now().UnixMilli()},
		},
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(recordingDir, "metadata.json"), data, 0o600); err != nil {
		t.Fatalf("os.WriteFile(metadata.json) error = %v", err)
	}
}
