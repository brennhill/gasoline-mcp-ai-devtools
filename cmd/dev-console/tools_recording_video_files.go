// tools_recording_video_files.go — Video file I/O: upload parsing, save endpoint, reveal, and observe listing.
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
)

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
