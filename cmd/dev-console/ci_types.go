// Purpose: Shared request/response payload types for CI endpoints.
// Why: Keeps HTTP handler code concise and gives snapshot contracts a single definition site.

package main

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"

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
