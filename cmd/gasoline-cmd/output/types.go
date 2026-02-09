// types.go — Shared types for output formatting.
package output

// Result represents the outcome of a CLI command execution.
type Result struct {
	Success     bool           `json:"success"`
	Tool        string         `json:"tool"`
	Action      string         `json:"action"`
	Data        map[string]any `json:"data,omitempty"`
	Error       string         `json:"error,omitempty"`
	TextContent string         `json:"-"` // Raw text from MCP response (not serialized in JSON)
}

// StreamEvent represents a single streaming progress event.
type StreamEvent struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	Percent   int    `json:"percent,omitempty"`
	BytesSent int64  `json:"bytes_sent,omitempty"`
	Total     int64  `json:"total_bytes,omitempty"`
	ETA       int    `json:"eta_seconds,omitempty"`
	Success   bool   `json:"success,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Formatter is the interface for all output formatters.
type Formatter interface {
	Format(w Writer, result *Result) error
}

// Writer is a minimal write interface (matches io.Writer).
type Writer interface {
	Write(p []byte) (n int, err error)
}
