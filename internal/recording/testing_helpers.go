// testing_helpers.go — Exported accessors for cross-package test access.
// These methods expose internal state that tests in other packages (e.g., capture)
// need to verify recording behavior. Not intended for production use.
package recording

// GetInMemoryRecording returns a recording from the in-memory map by ID.
// Returns nil if not found. For test verification only.
func (r *RecordingManager) GetInMemoryRecording(id string) *Recording {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recordings[id]
}

// SetRecordingStorageUsed sets the in-memory storage tracking value.
// For test setup only (e.g., simulating full storage).
func (r *RecordingManager) SetRecordingStorageUsed(bytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordingStorageUsed = bytes
}

// GetActiveRecordingID returns the currently active recording ID.
// For test verification only.
func (r *RecordingManager) GetActiveRecordingID() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeRecordingID
}
