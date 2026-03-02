// Purpose: Implements screenshot ingestion, decoding, naming, and persistence helpers.
// Why: Keeps media-specific HTTP handling separate from core log-store lifecycle behavior.
// Docs: docs/features/feature/tab-recording/index.md

package server

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

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

// sanitizeFilename removes characters unsafe for filenames.
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeForFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

const maxPostBodySize = 10 * 1024 * 1024 // 10MB

// screenshotRequest holds the parsed screenshot request body.
type screenshotRequest struct {
	DataURL       string `json:"data_url"`
	URL           string `json:"url"`
	CorrelationID string `json:"correlation_id"`
}

// decodeDataURL extracts raw image bytes from a data URL string.
func decodeDataURL(dataURL string) ([]byte, error) {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid dataUrl format")
	}
	return base64.StdEncoding.DecodeString(parts[1])
}

// buildScreenshotFilename creates a sanitized filename from hostname, timestamp, and optional correlation ID.
func buildScreenshotFilename(pageURL, correlationID string) string {
	hostname := "unknown"
	if pageURL != "" {
		if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
			hostname = u.Host
		}
	}
	ts := time.Now().Format("20060102-150405")
	if correlationID != "" {
		return fmt.Sprintf("%s-%s-%s.jpg", sanitizeForFilename(hostname), ts, sanitizeForFilename(correlationID))
	}
	return fmt.Sprintf("%s-%s.jpg", sanitizeForFilename(hostname), ts)
}

// saveScreenshotFile writes image data to the screenshots directory and returns the full path.
func saveScreenshotFile(filename string, imageData []byte) (string, error) {
	dir, err := state.ScreenshotsDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve screenshots directory: %w", err)
	}
	// #nosec G301 -- 0o755 is appropriate for screenshots directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create screenshots directory: %w", err)
	}
	savePath := filepath.Join(dir, filename)
	// #nosec G306 -- screenshots are intentionally world-readable
	if err := os.WriteFile(savePath, imageData, 0o644); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}
	return savePath, nil
}

// handleScreenshot saves a screenshot JPEG to disk and returns the filename.
func (s *Server) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPostBodySize)
	var body screenshotRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.DataURL == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Missing dataUrl"})
		return
	}

	imageData, err := decodeDataURL(body.DataURL)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid base64 data"})
		return
	}

	filename := buildScreenshotFilename(body.URL, body.CorrelationID)
	savePath, err := saveScreenshotFile(filename, imageData)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"filename":       filename,
		"path":           savePath,
		"correlation_id": body.CorrelationID,
	})
}
