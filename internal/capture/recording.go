package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	recordingtypes "github.com/dev-console/dev-console/internal/recording"
)

// ============================================================================
// Constants
// ============================================================================

const (
	recordingStorageMax    = 1024 * 1024 * 1024 // 1GB max storage
	recordingWarningLevel  = 800 * 1024 * 1024  // 800MB warning threshold (80%)
	recordingBaseDir       = ".gasoline/recordings"
	recordingMetadataFile  = "metadata.json"
)

// ============================================================================
// Recording Lifecycle Methods
// ============================================================================

// StartRecording starts a new recording session
// Returns recording_id and error status
func (c *Capture) StartRecording(name string, pageURL string, sensitiveDataEnabled bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already recording
	if c.activeRecordingID != "" {
		return "", fmt.Errorf("already_recording: A recording is already active (id: %s)", c.activeRecordingID)
	}

	// Check storage quota
	if c.recordingStorageUsed >= recordingStorageMax {
		return "", fmt.Errorf("recording_storage_full: Recording storage at capacity (1GB). Please delete old recordings.")
	}

	// Warn if approaching limit (80%) - goes to stderr, not stdout (MCP stdio silence)
	if c.recordingStorageUsed >= recordingWarningLevel {
		fmt.Fprintf(os.Stderr, "[WARNING] recording_storage_warning: Recording storage at 80%% (%d bytes / %d bytes)\n",
			c.recordingStorageUsed, recordingStorageMax)
	}

	// Generate recording ID: name-YYYYMMDDTHHMMSSZ
	now := time.Now()
	timestamp := now.Format("20060102T150405Z")
	var recordingID string
	if name != "" {
		recordingID = fmt.Sprintf("%s-%s", name, timestamp)
	} else {
		// Auto-name from page title or URL
		recordingID = fmt.Sprintf("recording-%s", timestamp)
	}

	// Create recording in memory
	recording := &Recording{
		ID:                   recordingID,
		Name:                 name,
		CreatedAt:            now.Format(time.RFC3339),
		StartURL:             pageURL,
		Actions:              make([]RecordingAction, 0),
		SensitiveDataEnabled: sensitiveDataEnabled,
		TestID:               "", // Can be set later
	}

	// Try to get viewport from the last EnhancedAction (hack but works for now)
	// In reality, this would come from the extension
	recording.Viewport = recordingtypes.ViewportInfo{Width: 1920, Height: 1080}

	// Store in memory
	c.recordings[recordingID] = recording
	c.activeRecordingID = recordingID

	return recordingID, nil
}

// StopRecording stops the current recording and persists it to disk
// Returns action count and duration
func (c *Capture) StopRecording(recordingID string) (int, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Validate recording exists
	recording, exists := c.recordings[recordingID]
	if !exists {
		return 0, 0, fmt.Errorf("recording_not_found: No active recording with id: %s", recordingID)
	}

	// Calculate duration
	startTime, _ := time.Parse(time.RFC3339, recording.CreatedAt)
	duration := time.Since(startTime).Milliseconds()
	recording.Duration = duration

	// Count actions
	actionCount := len(recording.Actions)
	recording.ActionCount = actionCount

	// Persist to disk
	err := c.persistRecordingToDisk(recording)
	if err != nil {
		return 0, 0, fmt.Errorf("recording_save_failed: Failed to save recording: %v", err)
	}

	// Update storage used
	c.recordingStorageUsed += calculateRecordingSize(recording)

	// Clear active recording
	if c.activeRecordingID == recordingID {
		c.activeRecordingID = ""
	}

	return actionCount, duration, nil
}

// AddRecordingAction adds an action to the current recording
func (c *Capture) AddRecordingAction(action RecordingAction) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeRecordingID == "" {
		return fmt.Errorf("not_recording: No active recording")
	}

	recording := c.recordings[c.activeRecordingID]
	if recording == nil {
		return fmt.Errorf("recording_missing: Active recording not found")
	}

	// Redact sensitive data if needed
	if !recording.SensitiveDataEnabled {
		// Redact text on type actions
		if action.Type == "type" && action.Text != "" {
			action.Text = "[redacted]"
		}
	}

	// Set timestamp if not provided
	if action.TimestampMs == 0 {
		action.TimestampMs = time.Now().UnixMilli()
	}

	recording.Actions = append(recording.Actions, action)
	return nil
}

