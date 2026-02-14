// recording.go — Recording lifecycle and persistence methods on RecordingManager.
// Handles start/stop/add-action lifecycle and disk I/O for recordings.
package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	recordingtypes "github.com/dev-console/dev-console/internal/recording"
	"github.com/dev-console/dev-console/internal/state"
)

// validateRecordingID rejects IDs containing path traversal sequences.
func validateRecordingID(id string) error {
	if id == "" {
		return fmt.Errorf("recording_id_empty: Recording ID must not be empty")
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("recording_id_invalid: Recording ID contains illegal characters")
	}
	// After cleaning, the ID must be a single path component
	if filepath.Base(id) != id {
		return fmt.Errorf("recording_id_invalid: Recording ID must be a single directory name")
	}
	return nil
}

// ============================================================================
// Constants
// ============================================================================

const (
	recordingStorageMax   = 1024 * 1024 * 1024 // 1GB max storage
	recordingWarningLevel = 800 * 1024 * 1024  // 800MB warning threshold (80%)
	recordingMetadataFile = "metadata.json"
)

// ============================================================================
// Storage Info Types
// ============================================================================

// StorageInfo provides information about recording storage usage.
type StorageInfo struct {
	UsedBytes      int64   `json:"used_bytes"`      // Current storage usage in bytes
	MaxBytes       int64   `json:"max_bytes"`       // Maximum storage limit in bytes
	WarningBytes   int64   `json:"warning_bytes"`   // Warning threshold in bytes
	UsedPercent    float64 `json:"used_percent"`    // Storage usage as percentage
	WarningLevel   bool    `json:"warning_level"`   // True if at or above warning threshold
	RecordingCount int     `json:"recording_count"` // Number of recordings stored
}

// ============================================================================
// Recording Lifecycle Methods
// ============================================================================

// StartRecording starts a new recording session.
// Returns recording_id and error status.
func (r *RecordingManager) StartRecording(name string, pageURL string, sensitiveDataEnabled bool) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already recording
	if r.activeRecordingID != "" {
		return "", fmt.Errorf("already_recording: A recording is already active (id: %s)", r.activeRecordingID)
	}

	// Check storage quota
	if r.recordingStorageUsed >= recordingStorageMax {
		return "", fmt.Errorf("recording_storage_full: Recording storage at capacity (1GB). Please delete old recordings")
	}

	// Warn if approaching limit (80%) - goes to stderr, not stdout (MCP stdio silence)
	if r.recordingStorageUsed >= recordingWarningLevel {
		fmt.Fprintf(os.Stderr, "[WARNING] recording_storage_warning: Recording storage at 80%% (%d bytes / %d bytes)\n",
			r.recordingStorageUsed, recordingStorageMax)
	}

	// Generate recording ID: name-YYYYMMDDTHHMMSS-nnnnnnnnnZ (nanosecond precision prevents collisions)
	now := time.Now()
	timestamp := fmt.Sprintf("%s-%09dZ", now.Format("20060102T150405"), now.Nanosecond())
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
	r.recordings[recordingID] = recording
	r.activeRecordingID = recordingID

	return recordingID, nil
}

// StopRecording stops the current recording and persists it to disk.
// Returns action count and duration.
func (r *RecordingManager) StopRecording(recordingID string) (int, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate recording exists
	recording, exists := r.recordings[recordingID]
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
	err := r.persistRecordingToDisk(recording)
	if err != nil {
		return 0, 0, fmt.Errorf("recording_save_failed: Failed to save recording: %w", err)
	}

	// Update storage used
	r.recordingStorageUsed += calculateRecordingSize(recording)

	// Clear active recording
	if r.activeRecordingID == recordingID {
		r.activeRecordingID = ""
	}

	return actionCount, duration, nil
}

