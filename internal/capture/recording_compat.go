// recording_compat.go — Unexported re-exports for backward compatibility.
// These thin wrappers let existing capture-package tests keep using the
// unexported names after the logic moved to internal/recording.
package capture

import "github.com/dev-console/dev-console/internal/recording"

// Constants re-exported as unexported for capture-package test compatibility.
const (
	recordingStorageMax   = recording.RecordingStorageMax
	recordingWarningLevel = recording.RecordingWarningLevel
)

// Function re-exports for capture-package test compatibility.
var (
	validateRecordingID    = recording.ValidateRecordingID
	calculateRecordingSize = recording.CalculateRecordingSize
)
