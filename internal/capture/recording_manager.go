// recording_manager.go — Recording lifecycle, persistence, and state management.
// Extracted from the Capture god object. Owns its own sync.Mutex,
// independent of Capture.mu. Zero cross-cutting dependencies.
package capture

import (
	"sync"

	"github.com/dev-console/dev-console/internal/recording"
)

// RecordingManager manages recording lifecycle, persistence, and storage tracking.
// Owns its own sync.Mutex — independent of Capture.mu.
type RecordingManager struct {
	mu                   sync.Mutex
	activeRecordingID    string
	recordings           map[string]*recording.Recording
	recordingStorageUsed int64
}

// NewRecordingManager creates a RecordingManager with initialized state.
func NewRecordingManager() *RecordingManager {
	return &RecordingManager{
		recordings: make(map[string]*recording.Recording),
	}
}

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
