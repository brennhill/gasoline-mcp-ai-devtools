// Purpose: Owns recording_manager.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// recording_manager.go — Capture delegation methods for recording subsystem.
// All recording logic lives in internal/recording. These thin methods preserve
// the external Capture API by delegating to c.rec.
package capture

import (
	"github.com/dev-console/dev-console/internal/recording"
)

// NewRecordingManager creates a RecordingManager with initialized state.
// Re-exported for backward compatibility with tests that call it directly.
var NewRecordingManager = recording.NewRecordingManager

// ============================================================================
// Capture delegation methods — preserve external API.
// ============================================================================

// StartRecording delegates to RecordingManager.
func (c *Capture) StartRecording(name, pageURL string, sensitiveDataEnabled bool) (string, error) {
	return c.rec.StartRecording(name, pageURL, sensitiveDataEnabled)
}

// StopRecording delegates to RecordingManager.
func (c *Capture) StopRecording(recordingID string) (int, int64, error) {
	return c.rec.StopRecording(recordingID)
}

// AddRecordingAction delegates to RecordingManager.
func (c *Capture) AddRecordingAction(action RecordingAction) error {
	return c.rec.AddRecordingAction(action)
}

// ListRecordings delegates to RecordingManager.
func (c *Capture) ListRecordings(limit int) ([]Recording, error) {
	return c.rec.ListRecordings(limit)
}

// GetRecording delegates to RecordingManager.
func (c *Capture) GetRecording(recordingID string) (*Recording, error) {
	return c.rec.GetRecording(recordingID)
}

// StartPlayback delegates to RecordingManager.
func (c *Capture) StartPlayback(recordingID string) (*PlaybackSession, error) {
	return c.rec.StartPlayback(recordingID)
}

// ExecutePlayback delegates to RecordingManager.
func (c *Capture) ExecutePlayback(recordingID string) (*PlaybackSession, error) {
	return c.rec.ExecutePlayback(recordingID)
}

// DetectFragileSelectors delegates to RecordingManager.
func (c *Capture) DetectFragileSelectors(sessions []*PlaybackSession) map[string]bool {
	return c.rec.DetectFragileSelectors(sessions)
}

// GetPlaybackStatus delegates to RecordingManager.
func (c *Capture) GetPlaybackStatus(session *PlaybackSession) map[string]any {
	return c.rec.GetPlaybackStatus(session)
}

// DiffRecordings delegates to RecordingManager.
func (c *Capture) DiffRecordings(originalID, replayID string) (*LogDiffResult, error) {
	return c.rec.DiffRecordings(originalID, replayID)
}

// CategorizeActionTypes delegates to RecordingManager.
func (c *Capture) CategorizeActionTypes(recording *Recording) map[string]int {
	return c.rec.CategorizeActionTypes(recording)
}

// GetStorageInfo delegates to RecordingManager.
func (c *Capture) GetStorageInfo() (StorageInfo, error) {
	return c.rec.GetStorageInfo()
}

// DeleteRecording delegates to RecordingManager.
func (c *Capture) DeleteRecording(recordingID string) error {
	return c.rec.DeleteRecording(recordingID)
}

// RecalculateStorageUsed delegates to RecordingManager.
func (c *Capture) RecalculateStorageUsed() error {
	return c.rec.RecalculateStorageUsed()
}