// AddRecordingAction adds an action to the current recording.
func (r *RecordingManager) AddRecordingAction(action RecordingAction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.activeRecordingID == "" {
		return fmt.Errorf("not_recording: No active recording")
	}

	recording := r.recordings[r.activeRecordingID]
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

func primaryRecordingsDir() (string, error) {
	return state.RecordingsDir()
}

func legacyRecordingsDir() (string, error) {
	return state.LegacyRecordingsDir()
}

// recordingReadRoots returns directories to search for existing recordings.
// New state location is preferred; legacy location is included when it exists.
func (r *RecordingManager) recordingReadRoots() ([]string, error) {
	primaryDir, err := primaryRecordingsDir()
	if err != nil {
		return nil, fmt.Errorf("cannot_determine_recordings_dir: %w", err)
	}

	roots := []string{primaryDir}
	legacyDir, err := legacyRecordingsDir()
	if err != nil || legacyDir == "" || legacyDir == primaryDir {
		return roots, nil
	}
	if info, statErr := os.Stat(legacyDir); statErr == nil && info.IsDir() {
		roots = append(roots, legacyDir)
	}
	return roots, nil
}

// persistRecordingToDisk writes recording metadata.json to state/recordings/{id}/
func (r *RecordingManager) persistRecordingToDisk(recording *Recording) error {
	// Determine storage directory
	recordingsDir, err := primaryRecordingsDir()
	if err != nil {
		return fmt.Errorf("cannot_find_recordings_dir: %w", err)
	}

	recordingDir := filepath.Join(recordingsDir, recording.ID)

	// Create directory
	err = os.MkdirAll(recordingDir, 0755)
	if err != nil {
		return fmt.Errorf("mkdir_failed: %w", err)
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
		return fmt.Errorf("json_marshal_failed: %w", err)
	}

	// Write file
	metadataPath := filepath.Join(recordingDir, recordingMetadataFile)
	err = os.WriteFile(metadataPath, data, 0600)
	if err != nil {
		return fmt.Errorf("write_file_failed: %w", err)
	}

	return nil
}

// ============================================================================
// Querying
// ============================================================================

// collectRecordingsFromRoots loads recordings from disk directories, deduplicating by name.
func (r *RecordingManager) collectRecordingsFromRoots(roots []string, limit int) ([]Recording, error) {
	recordings := make([]Recording, 0)
	seen := make(map[string]bool)

	for _, recordingsDir := range roots {
		entries, err := os.ReadDir(recordingsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("readdir_failed: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || seen[entry.Name()] {
				continue
			}
			seen[entry.Name()] = true
			recording, err := r.loadRecordingFromDisk(entry.Name())
			if err != nil {
				continue
			}
			recordings = append(recordings, *recording)
			if limit > 0 && len(recordings) >= limit {
				return recordings, nil
			}
		}
	}
	return recordings, nil
}

// ListRecordings returns all saved recordings from disk.
func (r *RecordingManager) ListRecordings(limit int) ([]Recording, error) {
	roots, err := r.recordingReadRoots()
	if err != nil {
		return nil, err
	}

	recordings, err := r.collectRecordingsFromRoots(roots, limit)
	if err != nil {
		return nil, err
	}

	// Sort by created_at (newest first)
	sort.Slice(recordings, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, recordings[i].CreatedAt)
		t2, _ := time.Parse(time.RFC3339, recordings[j].CreatedAt)
		return t2.Before(t1)
	})

	return recordings, nil
}

// GetRecording loads a specific recording by ID.
func (r *RecordingManager) GetRecording(recordingID string) (*Recording, error) {
	if err := validateRecordingID(recordingID); err != nil {
		return nil, err
	}
	return r.loadRecordingFromDisk(recordingID)
}

