// Purpose: Re-exports recording constants/helpers for capture-package backward compatibility.
// Why: Preserves existing capture test and call-site behavior after recording subsystem extraction.
// Docs: docs/features/feature/playback-engine/index.md
// Docs: docs/features/feature/tab-recording/index.md

package capture

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/recording"

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
