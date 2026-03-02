// Purpose: Shared video-recording constants and data types used by recording handlers and file endpoints.
// Why: Keeps durable contracts (state names, metadata schema, command timeouts) centralized.
// Docs: docs/features/feature/tab-recording/index.md

package main

import "time"

var maxRecordingUploadSizeBytes int64 = 1 << 30 // 1 GiB

const (
	recordingStateIdle            = "idle"
	recordingStateAwaitingGesture = "awaiting_user_gesture"
	recordingStateRecording       = "recording"
	recordingStateStopping        = "stopping"

	recordStartCommandTimeout = 2 * time.Minute
	recordStopCommandTimeout  = 90 * time.Second
)

// interactRecordingState tracks interact(record_start/record_stop) lifecycle.
type interactRecordingState struct {
	State              string
	StartCorrelationID string
	StopCorrelationID  string
	UpdatedAt          time.Time
}

// VideoRecordingMetadata is the sidecar JSON written next to each .webm file.
type VideoRecordingMetadata struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	CreatedAt       string `json:"created_at"`
	DurationSeconds int    `json:"duration_seconds"`
	SizeBytes       int64  `json:"size_bytes"`
	URL             string `json:"url"`
	TabID           int    `json:"tab_id"`
	Resolution      string `json:"resolution"`
	Format          string `json:"format"`
	FPS             int    `json:"fps"`
	HasAudio        bool   `json:"has_audio,omitempty"`
	AudioMode       string `json:"audio_mode,omitempty"`
	Truncated       bool   `json:"truncated,omitempty"`
}
