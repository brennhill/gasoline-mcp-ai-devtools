package main

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

func defaultEvidenceCapture(h *ToolHandler, clientID string) evidenceShot {
	if h == nil || h.capture == nil {
		return evidenceShot{Error: "capture_not_initialized"}
	}
	enabled, _, _ := h.capture.GetTrackingStatus()
	if !enabled {
		return evidenceShot{Error: "no_tracked_tab"}
	}

	queryID, qerr := h.capture.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: json.RawMessage(`{}`),
		},
		12*time.Second,
		clientID,
	)
	if qerr != nil {
		return evidenceShot{Error: "queue_full: " + qerr.Error()}
	}

	raw, err := h.capture.WaitForResult(queryID, 12*time.Second)
	if err != nil {
		return evidenceShot{Error: "screenshot_timeout: " + err.Error()}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return evidenceShot{Error: "screenshot_parse_error: " + err.Error()}
	}

	if errMsg, ok := payload["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
		return evidenceShot{Error: strings.TrimSpace(errMsg)}
	}

	path, _ := payload["path"].(string)
	filename, _ := payload["filename"].(string)
	path = strings.TrimSpace(path)
	filename = strings.TrimSpace(filename)
	if path == "" {
		return evidenceShot{
			Filename: filename,
			Error:    "screenshot_missing_path",
		}
	}

	return evidenceShot{
		Path:     path,
		Filename: filename,
	}
}

func (h *ToolHandler) captureEvidenceWithRetry(clientID string) evidenceShot {
	retries := evidenceRetryCount()
	attempts := retries + 1
	last := evidenceShot{Error: "evidence_capture_not_attempted"}

	for i := 0; i < attempts; i++ {
		shot := evidenceCaptureFn(h, clientID)
		shot.Attempts = i + 1
		if strings.TrimSpace(shot.Path) != "" {
			return shot
		}
		if strings.TrimSpace(shot.Error) == "" {
			shot.Error = "evidence_capture_failed"
		}
		last = shot
		if i < attempts-1 {
			time.Sleep(150 * time.Millisecond)
		}
	}

	return last
}
