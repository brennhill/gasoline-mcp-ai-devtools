// Purpose: Owns tools_recording_video.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// tools_recording_video.go — Tab video recording: save endpoint and observe handler.
// Handles POST /recordings/save (multipart: video blob + metadata JSON)
// and observe({what: "saved_videos"}) to list saved recordings.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/state"
)

var maxRecordingUploadSizeBytes int64 = 1 << 30 // 1 GiB

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

// queueRecordStart creates the pending query and returns the response for a record_start action.
func (h *ToolHandler) queueRecordStart(req JSONRPCRequest, fullName, audio, videoPath string, fps, tabID int) JSONRPCResponse {
	correlationID := fmt.Sprintf("rec_%d", time.Now().UnixNano())

	extParams := map[string]any{"action": "record_start", "name": fullName, "fps": fps, "audio": audio}
	// Error impossible: map contains only primitive types from input
	extJSON, _ := json.Marshal(extParams)

	query := queries.PendingQuery{
		Type:          "record_start",
		Params:        json.RawMessage(extJSON),
		TabID:         tabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("record_start", "", map[string]any{"name": fullName, "fps": fps, "audio": audio})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording queued", map[string]any{
		"status":                "queued",
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

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
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

	if !h.capture.IsPilotEnabled() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup", h.diagnosticHint())}
	}

	correlationID := fmt.Sprintf("recstop_%d", time.Now().UnixNano())

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
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("record_stop", "", nil)

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording stop queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"message":        "Recording stop queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}

// videoUpload holds the parsed multipart upload data for a recording save.
type videoUpload struct {
	videoFile io.ReadCloser
	meta      VideoRecordingMetadata
	queryID   string
}

// parseVideoUpload extracts and validates the multipart fields from a recording save request.
// Returns nil and writes an HTTP error if validation fails.
func parseVideoUpload(w http.ResponseWriter, r *http.Request) *videoUpload {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		parseErr := strings.ToLower(err.Error())
		if strings.Contains(parseErr, "request body too large") {
			jsonResponse(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "RECORDING_SAVE: Upload exceeds 1GB limit"})
			return nil
		}
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Failed to parse multipart form. " + err.Error()})
		return nil
	}

	videoFile, _, err := r.FormFile("video")
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Missing 'video' field. " + err.Error()})
		return nil
	}

	meta, metaErr := parseVideoMetadata(r.FormValue("metadata"))
	if metaErr != "" {
		_ = videoFile.Close()
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": metaErr})
		return nil
	}

	return &videoUpload{videoFile: videoFile, meta: meta, queryID: r.FormValue("query_id")}
}

// parseVideoMetadata validates and parses the metadata JSON string.
// Returns the parsed metadata and an empty error string on success,
// or a zero metadata and an error message string on failure.
func parseVideoMetadata(metadataStr string) (VideoRecordingMetadata, string) {
	if metadataStr == "" {
		return VideoRecordingMetadata{}, "RECORDING_SAVE: Missing 'metadata' field. Include metadata JSON in the form."
	}

	var meta VideoRecordingMetadata
	if err := json.Unmarshal([]byte(metadataStr), &meta); err != nil {
		return VideoRecordingMetadata{}, "RECORDING_SAVE: Invalid metadata JSON. " + err.Error()
	}

	if meta.Name == "" {
		return VideoRecordingMetadata{}, "RECORDING_SAVE: Metadata missing 'name' field. Include a name in the metadata."
	}

	if strings.ContainsAny(meta.Name, "/\\") || strings.Contains(meta.Name, "..") {
		return VideoRecordingMetadata{}, "RECORDING_SAVE: Invalid recording name — contains path separators. Use alphanumeric characters and hyphens."
	}
	meta.Name = sanitizeVideoSlug(meta.Name)

	return meta, ""
}

