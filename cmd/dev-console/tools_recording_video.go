// tools_recording_video.go — Recording state machine: types, helpers, and interact handlers (record_start/record_stop).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/state"
)

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

// recordingsDir returns the runtime recordings directory, creating it if needed.
func recordingsDir() (string, error) {
	dir, err := state.RecordingsDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine recordings directory: %w", err)
	}
	// #nosec G301 -- directory: owner rwx, group rx for traversal
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create recordings directory: %w", err)
	}
	return dir, nil
}

func recordingsReadDirs() []string {
	primaryDir, err := recordingsDir()
	if err != nil {
		return nil
	}
	dirs := []string{primaryDir}

	legacyDir, err := state.LegacyRecordingsDir()
	if err != nil || legacyDir == "" || legacyDir == primaryDir {
		return dirs
	}
	if info, statErr := os.Stat(legacyDir); statErr == nil && info.IsDir() {
		dirs = append(dirs, legacyDir)
	}

	return dirs
}

func pathWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSlugChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}

// sanitizeVideoSlug normalizes a recording name to a filesystem-safe slug.
func sanitizeVideoSlug(s string) string {
	s = strings.ToLower(s)
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if isSlugChar(s[i]) {
			result = append(result, s[i])
		} else {
			result = append(result, '-')
		}
	}
	s = collapseHyphens(string(result))
	if s == "" {
		s = "recording"
	}
	return s
}

func collapseHyphens(s string) string {
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

// generateVideoFilename creates a unique filename: {slug}--{YYYY-MM-DD-HHmmss}.webm
func generateVideoFilename(name string) string {
	slug := sanitizeVideoSlug(name)
	ts := time.Now().Format("2006-01-02-150405")
	return fmt.Sprintf("%s--%s", slug, ts)
}

// clampFPS applies default and bounds to a requested FPS value.
func clampFPS(fps int) int {
	if fps == 0 {
		fps = 15
	}
	if fps < 5 {
		return 5
	}
	if fps > 60 {
		return 60
	}
	return fps
}

// validAudioModes lists allowed values for the audio parameter.
var validAudioModes = map[string]bool{
	"":     true,
	"tab":  true,
	"mic":  true,
	"both": true,
}

// resolveRecordingPath picks a unique .webm path inside dir, handling collisions.
func resolveRecordingPath(dir, name string) (fullName string, videoPath string) {
	fullName = generateVideoFilename(name)
	videoPath = filepath.Join(dir, fullName+".webm")
	if _, err := os.Stat(videoPath); err == nil {
		slug := sanitizeVideoSlug(name)
		fullName = fmt.Sprintf("%s--%s", slug, time.Now().Format("2006-01-02-150405.000000000"))
		videoPath = filepath.Join(dir, fullName+".webm")
	}
	return fullName, videoPath
}

// extractRecordingLifecycleStatus pulls the extension-reported lifecycle status
// from command result payloads ("recording", "saved", "error", etc.).
func extractRecordingLifecycleStatus(result json.RawMessage) string {
	if len(result) == 0 {
		return ""
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payload.Status))
}

// resolveInteractRecordingState refreshes state using latest command results.
func (h *ToolHandler) resolveInteractRecordingState() interactRecordingState {
	h.recordInteractMu.Lock()
	defer h.recordInteractMu.Unlock()

	state := h.recordInteract
	if state.State == "" {
		state.State = recordingStateIdle
	}

	if state.StopCorrelationID != "" {
		if stopCmd, found := h.capture.GetCommandResult(state.StopCorrelationID); found {
			if stopCmd.Status == "pending" {
				state.State = recordingStateStopping
				state.UpdatedAt = time.Now()
				h.recordInteract = state
				return state
			}
			// Any terminal stop result returns the state machine to idle.
			state = interactRecordingState{State: recordingStateIdle, UpdatedAt: time.Now()}
			h.recordInteract = state
			return state
		}
	}

	if state.StartCorrelationID == "" {
		state.State = recordingStateIdle
		state.UpdatedAt = time.Now()
		h.recordInteract = state
		return state
	}

	startCmd, found := h.capture.GetCommandResult(state.StartCorrelationID)
	if !found {
		// Keep queued state until command result appears.
		if state.State == "" {
			state.State = recordingStateAwaitingGesture
		}
		state.UpdatedAt = time.Now()
		h.recordInteract = state
		return state
	}

	switch startCmd.Status {
	case "pending":
		state.State = recordingStateAwaitingGesture
	case "complete":
		switch extractRecordingLifecycleStatus(startCmd.Result) {
		case recordingStateRecording:
			state.State = recordingStateRecording
		case recordingStateAwaitingGesture:
			state.State = recordingStateAwaitingGesture
		default:
			state = interactRecordingState{State: recordingStateIdle}
		}
	default:
		// error/timeout/expired/cancelled and unknown statuses are terminal.
		state = interactRecordingState{State: recordingStateIdle}
	}

	state.UpdatedAt = time.Now()
	h.recordInteract = state
	return state
}