// loadRecordingFromDisk reads metadata.json and returns the Recording.
func (r *RecordingManager) loadRecordingFromDisk(recordingID string) (*Recording, error) {
	roots, err := r.recordingReadRoots()
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, root := range roots {
		metadataPath := filepath.Join(root, recordingID, recordingMetadataFile)

		// Read file
		data, err := os.ReadFile(metadataPath) // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads local recording metadata
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			lastErr = fmt.Errorf("read_failed: %w", err)
			continue
		}

		// Unmarshal metadata
		metadata := &recordingtypes.RecordingMetadata{}
		err = json.Unmarshal(data, metadata)
		if err != nil {
			lastErr = fmt.Errorf("json_unmarshal_failed: %w", err)
			continue
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

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("read_failed: recording not found: %s", recordingID)
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

// ============================================================================
// Storage Management
// ============================================================================

// GetStorageInfo returns information about recording storage usage.
func (r *RecordingManager) GetStorageInfo() (StorageInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Recalculate storage from disk for accuracy
	usedBytes, recordingCount, err := r.calculateStorageFromDisk()
	if err != nil {
		return StorageInfo{}, err
	}

	// Update in-memory tracking
	r.recordingStorageUsed = usedBytes

	usedPercent := float64(usedBytes) / float64(recordingStorageMax) * 100

	return StorageInfo{
		UsedBytes:      usedBytes,
		MaxBytes:       recordingStorageMax,
		WarningBytes:   recordingWarningLevel,
		UsedPercent:    usedPercent,
		WarningLevel:   usedBytes >= recordingWarningLevel,
		RecordingCount: recordingCount,
	}, nil
}

// deleteRecordingFromRoot attempts to delete a recording from a single root directory.
// Returns bytes removed and whether the recording was found.
func (r *RecordingManager) deleteRecordingFromRoot(root, recordingID string) (int64, bool, error) {
	recordingDir := filepath.Join(root, recordingID)
	if _, statErr := os.Stat(recordingDir); os.IsNotExist(statErr) {
		return 0, false, nil
	} else if statErr != nil {
		return 0, false, fmt.Errorf("delete_failed: %w", statErr)
	}

	sizeBefore, sizeErr := r.getDirectorySize(recordingDir)
	if sizeErr != nil {
		return 0, false, fmt.Errorf("size_calculation_failed: %w", sizeErr)
	}
	if removeErr := os.RemoveAll(recordingDir); removeErr != nil {
		return 0, false, fmt.Errorf("delete_failed: %w", removeErr)
	}
	return sizeBefore, true, nil
}

// DeleteRecording deletes a recording from disk and updates storage tracking.
func (r *RecordingManager) DeleteRecording(recordingID string) error {
	if err := validateRecordingID(recordingID); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	roots, err := r.recordingReadRoots()
	if err != nil {
		return err
	}

	var totalRemoved int64
	found := false
	for _, root := range roots {
		removed, ok, delErr := r.deleteRecordingFromRoot(root, recordingID)
		if delErr != nil {
			return delErr
		}
		if ok {
			totalRemoved += removed
			found = true
		}
	}

	if !found {
		return fmt.Errorf("recording_not_found: No recording with id: %s", recordingID)
	}

	r.recordingStorageUsed -= totalRemoved
	if r.recordingStorageUsed < 0 {
		r.recordingStorageUsed = 0
	}
	delete(r.recordings, recordingID)
	return nil
}

// RecalculateStorageUsed recalculates storage usage from disk.
// Useful for recovering from inconsistencies or on startup.
func (r *RecordingManager) RecalculateStorageUsed() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	usedBytes, _, err := r.calculateStorageFromDisk()
	if err != nil {
		return err
	}

	r.recordingStorageUsed = usedBytes
	return nil
}

// calculateStorageFromDisk calculates total storage usage by scanning disk.
// Returns (bytes used, recording count, error).
func (r *RecordingManager) calculateStorageFromDisk() (int64, int, error) {
	roots, err := r.recordingReadRoots()
	if err != nil {
		return 0, 0, err
	}

	var totalSize int64
	recordingCount := 0
	seen := make(map[string]bool)

	for _, recordingsDir := range roots {
		size, count, scanErr := r.scanRootForStorage(recordingsDir, seen)
		if scanErr != nil {
			return 0, 0, scanErr
		}
		totalSize += size
		recordingCount += count
	}

	return totalSize, recordingCount, nil
}

// scanRootForStorage scans a single root directory for recording sizes.
func (r *RecordingManager) scanRootForStorage(dir string, seen map[string]bool) (int64, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("readdir_failed: %w", err)
	}

	var totalSize int64
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() || seen[entry.Name()] {
			continue
		}
		seen[entry.Name()] = true
		size, sizeErr := r.getDirectorySize(filepath.Join(dir, entry.Name()))
		if sizeErr != nil {
			continue
		}
		totalSize += size
		count++
	}
	return totalSize, count, nil
}

// getDirectorySize calculates the total size of a directory recursively.
func (r *RecordingManager) getDirectorySize(dirPath string) (int64, error) {
	var size int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}
