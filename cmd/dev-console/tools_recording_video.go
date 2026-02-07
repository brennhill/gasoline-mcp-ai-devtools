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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

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

// recordingsDir returns the path to ~/.gasoline/recordings/, creating it if needed.
func recordingsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".gasoline", "recordings")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create recordings directory: %w", err)
	}
	return dir, nil
}

// sanitizeVideoSlug normalizes a recording name to a filesystem-safe slug.
func sanitizeVideoSlug(s string) string {
	s = strings.ToLower(s)
	// Replace non-alphanumeric chars with hyphens
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		} else {
			result = append(result, '-')
		}
	}
	s = string(result)
	// Collapse multiple hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "recording"
	}
	return s
}

// generateVideoFilename creates a unique filename: {slug}--{YYYY-MM-DD-HHmmss}.webm
func generateVideoFilename(name string) string {
	slug := sanitizeVideoSlug(name)
	ts := time.Now().Format("2006-01-02-150405")
	return fmt.Sprintf("%s--%s", slug, ts)
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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	// Default and clamp fps
	fps := params.FPS
	if fps == 0 {
		fps = 15
	}
	if fps < 5 {
		fps = 5
	}
	if fps > 60 {
		fps = 60
	}

	// Validate audio mode
	audio := params.Audio
	if audio != "" && audio != "tab" && audio != "mic" && audio != "both" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid audio mode: must be 'tab', 'mic', 'both', or omitted", "Use audio: 'tab', 'mic', 'both', or omit for no audio")}
	}

	// Generate filename
	name := params.Name
	if name == "" {
		name = "recording"
	}
	fullName := generateVideoFilename(name)

	// Ensure recordings directory exists
	dir, err := recordingsDir()
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Check disk permissions")}
	}

	// Check for name collision — re-stamp with nanosecond precision if needed
	videoPath := filepath.Join(dir, fullName+".webm")
	if _, err := os.Stat(videoPath); err == nil {
		slug := sanitizeVideoSlug(name)
		fullName = fmt.Sprintf("%s--%s", slug, time.Now().Format("2006-01-02-150405.000000000"))
		videoPath = filepath.Join(dir, fullName+".webm")
	}

	correlationID := fmt.Sprintf("rec_%d", time.Now().UnixNano())

	// Build params to forward to extension
	extParams := map[string]any{
		"action": "record_start",
		"name":   fullName,
		"fps":    fps,
		"audio":  audio,
	}
	extJSON, _ := json.Marshal(extParams)

	query := queries.PendingQuery{
		Type:          "record_start",
		Params:        json.RawMessage(extJSON),
		TabID:         params.TabID,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, queries.AsyncCommandTimeout, req.ClientID)

	h.recordAIAction("record_start", "", map[string]any{"name": fullName, "fps": fps, "audio": audio})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Recording queued", map[string]any{
		"status":         "queued",
		"correlation_id": correlationID,
		"name":           fullName,
		"fps":            fps,
		"audio":          audio,
		"path":           filepath.Join(dir, fullName+".webm"),
		"message":        "Recording start queued. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to confirm.",
	})}
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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrCodePilotDisabled, "AI Web Pilot is disabled", "Enable AI Web Pilot in the extension popup")}
	}

	correlationID := fmt.Sprintf("recstop_%d", time.Now().UnixNano())

	extParams := map[string]any{
		"action": "record_stop",
	}
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

// handleVideoRecordingSave handles POST /recordings/save from the extension.
// Accepts multipart form with "video" (binary) and "metadata" (JSON string) fields.
func (s *Server) handleVideoRecordingSave(w http.ResponseWriter, r *http.Request, cap interface{ SetQueryResult(string, json.RawMessage) }) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// 200MB max upload
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Failed to parse multipart form. " + err.Error()})
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	// Read video file
	videoFile, _, err := r.FormFile("video")
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Missing 'video' field. " + err.Error()})
		return
	}
	defer videoFile.Close()

	// Read metadata
	metadataStr := r.FormValue("metadata")
	if metadataStr == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Missing 'metadata' field. Include metadata JSON in the form."})
		return
	}

	var meta VideoRecordingMetadata
	if err := json.Unmarshal([]byte(metadataStr), &meta); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Invalid metadata JSON. " + err.Error()})
		return
	}

	if meta.Name == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Metadata missing 'name' field. Include a name in the metadata."})
		return
	}

	// Path traversal protection: reject names with path separators or dot-dot sequences
	if strings.ContainsAny(meta.Name, "/\\") || strings.Contains(meta.Name, "..") {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "RECORDING_SAVE: Invalid recording name — contains path separators. Use alphanumeric characters and hyphens."})
		return
	}

	// Ensure directory exists
	dir, err := recordingsDir()
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Write video file
	videoPath := filepath.Join(dir, meta.Name+".webm")
	outFile, err := os.Create(videoPath)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "RECORDING_SAVE: Failed to create file. " + err.Error()})
		return
	}

	written, err := io.Copy(outFile, videoFile)
	outFile.Close()
	if err != nil {
		os.Remove(videoPath)
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "RECORDING_SAVE: Failed to write video. " + err.Error()})
		return
	}

	// Update size from actual written bytes
	meta.SizeBytes = written

	// Write metadata sidecar
	metaPath := filepath.Join(dir, meta.Name+"_meta.json")
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	// #nosec G306 -- recordings are intentionally user-readable
	if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "RECORDING_SAVE: Failed to write metadata. " + err.Error()})
		return
	}

	// If query_id is provided, resolve the pending query
	queryID := r.FormValue("query_id")
	if queryID != "" && cap != nil {
		result, _ := json.Marshal(map[string]any{
			"status":           "saved",
			"name":             meta.Name,
			"path":             videoPath,
			"duration_seconds": meta.DurationSeconds,
			"size_bytes":       meta.SizeBytes,
			"truncated":        meta.Truncated,
		})
		cap.SetQueryResult(queryID, result)
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"status": "saved",
		"name":   meta.Name,
		"path":   videoPath,
		"size":   meta.SizeBytes,
	})
}

// toolObserveSavedVideos handles observe({what: "saved_videos"}).
// Globs ~/.gasoline/recordings/*_meta.json and returns recording metadata.
func (h *ToolHandler) toolObserveSavedVideos(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL   string `json:"url"`
		LastN int    `json:"last_n,omitempty"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}

	dir, err := recordingsDir()
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, err.Error(), "Check disk permissions")}
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*_meta.json"))
	if err != nil || len(matches) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("No saved videos", map[string]any{
			"recordings":        []any{},
			"total":             0,
			"storage_used_bytes": int64(0),
		})}
	}

	var recordings []VideoRecordingMetadata
	var totalSize int64

	for _, metaPath := range matches {
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta VideoRecordingMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}

		// Apply URL/name filter
		if params.URL != "" {
			if !strings.Contains(strings.ToLower(meta.Name), strings.ToLower(params.URL)) &&
				!strings.Contains(strings.ToLower(meta.URL), strings.ToLower(params.URL)) {
				continue
			}
		}

		// Add full path
		meta.Name = meta.Name // keep as-is; path is derived

		recordings = append(recordings, meta)
		totalSize += meta.SizeBytes
	}

	// Sort by created_at descending (newest first)
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].CreatedAt > recordings[j].CreatedAt
	})

	// Apply last_n limit
	if params.LastN > 0 && len(recordings) > params.LastN {
		recordings = recordings[:params.LastN]
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(fmt.Sprintf("%d saved videos", len(recordings)), map[string]any{
		"recordings":         recordings,
		"total":              len(recordings),
		"storage_used_bytes": totalSize,
	})}
}