// writeVideoToDisk writes the video blob and metadata sidecar to dir.
// Returns the video path and final byte count, or an error string for the HTTP response.
func writeVideoToDisk(dir string, meta *VideoRecordingMetadata, videoFile io.Reader) (string, error) {
	safeName := sanitizeVideoSlug(meta.Name)
	if safeName == "" {
		return "", fmt.Errorf("RECORDING_SAVE: Invalid recording name")
	}

	meta.Name = safeName
	outFile, err := os.CreateTemp(dir, "recording-*.webm")
	if err != nil {
		return "", fmt.Errorf("RECORDING_SAVE: Failed to create file. %w", err)
	}

	videoPath := outFile.Name()
	if !pathWithinDir(videoPath, dir) {
		_ = outFile.Close()
		// #nosec G703 -- path came from os.CreateTemp(dir, ...) and is constrained by pathWithinDir
		_ = os.Remove(videoPath)
		return "", fmt.Errorf("RECORDING_SAVE: Invalid recording path")
	}

	written, err := io.Copy(outFile, videoFile)
	closeErr := outFile.Close()
	if err != nil {
		// #nosec G703 -- path came from os.CreateTemp(dir, ...) and is constrained by pathWithinDir
		_ = os.Remove(videoPath)
		return "", fmt.Errorf("RECORDING_SAVE: Failed to write video. %w", err)
	}
	if closeErr != nil {
		// #nosec G703 -- path came from os.CreateTemp(dir, ...) and is constrained by pathWithinDir
		_ = os.Remove(videoPath)
		return "", fmt.Errorf("RECORDING_SAVE: Failed to finalize video file. %w", closeErr)
	}

	meta.SizeBytes = written

	base := strings.TrimSuffix(filepath.Base(videoPath), ".webm")
	metaPath := filepath.Join(dir, base+"_meta.json")
	if !pathWithinDir(metaPath, dir) {
		return "", fmt.Errorf("RECORDING_SAVE: Invalid metadata path")
	}
	// Error impossible: simple struct with no circular refs or unsupported types
	metaJSON, _ := json.MarshalIndent(*meta, "", "  ")
	// #nosec G306,G703 -- recording metadata path is derived from trusted temp filename in recordings dir
	if err := os.WriteFile(metaPath, metaJSON, 0o600); err != nil {
		return "", fmt.Errorf("RECORDING_SAVE: Failed to write metadata. %w", err)
	}

	return videoPath, nil
}

// handleVideoRecordingSave handles POST /recordings/save from the extension.
// Accepts multipart form with "video" (binary) and "metadata" (JSON string) fields.
func (s *Server) handleVideoRecordingSave(w http.ResponseWriter, r *http.Request, cap interface{ SetQueryResult(string, json.RawMessage) }) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRecordingUploadSizeBytes)

	upload := parseVideoUpload(w, r)
	if upload == nil {
		return
	}
	defer func() {
		_ = upload.videoFile.Close()
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	dir, err := recordingsDir()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	videoPath, writeErr := writeVideoToDisk(dir, &upload.meta, upload.videoFile)
	if writeErr != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": writeErr.Error()})
		return
	}

	if upload.queryID != "" && cap != nil {
		// Error impossible: map contains only primitive types from input
		result, _ := json.Marshal(map[string]any{
			"status":           "saved",
			"name":             upload.meta.Name,
			"path":             videoPath,
			"duration_seconds": upload.meta.DurationSeconds,
			"size_bytes":       upload.meta.SizeBytes,
			"truncated":        upload.meta.Truncated,
		})
		cap.SetQueryResult(upload.queryID, result)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status": "saved",
		"name":   upload.meta.Name,
		"path":   videoPath,
		"size":   upload.meta.SizeBytes,
	})
}

// resolveRevealPath resolves and validates a path against recordings directories.
// Returns the resolved path, an HTTP status code, and an error message.
// Status 0 means success.
func resolveRevealPath(rawPath string, dirs []string) (string, int, string) {
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return "", http.StatusBadRequest, "Invalid path"
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absPath); resolveErr == nil {
		absPath = resolved
	}

	if !isPathInAnyDir(absPath, dirs) {
		return "", http.StatusForbidden, "Path not within recordings directory"
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", http.StatusNotFound, "File not found"
	}

	return absPath, 0, ""
}

// isPathInAnyDir returns true if absPath is within any of the given directories.
func isPathInAnyDir(absPath string, dirs []string) bool {
	for _, dir := range dirs {
		if pathWithinDir(absPath, dir) {
			return true
		}
	}
	return false
}

