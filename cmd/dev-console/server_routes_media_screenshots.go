// Purpose: Screenshot ingest/rate-limit/file-save handlers for media routes.
// Why: Isolates screenshot upload flow from draw-mode annotation session logic.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

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
		// Inline eviction: purge stale entries before rejecting.
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
		// Include data_url in query result so observe(what="screenshot") can return inline image.
		// The HTTP response intentionally omits it to keep the /screenshots response lean.
		queryResult := map[string]string{
			"filename":       filename,
			"path":           savePath,
			"correlation_id": body.CorrelationID,
			"data_url":       body.DataURL,
		}
		// Error impossible: map contains only primitive types from input
		resultJSON, _ := json.Marshal(queryResult)
		cap.SetQueryResult(body.QueryID, resultJSON)
	}
	jsonResponse(w, http.StatusOK, result)
}
