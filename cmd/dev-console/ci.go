// ci.go — Gasoline CI Infrastructure endpoints.
// Provides /snapshot (aggregated state), /clear (buffer reset), and
// /test-boundary (test correlation) for CI/CD pipeline integration.
// These endpoints enable headless browser capture during automated testing.
package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// ============================================
// Types
// ============================================

// SnapshotResponse is the aggregated state returned by GET /snapshot.
type SnapshotResponse struct {
	Timestamp       string            `json:"timestamp"`
	TestID          string            `json:"test_id,omitempty"`
	Logs            []LogEntry        `json:"logs"`
	WebSocket       []WebSocketEvent  `json:"websocket_events"`
	NetworkBodies   []NetworkBody     `json:"network_bodies"`
	EnhancedActions []EnhancedAction  `json:"enhanced_actions,omitempty"`
	Stats           SnapshotStats     `json:"stats"`
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
func handleSnapshot(server *Server, capture *Capture) http.HandlerFunc {
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
		wsEvents := capture.GetWebSocketEvents(WebSocketEventFilter{})
		networkBodies := capture.GetNetworkBodies(NetworkBodyFilter{})
		enhancedActions := capture.GetEnhancedActions(EnhancedActionFilter{})

		// Apply test_id label (use current test ID if not specified).
		// Note: test_id is currently for labeling only. Full per-entry filtering
		// would require storing test boundary timestamps alongside buffer entries.
		if testID == "" {
			testID = capture.GetCurrentTestID()
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
func handleClear(server *Server, capture *Capture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" && r.Method != "DELETE" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		previousCount := server.getEntryCount()
		server.clearEntries()
		capture.ClearAll()

		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"cleared":         true,
			"entries_removed": previousCount,
		})
	}
}

// handleTestBoundary returns an HTTP handler for POST /test-boundary.
// Marks test boundaries for correlation.
func handleTestBoundary(capture *Capture) http.HandlerFunc {
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
			capture.SetCurrentTestID(req.TestID)
		} else {
			capture.SetCurrentTestID("")
		}

		jsonResponse(w, http.StatusOK, map[string]interface{}{
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
func computeSnapshotStats(logs []LogEntry, wsEvents []WebSocketEvent, networkBodies []NetworkBody) SnapshotStats {
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

// GetCurrentTestID returns the current test ID set via /test-boundary.
func (c *Capture) GetCurrentTestID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentTestID
}

// SetCurrentTestID sets the current test ID for correlating entries.
func (c *Capture) SetCurrentTestID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentTestID = id
}

// ClearAll resets all capture buffers atomically.
func (c *Capture) ClearAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wsEvents = nil
	c.wsAddedAt = nil
	c.networkBodies = nil
	c.networkAddedAt = nil
	c.enhancedActions = nil
	c.actionAddedAt = nil
	c.connections = make(map[string]*connectionState)
	c.closedConns = nil
	c.connOrder = nil
	c.currentTestID = ""
}
