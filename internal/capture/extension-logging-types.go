// Purpose: Owns extension-logging-types.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// extension-logging-types.go — Extension logging types.
// ExtensionLog, PollingLogEntry, and HTTPDebugEntry for extension internal logging.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package capture

import (
	"encoding/json"
	"time"
)

// ExtensionLog represents a log entry from extension's background or content scripts
type ExtensionLog struct {
	Timestamp time.Time       `json:"timestamp"`
	Level     string          `json:"level"`              // "debug", "info", "warn", "error"
	Message   string          `json:"message"`            // Log message
	Source    string          `json:"source"`             // "background", "content", "inject"
	Category  string          `json:"category,omitempty"` // DebugCategory (CONNECTION, CAPTURE, etc.)
	Data      json.RawMessage `json:"data,omitempty"`     // Additional structured data (any JSON)
}

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

// HTTPDebugEntry tracks detailed request/response data for debugging
type HTTPDebugEntry struct {
	Timestamp      time.Time         `json:"timestamp"`
	Endpoint       string            `json:"endpoint"` // URL path
	Method         string            `json:"method"`   // HTTP method
	SessionID      string            `json:"session_id,omitempty"`
	ClientID       string            `json:"client_id,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`         // Request headers (redacted auth)
	RequestBody    string            `json:"request_body,omitempty"`    // First 1KB of request body
	ResponseStatus int               `json:"response_status,omitempty"` // HTTP status code
	ResponseBody   string            `json:"response_body,omitempty"`   // First 1KB of response body
	DurationMs     int64             `json:"duration_ms"`               // Request processing duration
	Error          string            `json:"error,omitempty"`           // Error message if any
}
