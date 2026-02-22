// server_routes_media.go — Screenshot, draw mode, and annotation HTTP handlers.
// Extracted from server_routes.go for file size compliance.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/state"
)

// Screenshot rate limiting
const (
	screenshotMinInterval = 1 * time.Second // Max 1 screenshot per second per client
)

// sanitizeFilename removes characters unsafe for filenames
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeForFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

// screenshotsDir returns the runtime screenshots directory, creating it if needed.
func screenshotsDir() (string, error) {
	dir, err := state.ScreenshotsDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine screenshots directory: %w", err)
	}
	// #nosec G301 -- directory: owner rwx, group rx for traversal
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create screenshots directory: %w", err)
	}
	return dir, nil
}

// checkScreenshotRateLimit enforces per-client screenshot rate limiting.
// Returns an HTTP status code (0 means allowed) and an error message.
func checkScreenshotRateLimit(clientID string) (int, string) {
	if clientID == "" {
		return 0, ""
	}
	screenshotRateMu.Lock()
	defer screenshotRateMu.Unlock()

	lastUpload, exists := screenshotRateLimiter[clientID]
	if exists && time.Since(lastUpload) < screenshotMinInterval {
		return http.StatusTooManyRequests, "Rate limit exceeded: max 1 screenshot per second"
	}
	if len(screenshotRateLimiter) >= 10000 && !exists {
		// Inline eviction: purge stale entries before rejecting
		for id, ts := range screenshotRateLimiter {
			if time.Since(ts) > screenshotMinInterval {
				delete(screenshotRateLimiter, id)
			}
		}
		if len(screenshotRateLimiter) >= 10000 {
			return http.StatusServiceUnavailable, "Rate limiter capacity exceeded"
		}
	}
	screenshotRateLimiter[clientID] = time.Now()
	return 0, ""
}

// decodeDataURL extracts binary data from a data URL (e.g. "data:image/jpeg;base64,...").
// Returns the decoded bytes, or an error string suitable for HTTP responses.
func decodeDataURL(dataURL string) ([]byte, string) {
	if dataURL == "" {
		return nil, "Missing dataUrl"
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, "Invalid dataUrl format"
	}
	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "Invalid base64 data"
	}
	return imageData, ""
}

// buildScreenshotFilename constructs a sanitized filename from URL hostname,
// timestamp, and optional correlation ID.
func buildScreenshotFilename(pageURL, correlationID string) string {
	hostname := "unknown"
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}
	timestamp := time.Now().Format("20060102-150405")
	if correlationID != "" {
		return fmt.Sprintf("%s-%s-%s.jpg",
			sanitizeForFilename(hostname),
			timestamp,
			sanitizeForFilename(correlationID))
	}
	return fmt.Sprintf("%s-%s.jpg", sanitizeForFilename(hostname), timestamp)
}