// ============================================================================
// Persistence
// ============================================================================

// persistRecordingToDisk writes recording metadata.json to ~/.gasoline/recordings/{id}/
func (c *Capture) persistRecordingToDisk(recording *Recording) error {
	// Determine storage directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot_find_home: %v", err)
	}

	recordingDir := filepath.Join(homeDir, recordingBaseDir, recording.ID)

	// Create directory
	err = os.MkdirAll(recordingDir, 0755)
	if err != nil {
		return fmt.Errorf("mkdir_failed: %v", err)
	}

	// Create metadata
	metadata := &recordingtypes.RecordingMetadata{
		ID:                   recording.ID,
		Name:                 recording.Name,
		CreatedAt:            recording.CreatedAt,
		StartURL:             recording.StartURL,
		Viewport:             recording.Viewport,
		Duration:             recording.Duration,
		ActionCount:          recording.ActionCount,
		Actions:              recording.Actions,
		SensitiveDataEnabled: recording.SensitiveDataEnabled,
		TestID:               recording.TestID,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("json_marshal_failed: %v", err)
	}

	// Write file
	metadataPath := filepath.Join(recordingDir, recordingMetadataFile)
	err = os.WriteFile(metadataPath, data, 0644)
	if err != nil {
		return fmt.Errorf("write_file_failed: %v", err)
	}

	return nil
}

// ============================================================================
// Querying
// ============================================================================

// ListRecordings returns all saved recordings from disk
func (c *Capture) ListRecordings(limit int) ([]Recording, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot_find_home: %v", err)
	}

	recordingsDir := filepath.Join(homeDir, recordingBaseDir)

	// Check if directory exists
	if _, err := os.Stat(recordingsDir); os.IsNotExist(err) {
		return []Recording{}, nil // No recordings yet
	}

	entries, err := os.ReadDir(recordingsDir)
	if err != nil {
		return nil, fmt.Errorf("readdir_failed: %v", err)
	}

	recordings := make([]Recording, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Try to load metadata
		recording, err := c.loadRecordingFromDisk(entry.Name())
		if err != nil {
			// Skip broken recordings
			continue
		}

		recordings = append(recordings, *recording)

		// Respect limit
		if limit > 0 && len(recordings) >= limit {
			break
		}
	}

	// Sort by created_at (newest first)
	sort.Slice(recordings, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, recordings[i].CreatedAt)
		t2, _ := time.Parse(time.RFC3339, recordings[j].CreatedAt)
		return t2.Before(t1)
	})

	return recordings, nil
}

// GetRecording loads a specific recording by ID
func (c *Capture) GetRecording(recordingID string) (*Recording, error) {
	return c.loadRecordingFromDisk(recordingID)
}

// loadRecordingFromDisk reads metadata.json and returns the Recording
func (c *Capture) loadRecordingFromDisk(recordingID string) (*Recording, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot_find_home: %v", err)
	}

	metadataPath := filepath.Join(homeDir, recordingBaseDir, recordingID, recordingMetadataFile)

	// Read file
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read_failed: %v", err)
	}

	// Unmarshal metadata
	metadata := &recordingtypes.RecordingMetadata{}
	err = json.Unmarshal(data, metadata)
	if err != nil {
		return nil, fmt.Errorf("json_unmarshal_failed: %v", err)
	}

	// Convert to Recording
	recording := &Recording{
		ID:                   metadata.ID,
		Name:                 metadata.Name,
		CreatedAt:            metadata.CreatedAt,
		StartURL:             metadata.StartURL,
		Viewport:             metadata.Viewport,
		Duration:             metadata.Duration,
		ActionCount:          metadata.ActionCount,
		Actions:              metadata.Actions,
		SensitiveDataEnabled: metadata.SensitiveDataEnabled,
		TestID:               metadata.TestID,
	}

	return recording, nil
}

// ============================================================================
// Helpers
// ============================================================================

// calculateRecordingSize estimates the size of a recording in bytes
func calculateRecordingSize(recording *Recording) int64 {
	// Rough estimate: metadata (JSON overhead) + actions
	// Each action: ~200 bytes (type, timestamps, selectors, text)
	size := int64(len(recording.Name) + len(recording.StartURL) + len(recording.TestID) + 500)
	size += int64(len(recording.Actions)) * 200
	return size
}