func (h *ToolHandler) setInteractRecordingStart(correlationID string) {
	h.recordInteractMu.Lock()
	defer h.recordInteractMu.Unlock()
	h.recordInteract = interactRecordingState{
		State:              recordingStateAwaitingGesture,
		StartCorrelationID: correlationID,
		UpdatedAt:          time.Now(),
	}
}

func (h *ToolHandler) setInteractRecordingStopping(correlationID string) {
	h.recordInteractMu.Lock()
	defer h.recordInteractMu.Unlock()
	state := h.recordInteract
	if state.State == "" {
		state.State = recordingStateIdle
	}
	state.State = recordingStateStopping
	state.StopCorrelationID = correlationID
	state.UpdatedAt = time.Now()
	h.recordInteract = state
}

// queueRecordStart creates the pending query and returns the response for a record_start action.
func (h *ToolHandler) queueRecordStart(req JSONRPCRequest, fullName, audio, videoPath string, fps, tabID int) JSONRPCResponse {
	correlationID := newCorrelationID("rec")

	extParams := map[string]any{"action": "record_start", "name": fullName, "fps": fps, "audio": audio}
	// Error impossible: map contains only primitive types from input
	extJSON, _ := json.Marshal(extParams)

	query := queries.PendingQuery{
		Type:          "record_start",
		Params:        json.RawMessage(extJSON),
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, recordStartCommandTimeout, req.ClientID)
	h.setInteractRecordingStart(correlationID)

	h.recordAIAction("record_start", "", map[string]any{"name": fullName, "fps": fps, "audio": audio})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording queued", map[string]any{
		"status":                "queued",
		"recording_state":       recordingStateAwaitingGesture,
		"correlation_id":        correlationID,
		"name":                  fullName,
		"fps":                   fps,
		"audio":                 audio,
		"path":                  videoPath,
		"requires_user_gesture": true,
		"user_prompt":           "Prompt the user to click the Gasoline icon to grant recording permission for the target tab.",
		"message":               "Recording start queued. Prompt user to click the Gasoline icon to enable recording, then use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to confirm.",
	})}
}

// handleRecordStart processes interact({action: "record_start"}).
// Generates the filename, forwards to extension via PendingQuery.
func (h *ToolHandler) handleRecordStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name  string `json:"name"`
		FPS   int    `json:"fps,omitempty"`
		TabID int    `json:"tab_id,omitempty"`
		Audio string `json:"audio,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	fps := clampFPS(params.FPS)

	if !validAudioModes[params.Audio] {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid audio mode: must be 'tab', 'mic', 'both', or omitted", "Use audio: 'tab', 'mic', 'both', or omit for no audio")}
	}

	name := params.Name
	if name == "" {
		name = "recording"
	}

	dir, err := recordingsDir()
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Check disk permissions")}
	}

	fullName, videoPath := resolveRecordingPath(dir, name)
	return h.queueRecordStart(req, fullName, params.Audio, videoPath, fps, params.TabID)
}

// handleRecordStop processes interact({action: "record_stop"}).
func (h *ToolHandler) handleRecordStop(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TabID int `json:"tab_id,omitempty"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	if resp, blocked := h.requirePilot(req); blocked {
		return resp
	}

	recordingState := h.resolveInteractRecordingState()
	if recordingState.State != recordingStateRecording {
		retry := "Run interact(action:'record_start') and wait for observe(what:'command_result') to report status 'recording' before stopping."
		if recordingState.State == recordingStateAwaitingGesture {
			retry = "Recording start is still awaiting user gesture. Ask the user to click the Gasoline icon, then retry stop after start reports status 'recording'."
		}
		if recordingState.State == recordingStateStopping {
			retry = "A previous record_stop is still in progress. Poll observe(what:'command_result') for the stop correlation_id and wait for a terminal status."
		}
		msg := fmt.Sprintf("Cannot stop recording while state is %q", recordingState.State)
		if recordingState.StartCorrelationID == "" {
			msg = "Cannot stop recording: no active interact(record_start) session found"
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrNoData, msg, retry, h.diagnosticHint()),
		}
	}

	correlationID := newCorrelationID("recstop")

	extParams := map[string]any{
		"action": "record_stop",
	}
	// Error impossible: map contains only string values
	extJSON, _ := json.Marshal(extParams)

	query := queries.PendingQuery{
		Type:          "record_stop",
		Params:        json.RawMessage(extJSON),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, recordStopCommandTimeout, req.ClientID)
	h.setInteractRecordingStopping(correlationID)

	h.recordAIAction("record_stop", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording stop queued", map[string]any{
		"status":          "queued",
		"recording_state": recordingStateStopping,
		"correlation_id":  correlationID,
		"message":         "Recording stop queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}

