// Purpose: Re-exports recording-manager constructors and delegates recording lifecycle methods.
// Why: Preserves capture package API compatibility while recording logic lives in internal/recording.
// Docs: docs/features/feature/playback-engine/index.md
// Docs: docs/features/feature/tab-recording/index.md

package capture

import (
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/recording"
)

// NewRecordingManager creates a RecordingManager with initialized state.
// Re-exported for backward compatibility with tests that call it directly.
var NewRecordingManager = recording.NewRecordingManager

// ============================================================================
// Capture delegation methods — preserve external API.
// ============================================================================

// StartRecording delegates to RecordingManager.
func (c *Capture) StartRecording(name, pageURL string, sensitiveDataEnabled bool) (string, error) {
	return c.recordingManager.StartRecording(name, pageURL, sensitiveDataEnabled)
}

// StopRecording delegates to RecordingManager.
func (c *Capture) StopRecording(recordingID string) (int, int64, error) {
	return c.recordingManager.StopRecording(recordingID)
}

// AddRecordingAction delegates to RecordingManager.
func (c *Capture) AddRecordingAction(action RecordingAction) error {
	return c.recordingManager.AddRecordingAction(action)
}

// ListRecordings delegates to RecordingManager.
func (c *Capture) ListRecordings(limit int) ([]Recording, error) {
	return c.recordingManager.ListRecordings(limit)
}

// GetRecording delegates to RecordingManager.
func (c *Capture) GetRecording(recordingID string) (*Recording, error) {
	return c.recordingManager.GetRecording(recordingID)
}

// StartPlayback delegates to RecordingManager.
func (c *Capture) StartPlayback(recordingID string) (*PlaybackSession, error) {
	return c.recordingManager.StartPlayback(recordingID)
}

// ExecutePlayback delegates to RecordingManager.
func (c *Capture) ExecutePlayback(recordingID string) (*PlaybackSession, error) {
	return c.recordingManager.ExecutePlayback(recordingID)
}

// DetectFragileSelectors delegates to RecordingManager.
func (c *Capture) DetectFragileSelectors(sessions []*PlaybackSession) map[string]bool {
	return c.recordingManager.DetectFragileSelectors(sessions)
}

// GetPlaybackStatus delegates to RecordingManager.
func (c *Capture) GetPlaybackStatus(session *PlaybackSession) map[string]any {
	return c.recordingManager.GetPlaybackStatus(session)
}

// DiffRecordings delegates to RecordingManager.
func (c *Capture) DiffRecordings(originalID, replayID string) (*LogDiffResult, error) {
	return c.recordingManager.DiffRecordings(originalID, replayID)
}

// CategorizeActionTypes delegates to RecordingManager.
func (c *Capture) CategorizeActionTypes(recording *Recording) map[string]int {
	return c.recordingManager.CategorizeActionTypes(recording)
}

// GetStorageInfo delegates to RecordingManager.
func (c *Capture) GetStorageInfo() (StorageInfo, error) {
	return c.recordingManager.GetStorageInfo()
}

// DeleteRecording delegates to RecordingManager.
func (c *Capture) DeleteRecording(recordingID string) error {
	return c.recordingManager.DeleteRecording(recordingID)
}

// RecalculateStorageUsed delegates to RecordingManager.
func (c *Capture) RecalculateStorageUsed() error {
	return c.recordingManager.RecalculateStorageUsed()
}