// saveImageToScreenshotsDir writes image data to the screenshots directory.
// Returns the full path on success, or an HTTP status and error message on failure.
func saveImageToScreenshotsDir(filename string, imageData []byte) (string, int, string) {
	dir, dirErr := screenshotsDir()
	if dirErr != nil {
		return "", http.StatusInternalServerError, "Failed to resolve screenshots directory"
	}
	savePath := filepath.Join(dir, filename)
	if !isWithinDir(savePath, dir) {
		return "", http.StatusBadRequest, "Invalid screenshot path"
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	if err := os.WriteFile(savePath, imageData, 0o600); err != nil {
		return "", http.StatusInternalServerError, "Failed to save screenshot"
	}
	return savePath, 0, ""
}

// handleScreenshot saves a screenshot JPEG to disk and returns the filename.
// If query_id is provided, resolves the pending query directly (on-demand screenshot flow).
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if status, msg := checkScreenshotRateLimit(r.Header.Get("X-Gasoline-Client")); status != 0 {
		jsonResponse(w, status, map[string]string{"error": msg})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body struct {
		DataURL       string `json:"data_url"`
		URL           string `json:"url"`
		CorrelationID string `json:"correlation_id"`
		QueryID       string `json:"query_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	imageData, errMsg := decodeDataURL(body.DataURL)
	if errMsg != "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": errMsg})
		return
	}

	filename := buildScreenshotFilename(body.URL, body.CorrelationID)
	savePath, status, saveErr := saveImageToScreenshotsDir(filename, imageData)
	if status != 0 {
		jsonResponse(w, status, map[string]string{"error": saveErr})
		return
	}

	result := map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	}
	if body.QueryID != "" && cap != nil {
		// Error impossible: map contains only primitive types from input
		resultJSON, _ := json.Marshal(result)
		cap.SetQueryResult(body.QueryID, resultJSON)
	}
	jsonResponse(w, http.StatusOK, result)
}

// saveDrawScreenshot decodes a data URL and writes the screenshot to disk.
// Returns the saved path (empty string on any failure, with a non-nil error
// only for directory resolution failures that should abort the request).
func saveDrawScreenshot(dataURL string, tabID int) (string, error) {
	imageData, errMsg := decodeDataURL(dataURL)
	if errMsg != "" {
		return "", nil
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("draw_%s_tab%d_%d.png", sanitizeForFilename(timestamp), tabID, randomInt63()%10000)

	dir, dirErr := screenshotsDir()
	if dirErr != nil {
		return "", dirErr
	}
	path := filepath.Join(dir, filename)
	if !isWithinDir(path, dir) {
		return "", nil
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	if err := os.WriteFile(path, imageData, 0o600); err != nil {
		return "", nil
	}
	return path, nil
}

// parseAnnotations unmarshals raw annotation JSON, collecting warnings for
// entries that fail to parse.
func parseAnnotations(rawAnnotations []json.RawMessage) ([]Annotation, []string) {
	parsed := make([]Annotation, 0, len(rawAnnotations))
	var warnings []string
	for i, raw := range rawAnnotations {
		var ann Annotation
		if err := json.Unmarshal(raw, &ann); err != nil {
			warnings = append(warnings, fmt.Sprintf("annotation[%d]: %v", i, err))
		} else {
			parsed = append(parsed, ann)
		}
	}
	return parsed, warnings
}

// storeElementDetails persists annotation element details into the global store.
func storeElementDetails(details map[string]json.RawMessage) {
	for correlationID, rawDetail := range details {
		var detail AnnotationDetail
		if err := json.Unmarshal(rawDetail, &detail); err == nil {
			if detail.Selector == "" && detail.Tag == "" {
				rawStr := string(rawDetail)
				if len(rawStr) > 200 {
					rawStr = rawStr[:200] + "..."
				}
				stderrf("[gasoline] draw detail %s: empty (raw=%s)\n", correlationID, rawStr)
			}
			detail.CorrelationID = correlationID
			globalAnnotationStore.StoreDetail(correlationID, detail)
		} else {
			rawStr := string(rawDetail)
			if len(rawStr) > 200 {
				rawStr = rawStr[:200] + "..."
			}
			stderrf("[gasoline] draw detail %s: unmarshal error: %v (raw=%s)\n", correlationID, err, rawStr)
		}
	}
}

// drawModeRequest holds the parsed and validated fields from a draw mode POST body.
type drawModeRequest struct {
	ScreenshotDataURL string                     `json:"screenshot_data_url"`
	Annotations       []json.RawMessage          `json:"annotations"`
	ElementDetails    map[string]json.RawMessage `json:"element_details"`
	PageURL           string                     `json:"page_url"`
	TabID             int                        `json:"tab_id"`
	AnnotSessionName  string                     `json:"annot_session_name"`
	CorrelationID     string                     `json:"correlation_id"`
}

// persistDrawSession writes the full draw session (annotations + element details) to disk
// as a JSON file alongside the screenshot. Files are retained until manually cleared.
func persistDrawSession(body *drawModeRequest, screenshotPath string, annotations []Annotation) {
	dir, err := screenshotsDir()
	if err != nil {
		return
	}
	ts := time.Now().UnixMilli()
	filename := fmt.Sprintf("draw-session-%d-%d.json", body.TabID, ts)
	path := filepath.Join(dir, filename)

	session := map[string]any{
		"annotations":     annotations,
		"element_details": body.ElementDetails,
		"page_url":        body.PageURL,
		"tab_id":          body.TabID,
		"screenshot":      screenshotPath,
		"timestamp":       ts,
		"correlation_id":  body.CorrelationID,
	}
	if body.AnnotSessionName != "" {
		session["annot_session_name"] = body.AnnotSessionName
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return
	}
	// #nosec G306 -- path is validated to remain within screenshots dir
	_ = os.WriteFile(path, data, 0o600)
}

// storeAnnotationSession creates and persists an annotation session, returning
// the stored session for response building.
func storeAnnotationSession(body *drawModeRequest, screenshotPath string, annotations []Annotation) {
	session := &AnnotationSession{
		Annotations:    annotations,
		ScreenshotPath: screenshotPath,
		PageURL:        body.PageURL,
		TabID:          body.TabID,
		Timestamp:      time.Now().UnixMilli(),
	}
	globalAnnotationStore.StoreSession(body.TabID, session)
	if body.AnnotSessionName != "" {
		globalAnnotationStore.AppendToNamedSession(body.AnnotSessionName, session)
	}
}

// handleDrawModeComplete receives annotation data and screenshot from the extension
// when the user finishes a draw mode session.
func (s *Server) handleDrawModeComplete(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body drawModeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.TabID <= 0 {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "tab_id is required and must be > 0"})
		return
	}

	var screenshotPath string
	if body.ScreenshotDataURL != "" {
		path, err := saveDrawScreenshot(body.ScreenshotDataURL, body.TabID)
		if err != nil {
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to resolve screenshots directory"})
			return
		}
		screenshotPath = path
	}

	parsedAnnotations, parseWarnings := parseAnnotations(body.Annotations)
	storeAnnotationSession(&body, screenshotPath, parsedAnnotations)
	storeElementDetails(body.ElementDetails)

	// Persist full session to disk so the LLM can compare/contrast across restarts
	persistDrawSession(&body, screenshotPath, parsedAnnotations)

	result := map[string]any{
		"status":           "stored",
		"annotation_count": len(parsedAnnotations),
		"screenshot":       screenshotPath,
	}
	if len(parseWarnings) > 0 {
		result["warnings"] = parseWarnings
	}

	// Complete the pending command — unblocks WaitForCommand in tools_async.go
	// so the LLM can retrieve results via correlation_id.
	if body.CorrelationID != "" && cap != nil {
		resultJSON, _ := json.Marshal(result)
		cap.CompleteCommand(body.CorrelationID, resultJSON, "")
	}

	jsonResponse(w, http.StatusOK, result)
}
