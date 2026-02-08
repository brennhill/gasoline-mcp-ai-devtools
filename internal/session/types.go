// types.go — Session comparison types.
// CaptureStateReader, NamedSnapshot, and related snapshot types.
package session

import (
	"time"

	"github.com/dev-console/dev-console/internal/performance"
)

// CaptureStateReader abstracts reading current server state for snapshot capture.
type CaptureStateReader interface {
	GetConsoleErrors() []SnapshotError
	GetConsoleWarnings() []SnapshotError
	GetNetworkRequests() []SnapshotNetworkRequest
	GetWSConnections() []SnapshotWSConnection
	GetPerformance() *performance.PerformanceSnapshot
	GetCurrentPageURL() string
}

// SnapshotError represents a console error or warning in a snapshot.
type SnapshotError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// SnapshotNetworkRequest represents a network request in a snapshot.
type SnapshotNetworkRequest struct {
	Method       string `json:"method"`
	URL          string `json:"url"`
	Status       int    `json:"status"`
	Duration     int    `json:"duration,omitempty"`
	ResponseSize int    `json:"response_size,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
}

// SnapshotWSConnection represents a WebSocket connection in a snapshot.
type SnapshotWSConnection struct {
	URL         string  `json:"url"`
	State       string  `json:"state"`
	MessageRate float64 `json:"message_rate,omitempty"`
}

// NamedSnapshot is a stored point-in-time browser state.
type NamedSnapshot struct {
	Name                 string                           `json:"name"`
	CapturedAt           time.Time                        `json:"captured_at"`
	URLFilter            string                           `json:"url,omitempty"`
	PageURL              string                           `json:"page_url"`
	ConsoleErrors        []SnapshotError                  `json:"console_errors"`
	ConsoleWarnings      []SnapshotError                  `json:"console_warnings"`
	NetworkRequests      []SnapshotNetworkRequest         `json:"network_requests"`
	WebSocketConnections []SnapshotWSConnection           `json:"websocket_connections"`
	Performance          *performance.PerformanceSnapshot `json:"performance,omitempty"`
}

// SnapshotListEntry is a summary of a snapshot for list response.
type SnapshotListEntry struct {
	Name       string    `json:"name"`
	CapturedAt time.Time `json:"captured_at"`
	PageURL    string    `json:"page_url"`
	ErrorCount int       `json:"error_count"`
}
