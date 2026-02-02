// log.go — Logging types for server and extension instrumentation.
// Contains canonical definitions for server logs and extension debug logs.
// Zero dependencies - foundational types used by capture and debugging packages.
package types

import "time"

// ============================================
// Server Logging
// ============================================

// LogEntry represents a generic log entry from the server.
// Implemented as a map to allow flexible field addition without schema changes.
type LogEntry map[string]interface{}

// ============================================
// Extension Logging
// ============================================

// ExtensionLog represents a log entry from the extension's background or content scripts
type ExtensionLog struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`    // "debug", "info", "warn", "error"
	Message   string                 `json:"message"`  // Log message
	Source    string                 `json:"source"`   // "background", "content", "inject"
	Category  string                 `json:"category,omitempty"` // DebugCategory (CONNECTION, CAPTURE, etc.)
	Data      map[string]interface{} `json:"data,omitempty"`     // Additional structured data
}

// ============================================
// Polling Activity Log
// ============================================

// PollingLogEntry tracks a single polling request (GET /pending-queries or POST /settings)
type PollingLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	Endpoint     string    `json:"endpoint"` // "pending-queries" or "settings"
	Method       string    `json:"method"`   // "GET" or "POST"
	SessionID    string    `json:"session_id,omitempty"`
	PilotEnabled *bool     `json:"pilot_enabled,omitempty"` // Only for POST /settings
	PilotHeader  string    `json:"pilot_header,omitempty"`  // Only for GET with X-Gasoline-Pilot header
	QueryCount   int       `json:"query_count,omitempty"`   // Number of pending queries returned
}

// ============================================
// HTTP Debug Log
// ============================================

// HTTPDebugEntry tracks detailed request/response data for debugging
type HTTPDebugEntry struct {
	Timestamp       time.Time         `json:"timestamp"`
	Endpoint        string            `json:"endpoint"`        // URL path
	Method          string            `json:"method"`          // HTTP method
	SessionID       string            `json:"session_id,omitempty"`
	ClientID        string            `json:"client_id,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`         // Request headers (redacted auth)
	RequestBody     string            `json:"request_body,omitempty"`    // First 1KB of request body
	ResponseStatus  int               `json:"response_status,omitempty"` // HTTP status code
	ResponseBody    string            `json:"response_body,omitempty"`   // First 1KB of response body
	DurationMs      int64             `json:"duration_ms"`               // Request processing duration
	Error           string            `json:"error,omitempty"`           // Error message if any
}
