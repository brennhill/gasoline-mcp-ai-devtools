// Purpose: Owns ci.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// ci.go — Gasoline CI Infrastructure endpoints.
// Provides /snapshot (aggregated state), /clear (buffer reset), and
// /test-boundary (test correlation) for CI/CD pipeline integration.
// These endpoints enable headless browser capture during automated testing.
package main

import (
	"encoding/json"
	"github.com/dev-console/dev-console/internal/capture"
	"io"
	"net/http"
	"time"
)

// ============================================
// Types
// ============================================

// SnapshotResponse is the aggregated state returned by GET /snapshot.
type SnapshotResponse struct {
	Timestamp       string                   `json:"timestamp"`
	TestID          string                   `json:"test_id,omitempty"`
	Logs            []LogEntry               `json:"logs"`
	WebSocket       []capture.WebSocketEvent `json:"websocket_events"`
	NetworkBodies   []capture.NetworkBody    `json:"network_bodies"`
	EnhancedActions []capture.EnhancedAction `json:"enhanced_actions,omitempty"`
	Stats           SnapshotStats            `json:"stats"`
}

// SnapshotStats summarizes the snapshot contents.
type SnapshotStats struct {
	TotalLogs       int `json:"total_logs"`
	ErrorCount      int `json:"error_count"`
	WarningCount    int `json:"warning_count"`
	NetworkFailures int `json:"network_failures"`
	WSConnections   int `json:"ws_connections"`
}

// TestBoundaryRequest is the request body for POST /test-boundary.
type TestBoundaryRequest struct {
	TestID string `json:"test_id"`
	Action string `json:"action"` // "start" or "end"
}

// ============================================
// Handlers
// ============================================

// handleSnapshot returns an HTTP handler for GET /snapshot.
// Returns all captured state in a single response.
// #lizard forgives
func handleSnapshot(server *Server, cap *capture.Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		since := r.URL.Query().Get("since")
		testID := r.URL.Query().Get("test_id")

		var sinceTime time.Time
		if since != "" {
			var err error
			sinceTime, err = time.Parse(time.RFC3339Nano, since)
			if err != nil {
				jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid since timestamp"})
				return
			}
		}

		// Gather logs
		logs := server.getEntries()
		if !sinceTime.IsZero() {
			logs = filterLogsSince(logs, sinceTime)
		}

		// Gather capture data
		wsEvents := cap.GetAllWebSocketEvents()
		networkBodies := cap.GetNetworkBodies()
		enhancedActions := cap.GetAllEnhancedActions()

		// Apply test_id label (use first active test ID if not specified).
		// Note: test_id is currently for labeling only. Full per-entry filtering
		// would require storing test boundary timestamps alongside buffer entries.
		if testID == "" {
			activeIDs := cap.GetActiveTestIDs()
			if len(activeIDs) > 0 {
				testID = activeIDs[0]
			}
		}

		stats := computeSnapshotStats(logs, wsEvents, networkBodies)

		snapshot := SnapshotResponse{
			Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
			TestID:          testID,
			Logs:            logs,
			WebSocket:       wsEvents,
			NetworkBodies:   networkBodies,
			EnhancedActions: enhancedActions,
			Stats:           stats,
		}

		jsonResponse(w, http.StatusOK, snapshot)
	}
}

// handleClear returns an HTTP handler for POST/DELETE /clear.
// Resets all buffers atomically.
func handleClear(server *Server, cap *capture.Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		previousCount := server.getEntryCount()
		server.clearEntries()
		cap.ClearAll()

		jsonResponse(w, http.StatusOK, map[string]any{
			"cleared":         true,
			"entries_removed": previousCount,
		})
	}
}

// handleTestBoundary returns an HTTP handler for POST /test-boundary.
// Marks test boundaries for correlation.
func handleTestBoundary(cap *capture.Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
		if err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
			return
		}

		var req TestBoundaryRequest
		if err := json.Unmarshal(body, &req); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}

		if req.TestID == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test_id is required"})
			return
		}
		if req.Action != "start" && req.Action != "end" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "action must be 'start' or 'end'"})
			return
		}

		now := time.Now().UTC()

		if req.Action == "start" {
			cap.SetTestBoundaryStart(req.TestID)
		} else {
			cap.SetTestBoundaryEnd(req.TestID)
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"test_id":   req.TestID,
			"action":    req.Action,
			"timestamp": now.Format(time.RFC3339Nano),
		})
	}
}

// ============================================
// Helpers
// ============================================

// filterLogsSince returns only log entries with timestamps after the given time.
// Uses the server's addedAt timestamps for ordering, falling back to the "ts" field.
func filterLogsSince(logs []LogEntry, since time.Time) []LogEntry {
	result := make([]LogEntry, 0, len(logs))
	for _, entry := range logs {
		ts, ok := entry["ts"].(string)
		if !ok {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			continue
		}
		if entryTime.After(since) {
			result = append(result, entry)
		}
	}
	return result
}

// computeSnapshotStats computes summary statistics for a snapshot.
func computeSnapshotStats(logs []LogEntry, wsEvents []capture.WebSocketEvent, networkBodies []capture.NetworkBody) SnapshotStats {
	stats := SnapshotStats{
		TotalLogs: len(logs),
	}

	for _, entry := range logs {
		level, _ := entry["level"].(string)
		switch level {
		case "error":
			stats.ErrorCount++
		case "warn", "warning":
			stats.WarningCount++
		}
	}

	for _, body := range networkBodies {
		if body.Status >= 400 {
			stats.NetworkFailures++
		}
	}

	// Count unique WS connections
	connSeen := make(map[string]bool)
	for _, evt := range wsEvents {
		if evt.URL != "" {
			connSeen[evt.URL] = true
		}
	}
	stats.WSConnections = len(connSeen)

	return stats
}

// ============================================
// Capture methods for test boundary tracking
// ============================================