// validateRevealPath checks that the path is valid and within a recordings directory.
// Returns the resolved absolute path or writes an HTTP error and returns empty string.
func validateRevealPath(w http.ResponseWriter, rawPath string) string {
	dirs := recordingsReadDirs()
	if len(dirs) == 0 {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Could not resolve recordings directory"})
		return ""
	}

	absPath, status, errMsg := resolveRevealPath(rawPath, dirs)
	if status != 0 {
		jsonResponse(w, status, map[string]string{"error": errMsg})
		return ""
	}

	return absPath
}

type revealCommandRunner func(name string, args ...string) error

func defaultRevealCommandRunner(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func revealCommandForOS(goos, absPath string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{"-R", absPath}
	case "windows":
		return "explorer", []string{"/select,", absPath}
	default:
		return "xdg-open", []string{filepath.Dir(absPath)}
	}
}

// revealInFileManagerWithRunner separates command selection from execution so
// tests can verify behavior without opening Finder/Explorer on the developer machine.
func revealInFileManagerWithRunner(goos, absPath string, runner revealCommandRunner) error {
	name, args := revealCommandForOS(goos, absPath)
	return runner(name, args...)
}

// revealInFileManager opens the platform file manager highlighting the given path.
func revealInFileManager(absPath string) error {
	return revealInFileManagerWithRunner(runtime.GOOS, absPath, defaultRevealCommandRunner)
}

// handleRevealRecording handles POST /recordings/reveal — opens Finder/Explorer to the file.
func handleRevealRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Path == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing path"})
		return
	}

	absPath := validateRevealPath(w, body.Path)
	if absPath == "" {
		return
	}

	if err := revealInFileManager(absPath); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to reveal file: " + err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "revealed", "path": absPath})
}

// collectRecordingMetadata scans recording directories and returns deduplicated metadata files.
func collectRecordingMetadata(dirs []string) []string {
	matches := make([]string, 0)
	seen := make(map[string]bool)
	for _, dir := range dirs {
		dirMatches, globErr := filepath.Glob(filepath.Join(dir, "*_meta.json")) // nosemgrep: go_filesystem_rule-fileread
		if globErr != nil {
			continue
		}
		for _, m := range dirMatches {
			if seen[m] {
				continue
			}
			seen[m] = true
			matches = append(matches, m)
		}
	}
	return matches
}

// loadAndFilterRecordings reads metadata files, deduplicates by name, and applies URL filter.
func loadAndFilterRecordings(matches []string, urlFilter string) ([]VideoRecordingMetadata, int64) {
	var recordings []VideoRecordingMetadata
	var totalSize int64
	seenByName := make(map[string]bool)

	for _, metaPath := range matches {
		data, err := os.ReadFile(metaPath) // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads local recording metadata
		if err != nil {
			continue
		}
		var meta VideoRecordingMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if seenByName[meta.Name] {
			continue
		}
		seenByName[meta.Name] = true

		if urlFilter != "" && !recordingMatchesFilter(meta, urlFilter) {
			continue
		}

		recordings = append(recordings, meta)
		totalSize += meta.SizeBytes
	}

	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].CreatedAt > recordings[j].CreatedAt
	})

	return recordings, totalSize
}

// recordingMatchesFilter checks if a recording's name or URL contains the filter string (case-insensitive).
func recordingMatchesFilter(meta VideoRecordingMetadata, filter string) bool {
	lower := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(meta.Name), lower) ||
		strings.Contains(strings.ToLower(meta.URL), lower)
}

// toolObserveSavedVideos handles observe({what: "saved_videos"}).
// Globs state recordings metadata files and returns recording metadata.
func (h *ToolHandler) toolObserveSavedVideos(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL   string `json:"url"`
		LastN int    `json:"last_n,omitempty"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	dirs := recordingsReadDirs()
	if len(dirs) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Could not resolve recordings directory", "Check disk permissions")}
	}

	matches := collectRecordingMetadata(dirs)
	if len(matches) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No saved videos", map[string]any{
			"recordings":         []any{},
			"total":              0,
			"storage_used_bytes": int64(0),
		})}
	}

	recordings, totalSize := loadAndFilterRecordings(matches, params.URL)

	if params.LastN > 0 && len(recordings) > params.LastN {
		recordings = recordings[:params.LastN]
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(fmt.Sprintf("%d saved videos", len(recordings)), map[string]any{
		"recordings":         recordings,
		"total":              len(recordings),
		"storage_used_bytes": totalSize,
	})}
}
